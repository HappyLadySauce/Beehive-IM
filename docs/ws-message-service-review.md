# WS 与 Message 服务审查文档

## 1. 审查范围

本文基于当前仓库代码审查 WebSocket 服务与消息服务的职责边界、调用关系、资源开销和中间件交互。审查对象包括：

- `cmd/app/routes/ws/*`：WebSocket HTTP 升级入口与认证后路由注册。
- `cmd/app/service/ws/*`：连接管理、读写泵、协议 envelope、MQ 消费后在线投递。
- `cmd/app/service/message/*`：消息请求校验、会话成员校验、消息落库、幂等处理、按接收人发布 MQ 事件。
- `pkg/mq/*`：RabbitMQ 连接、Topic Exchange、Queue 绑定、发布与消费。
- `cmd/app/svc/servicecontext.go`：PostgreSQL、Redis、RabbitMQ、MessageService、Hub、consumer 的启动装配。

## 2. 总体结论

当前实现采用单体 IM 中较常见的“WS Gateway + Message Domain Service + RabbitMQ Dispatch Queue”结构：

1. WebSocket 负责长连接生命周期、协议 envelope 收发、在线用户本地投递。
2. MessageService 负责业务消息的校验、持久化、幂等、会话序号分配和 MQ 扇出。
3. RabbitMQ 作为进程内 Hub 与可靠消息域之间的异步分发边界。
4. PostgreSQL 是消息与投递状态的事实存储。
5. Redis 当前主要由认证/session 链路使用，WS/message 投递链路没有直接使用 Redis。

这个方向是合理的：业务消息进入可靠链路，WebSocket 控制帧和连接状态留在本地 Hub；不要把 ping/pong、错误帧、连接注册这类高频控制事件强行经过 MQ。

## 3. 模块职责划分

### 3.1 WS Route 层

`cmd/app/routes/ws/connect.go` 将 HTTP 请求升级为 WebSocket，构造 `ClientIdentity`，注册到 Hub，然后启动：

- 一个 `WritePump` goroutine：负责从 per-client send channel 取消息并写回 socket。
- 当前请求 goroutine 中的 `ReadPump`：负责从 socket 读取 envelope 并交给 Hub。

路由注册在 `/api/v1/ws/connect`，并使用认证中间件，因此连接身份来自认证上下文。

### 3.2 WS Client 层

`Client` 持有：

- `websocket.Conn`
- 用户、会话、设备、平台身份
- 固定容量 send channel，当前容量为 `256`
- `ReadPump` 与 `WritePump`

关键资源模型：

- 每个连接至少 1 个独立写 goroutine。
- 每个连接还会占用一个正在执行 `ReadPump` 的请求 goroutine。
- 每个连接有一个 `chan []byte`，最多缓存 256 条待写帧。
- 每个连接维护 websocket TCP 连接、读写 deadline、ping/pong 心跳。

### 3.3 WS Hub 层

Hub 使用内存 map 管理连接：

- `clients map[*Client]struct{}`
- `clientByUserID map[string]map[*Client]struct{}`

其职责是：

- 注册/注销连接。
- 同一个 user + session 重连时关闭旧连接。
- 将收到的 `TypeMessageSend` 转给 MessageService。
- 将 RabbitMQ 消费到的 `MessageDeliverPayload` 推送给目标用户所有在线 session。

Hub 不负责消息落库、会话成员校验、幂等、broker 连接和离线消息补偿，这是正确边界。

### 3.4 MessageService 层

`MessageService.SendMessage` 是业务消息核心入口，当前流程是：

1. 校验服务依赖是否存在。
2. 解析 `conversation_id`、`client_message_id`、`message_content`。
3. 在数据库事务内校验会话存在。
4. 校验发送者是会话成员。
5. 按 `sender_user_id + client_message_id` 做幂等查询。
6. 锁定会话行，读取当前最大 sequence，并分配 `max(sequence) + 1`。
7. 插入 `messages`。
8. 查询会话成员，为除发送者外的每个成员创建 `message_deliveries`。
9. 事务提交后，逐个接收人发布 RabbitMQ 事件。
10. 发布成功后将对应 delivery 标记为 `delivered`。
11. 向发送方返回 `TypeMessageAck`。

这个设计的优点是业务校验和持久化清晰，幂等键避免客户端重试造成重复消息，会话行锁保证单会话内 sequence 递增。

## 4. 数据流向

### 4.1 用户发送消息

```text
Client A
  -> WebSocket /api/v1/ws/connect
  -> ReadPump
  -> Hub.HandleEnvelope(TypeMessageSend)
  -> MessageService.SendMessage
  -> PostgreSQL transaction
       - check conversation
       - check membership
       - check client_message_id idempotency
       - lock conversation
       - insert message
       - insert per-recipient delivery rows
  -> RabbitMQ topic exchange publish per recipient
  -> mark message_deliveries delivered
  -> Hub sends TypeMessageAck to Client A
```

### 4.2 接收方在线投递

```text
RabbitMQ queue im.message.dispatch
  -> Hub.StartDeliveryConsumer
  -> mq.Consume manual ack loop
  -> Hub.DeliverToUser
  -> find online sessions by recipient_user_id
  -> Client.SendEnvelope(TypeMessageReceive)
  -> target client's WritePump
  -> WebSocket frame to Client B / Client B multiple sessions
```

### 4.3 接收方离线

当前 consumer 即使目标用户不在线，`Hub.DeliverToUser` 也会返回成功，RabbitMQ delivery 会被 ack。由于 `message_deliveries` 已在发布后标记为 `delivered`，当前实现不区分“已交给本地 Hub”与“实际写入某个在线 socket”。因此严格来说，当前状态更接近“已发布/已分发到网关”，不是“客户端已收到”。

如果未来需要离线补偿或已读/已达语义，建议拆分状态：

- `pending`：待发布 MQ。
- `published`：已发布到 MQ。
- `dispatched`：已由 gateway consumer 处理。
- `delivered`：已写入在线 socket 或被客户端 ack。
- `read`：用户明确已读。

## 5. 中间件关系

### 5.1 PostgreSQL

PostgreSQL 保存核心事实数据：

- `conversations`
- `conversation_members`
- `messages`
- `message_deliveries`

每次发送消息都会产生：

- 至少 1 次会话存在校验。
- 至少 1 次成员校验。
- 1 次幂等查询。
- 1 次会话行级锁。
- 1 次 `MAX(sequence)` 查询。
- 1 次 message 插入。
- N-1 次 delivery 插入，其中 N 是会话成员数。
- N-1 次 delivery 状态更新。

高并发风险集中在同一会话：

- `lockConversationForUpdate` 会串行化同一个 conversation 的 sequence 分配。
- 大群消息会放大 delivery 插入和 MQ 发布次数。
- `MAX(sequence)` 在高并发大表下依赖索引质量，当前迁移中存在 `(conversation_id, sequence)` 唯一约束和 `sequence DESC` 索引，方向合理。

### 5.2 RabbitMQ

RabbitMQ 使用 Topic Exchange：

- exchange 默认类似 `im.events`
- dispatch queue 默认类似 `im.message.dispatch`
- binding pattern 为 `message.deliver.user.#`
- routing key 为 `message.deliver.user.<userID>`

当前单实例中所有按用户投递事件都会进入同一个 dispatch queue，再由本进程 consumer 读取并投递到本地在线连接。

注意：如果未来部署多个应用实例，并且每个实例使用同一个 queue 名称，那么 RabbitMQ 会在多个 consumer 之间竞争消费。一条投递事件可能被没有目标用户在线连接的实例消费并 ack，导致在线用户所在实例收不到该事件。多实例部署时需要改成每个 gateway 实例独立 queue，或使用 presence 路由、fanout/broadcast 加本地过滤、Redis presence、或按实例绑定的动态队列。

### 5.3 Redis

当前 WS/message 投递链路没有直接使用 Redis。Redis 主要承担认证和 session 相关能力。未来如果要支持多实例在线状态、设备路由、离线补偿扫描锁、限流等，可以引入 Redis：

- presence：`userID -> instanceID/sessionID`
- 分布式限流：按 user/device/IP 限制发消息频率。
- 幂等缓存：短期缓存 `client_message_id` 查询结果，降低数据库压力。
- 离线补偿任务锁：避免多个实例重复扫描 pending delivery。

## 6. 单用户访问资源开销模拟

假设 1 个用户建立 1 条 WebSocket 连接，不发送消息：

- 应用进程：
  - 1 条 TCP/WebSocket 连接。
  - 1 个 `ReadPump` 执行上下文。
  - 1 个 `WritePump` goroutine。
  - 1 个容量为 256 的 send channel。
  - Hub 中 2 个 map 入口引用：全局 client 集合、按 userID 集合。
- RabbitMQ：
  - 启动时已有 1 条 AMQP connection。
  - 1 条发布 channel。
  - 1 条消费 channel。
  - 没有消息发送时无新增队列消息。
- PostgreSQL：
  - 启动时已有连接池。
  - 不发送消息时不会因为 WS 空闲连接产生 DB 查询。
- Redis：
  - 建连认证阶段可能访问 session/token 信息，连接保持期间 message 链路无 Redis 开销。

当该用户发送 1 条二人会话消息：

- 应用进程执行一次 JSON decode/encode、一次事务、一次 MQ publish、一次 ACK 写入 send channel。
- PostgreSQL 产生一次事务内读写和一次事务后 delivery 状态更新。
- RabbitMQ 新增 1 条按接收人路由的持久化消息。
- 如果接收方在线，consumer 读取该消息并写入接收方 send channel。

## 7. 多用户访问资源开销模拟

### 7.1 M 个用户各 1 条连接

连接资源近似线性增长：

```text
WebSocket connections = M
ReadPump contexts     = M
WritePump goroutines  = M
send channels         = M
Hub client refs       = O(M)
```

如果每个用户还有多端登录，资源按 session 数增长，而不是按 user 数增长。

### 7.2 单个群聊 G 个成员，1 人发送 1 条消息

发送一条群消息的投递放大：

```text
message rows          = 1
delivery rows         = G - 1
RabbitMQ publishes    = G - 1
delivery updates      = G - 1
online socket writes  <= 在线接收方 session 数
```

因此大群消息的瓶颈通常不是 WebSocket 本身，而是：

- 数据库 delivery 批量写入。
- RabbitMQ publish 次数。
- 单个 `MessageService.publishToRecipients` 当前逐个串行发布。
- 单个 consumer 当前串行消费并投递。

### 7.3 多用户同时向同一会话发消息

同一 conversation 的 sequence 分配会被会话行锁串行化：

```text
User A SendMessage -> lock conversation -> allocate sequence 101 -> commit
User B SendMessage -> wait lock         -> allocate sequence 102 -> commit
User C SendMessage -> wait lock         -> allocate sequence 103 -> commit
```

这个策略保证顺序正确，但同一个超活跃会话会形成热点。对于普通单聊和小群，这是可接受的；对于万人级大群或超高 TPS 频道，需要考虑按 conversation 维护独立 sequence 表、异步批量入库、分区表或消息流系统。

## 8. 主要风险与改进建议

### 8.1 高优先级

1. `CheckOrigin` 当前无条件返回 true。生产环境应按配置校验允许的 Origin，避免跨站 WebSocket 劫持风险。
2. delivery 状态语义不准确。当前发布 RabbitMQ 成功后就标记 `delivered`，但这不等于客户端收到，建议引入 `published/dispatched/delivered/read` 等更细状态。
3. RabbitMQ publish 当前没有 publisher confirm。持久化消息发送后，如果 broker 接收失败但 TCP 层未及时暴露，可能出现数据库已提交但事件未可靠进入 broker 的风险。
4. 多实例部署时共享单一 dispatch queue 会造成错误实例消费在线投递事件。需要实例级 queue 或 presence-aware 路由。

### 8.2 中优先级

1. `publishToRecipients` 串行逐个发布，群聊会放大延迟。可以引入批处理、有限并发、或 outbox 表异步发布。
2. consumer 当前单协程串行处理，配置中存在 `ConsumeConcurrency` 但当前消费实现未使用。建议按 prefetch 和 worker 数实现并发消费，并控制每个用户/会话顺序。
3. `ReadPump` 调用 `HandleEnvelope(context.Background(), ...)`，不会继承请求取消、用户断连或服务关闭上下文。建议传入可取消上下文。
4. 发送失败后的 delivery pending 重试机制尚未看到后台扫描实现。建议增加 outbox/pending delivery retry worker。

### 8.3 低优先级

1. Envelope 常量和结构体格式可以补充协议文档，明确 type、payload schema、错误码。
2. 当前 send buffer 满时只返回错误并记录日志，后续可按策略关闭慢连接或计入指标。
3. 历史消息 HTTP 查询接口在当前代码中未发现。如果客户端需要断线后补齐消息，应补充会话列表和历史消息分页接口。

## 9. 推荐目标架构

```text
                 +----------------------+
Client           | WebSocket Route      |
  |              | Auth + Upgrade       |
  v              +----------+-----------+
ReadPump                   |
  |                         v
  |              +----------+-----------+
  +------------> | Hub                  |
                 | connection registry |
                 | local delivery      |
                 +----------+-----------+
                            |
                            v
                 +----------+-----------+
                 | MessageService      |
                 | validation          |
                 | idempotency         |
                 | persistence         |
                 +-----+----------+----+
                       |          |
                       v          v
              PostgreSQL      RabbitMQ Topic Exchange
                                    |
                                    v
                          Dispatch Queue / Instance Queue
                                    |
                                    v
                              Hub Consumer
                                    |
                                    v
                              WritePump -> Client
```

## 10. 总结

当前 WS 与 MessageService 的职责边界总体清晰：WS 管连接和在线投递，MessageService 管业务消息和持久化，RabbitMQ 管异步分发，PostgreSQL 管事实状态。该设计适合单体 IM 的开发阶段，并且已经具备幂等、会话成员校验、消息序号、per-recipient delivery 等生产级基础。

当前最需要优先补强的是可靠投递语义和生产部署安全性：限制 WebSocket Origin、引入 publisher confirm 或 outbox、修正 delivery 状态定义、明确多实例 RabbitMQ queue 拓扑。若目标是高并发群聊，还需要进一步优化 delivery 批量写入、有限并发发布、consumer worker 和热点会话 sequence 分配策略。
