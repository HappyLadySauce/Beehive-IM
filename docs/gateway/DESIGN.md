# Beehive-IM 网关模块设计文档

> 版本：v1.2
> 适用范围：Edge 公网 WebSocket 代理入口、Gateway 内网上游会话节点、连接恢复、在线态与下行推送
> 关联文件：[`docs/auth/DESIGN.md`](../auth/DESIGN.md)、[`docs/conversation/DESIGN.md`](../conversation/DESIGN.md)、[`docs/message/DESIGN.md`](../message/DESIGN.md)、[`docs/presence/DESIGN.md`](../presence/DESIGN.md)、[`docs/notification/DESIGN.md`](../notification/DESIGN.md)、[`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md)、[`api/edge.api`](../../api/edge.api)、[`proto/gateway.proto`](../../proto/gateway.proto)、[`proto/message.proto`](../../proto/message.proto)、[`proto/conversation.proto`](../../proto/conversation.proto)、[`proto/auth.proto`](../../proto/auth.proto)

---

## 1. 目标与范围

### 1.1 目标

网关模块负责 IM 客户端的**长连接接入**、**连接代理**、**上游 Gateway 选择**、**故障恢复**和**在线态管理**，采用两层架构：

| 组件 | 职责 |
|------|------|
| **Edge** | 公网唯一 WebSocket 入口；TLS 终止；JWT 入口验签；连接限流；Watch Gateway；选择和代理内网上游 Gateway；调用 Presence 注册/刷新客户端在线态；消费 RabbitMQ 下行推送 |
| **Gateway** | 内网上游会话节点；接收 Edge 转发的客户端帧；执行协议处理、会话恢复、业务上游编排；etcd 租约注册和负载上报 |

客户端连接路径：

```text
Client -> Edge(public WSS) -> Gateway(internal upstream)
```

客户端只感知 Edge 域名，不感知 Gateway 实例地址。

### 1.2 非目标

- 不承诺 Gateway 崩溃时底层 TCP/WebSocket 状态无损迁移。
- 不在 Edge 或 Gateway 长期保存消息事实，消息事实源仍为 PostgreSQL。
- 不把 Auth gRPC 服务直接暴露公网。
- 不在 v1 实现跨机房多活和端到端加密。

### 1.3 已确认架构决策

| 决策 | 选择 |
|------|------|
| 连接模型 | Client 只连接 Edge，Edge 代理到内网 Gateway |
| 公网暴露 | 生产环境公网只暴露 Edge/API Gateway 入口，Gateway 和 Auth gRPC 均为内网服务 |
| 路由策略 | Edge 基于 `user_id`、`session_id`、Gateway 负载和 region 选择上游 Gateway |
| 恢复语义 | Gateway 异常后 Edge 保持客户端连接，重新绑定其他 Gateway 并执行应用层 `resume` |
| 下行推送 | Notification 发布到 `push.edge.{edge_id}`，Edge 写入客户端 WebSocket |
| 在线态归属 | Edge 是客户端 WebSocket 连接拥有者；Presence 是在线态服务边界和 Redis key 拥有者 |
| 认证位置 | Edge 入口验签，Gateway 对 Edge 传递的 auth context 做零信任校验 |

### 1.4 当前 MVP 实现状态

当前代码已落地第一版接入层闭环，目标是验证 WebSocket 接入、Edge 代理、Gateway 会话与实时帧通道，不等同于完整生产能力：

| 能力 | 当前状态 |
|------|----------|
| Edge HTTP API | 已通过 `api/edge.api` 生成 `services/edge`，提供 `GET /healthz`、`POST /v1/ws/ticket`、`GET /ws`、`GET /v1/conversations/:conversation_id/messages`、`POST /v1/messages/sync`、`POST /v1/messages/ack` |
| WebSocket ticket | Edge 本地内存存储，TTL 默认 30s，单次消费，绑定 `user_id`、`device_id`、`session_id`、Origin |
| 开发鉴权 | `POST /v1/ws/ticket` 暂用 `X-Debug-User-Id` 生成 ticket，后续替换为 Auth/JWT 校验 |
| Gateway zRPC | 已通过 `proto/gateway.proto` 生成 `services/gateway`，包含 `Attach`、`Resume`、`CloseSession`、`Stream` |
| Edge -> Gateway | 已采用 gRPC bidirectional stream 转发 JSON WebSocket 信封 |
| Gateway 会话 | 当前为内存 session manager，支持 attach、resume、close、容量限制和 ping/pong/session.resumed |
| Gateway rebind | Edge 已支持短窗口多节点 rebind/resume：上游 stream 断开后隔离故障 Gateway、按配置重试多个 Gateway、调用 `Resume`、更新 Presence route 并向客户端发送 `session.resumed` |
| Gateway draining | Gateway 收到 SIGINT/SIGTERM 后更新 etcd 状态为 `draining`；Edge Watch 到后停止新分配，并主动迁移本机已绑定连接 |
| Presence | 已生成 `services/presence`，Edge 通过 zRPC 调用 Presence，在线态写入 Redis |
| Presence refresh | Edge 收到客户端业务帧后调用 Presence `RefreshConnection`，刷新 `conn:meta` 与 `session:route` TTL |
| Gateway 选择 | Gateway 注册到 etcd，Edge watch `/beehive-im/{env}/services/gateway/` 并回退静态 Gateway endpoint |
| RabbitMQ push | Edge 已消费 RabbitMQ `edge.push.{edge_id}` 队列，绑定 routing key `push.edge.{edge_id}`，按 `conn_id` 或 `session_id` 写入本机 WebSocket |
| User PostgreSQL | User 服务已接 PostgreSQL，`GetUser` 从 `users` / `user_profiles` 读取 |
| Message/Conversation | Gateway 已接入 Message zRPC；`message.send` 持久化消息并返回 `message.persisted`，`message.ack` 写入回执并返回 `message.ack` |
| 未知业务帧 | 未识别帧仍按 echo 验证链路处理 |
| Notification | 已新增 `services/notification`，消费 `message.created.#`，查询 Conversation + Presence，并发布 `message.new` 在线 push |

### 1.5 关键约束

- Edge 故障时客户端仍必须重连，因为客户端 TCP/WebSocket 连接终止在 Edge。
- Gateway 故障时 Edge 可以屏蔽大部分客户端重连，但必须依赖 `session_id`、`last_seq` 和 Message 同步接口补偿缺失消息。
- Edge 到 Gateway 的上游连接必须有背压、超时、熔断和快速切换机制，禁止无限制堆积帧。

---

## 2. 总体架构

```mermaid
flowchart TB
    subgraph clients [Clients]
        Client[IM Client]
    end

    subgraph public_edge [Public Edge Layer]
        Edge1[Edge A]
        Edge2[Edge B]
    end

    subgraph internal_gateway [Internal Gateway Layer]
        GW1[Gateway A]
        GW2[Gateway B]
        GW3[Gateway C]
    end

    subgraph backend [Backend]
        Auth[Auth Service]
        Etcd[(etcd)]
        Presence[Presence Service]
        Notification[Notification Service]
        Redis[(Redis)]
        RMQ[(RabbitMQ)]
        Msg[Message Service]
        PG[(PostgreSQL)]
    end

    Client -->|"1 Login HTTP via public API"| Edge1
    Edge1 -->|"proxy /v1/auth/*"| Auth
    Client -->|"2 WSS /ws + ticket"| Edge1
    Edge1 -->|"3 internal stream"| GW2
    Edge1 --> Etcd
    GW2 --> Etcd
    Edge1 --> Presence
    Presence --> Redis
    Msg --> PG
    Msg -->|"message.created"| RMQ
    RMQ -->|"message.created"| Notification
    Notification --> Presence
    Notification -->|"push.edge.edge-1"| RMQ
    RMQ -->|"push.edge.edge-1"| Edge1
    Edge1 -->|"4 WebSocket frame"| Client
```

### 2.1 与 Auth 模块的衔接

- 客户端登录、注册、刷新令牌通过公网 API 入口访问，入口可以由 Edge 或独立 API Gateway 承载。
- Auth gRPC 服务保持内网暴露，只被 Edge/API Gateway/BFF 等可信入口调用。
- Edge 与 Gateway 从 etcd `secrets/global/jwt.secret` 读取同一验签密钥；v1 密钥变更通过滚动重启生效。
- Web 客户端先通过 HTTPS 申请一次性 `ws_ticket`，再用 `wss://im.example.com/ws?ticket=...` 建立连接；Edge 在握手阶段消费 ticket 并还原 auth context。
- Gateway 仍需校验 Edge 传递的签名上下文或内部 mTLS 身份，避免盲信上游。
- JWT 载荷约定见 [`docs/auth/DESIGN.md`](../auth/DESIGN.md) 第 4.1 节：`sub`、`username`、`iat`、`exp`、`jti`。

### 2.2 组件职责边界

| 组件 | 做 | 不做 |
|------|----|------|
| Edge | 公网 WSS、JWT 入口验签、连接保持、上游代理、Gateway rebind、在线态、RabbitMQ push 消费 | 消息持久化、长期业务状态、OAuth secret 处理 |
| Gateway | 内网上游协议处理、会话恢复、业务帧编排、负载上报 | 公网暴露、保存客户端在线事实、直接写客户端 TCP |
| Auth | 签发/撤销 JWT、refresh token、OAuth 流程 | 长连接管理、消息持久化 |
| Message | 消息持久化、会话序号、Outbox、消息同步 | WebSocket 连接管理 |
| Presence | 在线态、session route、Gateway rebind 路由更新 | WebSocket 连接代理、消息持久化 |
| Notification | 消费消息/会话事件，编排在线 push 和离线通知 | 消息事实存储、客户端连接管理 |

---

## 3. 连接全流程

```mermaid
sequenceDiagram
    participant C as Client
    participant E as Edge
    participant A as Auth
    participant EC as etcd
    participant P as Presence
    participant G as Gateway

    C->>E: POST /v1/auth/login
    E->>A: internal Auth RPC
    A-->>E: access_token + refresh_token
    E-->>C: login response

    C->>E: WSS /ws?ticket=...
    E->>E: verify origin and consume ws_ticket
    E->>EC: read Gateway view from Watch cache
    E->>E: select Gateway
    E->>G: internal attach(session_id, auth_context)
    G-->>E: attach accepted
    E->>P: UpsertConnection(edge_id, gateway_id, session_id)
    E-->>C: 101 Switching Protocols

    loop normal frames
        C->>E: client frame
        E->>G: upstream frame
        G-->>E: downstream frame
        E-->>C: client frame
    end
```

### 3.1 客户端连接状态机

```mermaid
stateDiagram-v2
    [*] --> Disconnected
    Disconnected --> Authenticating: Auth Login OK
    Authenticating --> ConnectingEdge: WSS /ws
    ConnectingEdge --> Connected: Edge handshake OK
    Connected --> Connected: ping/pong and message frames
    Connected --> Recovering: Gateway upstream failed
    Recovering --> Connected: Edge rebind + resume OK
    Recovering --> Disconnected: Edge cannot recover
    Connected --> RefreshingToken: token expired
    RefreshingToken --> ConnectingEdge: refresh OK
    Connected --> Disconnected: Edge closed or network failed
```

### 3.2 Gateway 故障恢复流程

```mermaid
sequenceDiagram
    participant C as Client
    participant E as Edge
    participant EC as etcd
    participant G1 as Gateway A
    participant G2 as Gateway B
    participant P as Presence

    C->>E: active WebSocket
    E->>G1: upstream frames
    G1--xE: upstream broken
    E->>E: pause upstream writes and start bounded buffer
    E->>EC: pick healthy Gateway
    E->>G2: resume(session_id, user_id, device_id, last_client_seq, last_delivered_seq)
    G2-->>E: resume accepted
    E->>P: RebindGateway(session_id, gateway_b)
    E->>G2: flush bounded buffer
    E-->>C: optional session.resumed
```

恢复要求：

- Edge 的恢复窗口由 `GatewayRecovery.WindowMs` 控制，默认 5s。
- Edge 只允许有限缓冲客户端上行帧，默认按连接 64 到 256 条或固定字节数限制。
- Edge 默认最多尝试 3 个 Gateway，退避为 50ms、100ms、200ms，失败 Gateway 默认隔离 10s。
- 恢复失败时 Edge 关闭客户端连接，客户端按正常重连流程进入 Message 同步补偿。
- `resume` 成功不代表消息无缺口，客户端仍应按 `conversation_id + seq` 同步缺失消息。

---

## 4. Edge 服务设计

### 4.1 公网 HTTP API

Edge 可以承载 Auth HTTP 入口，也可以与独立 API Gateway 并存。生产建议统一公网 API 域名：

```text
https://api.example.com/v1/auth/*
wss://im.example.com/ws
```

Auth gRPC 服务不直接公网暴露。

### 4.2 WebSocket 端点

```http
GET /ws?ticket={ws_ticket} HTTP/1.1
Host: im.example.com
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Protocol: beehive.im.v1
```

浏览器原生 WebSocket 不能设置自定义 `Authorization` 或 `X-Device-Id` 头。Web 端必须先通过 `POST /v1/ws/ticket` 获取一次性短 TTL ticket；生产环境禁止把 access token / refresh token 放入 query，只有单次使用的 `ws_ticket` 可以出现在 WebSocket URL 中。

### 4.3 握手认证流程

```mermaid
flowchart TD
    A[收到 WS Upgrade 请求] --> B{TLS and Origin valid?}
    B -->|否| R403[403 Forbidden]
    B -->|是| C{解析 Authorization}
    C -->|缺失| R401[401 Unauthorized]
    C -->|存在| D{ticket valid and unused?}
    D -->|失败/过期| R401
    D -->|成功| E{jti blacklist?}
    E -->|命中| R401
    E -->|未命中| F{device_id valid?}
    F -->|否| R400[400 Bad Request]
    F -->|是| G{edge_conn_count < max_conn?}
    G -->|否| R503[503 Service Unavailable]
    G -->|是| H[选择 Gateway 并 attach]
    H --> I[调用 Presence 注册在线态]
    I --> J[101 Switching Protocols]
```

### 4.4 Gateway 选择

输入：

- `user_id`：JWT `sub`
- `device_id`
- `session_id`：客户端恢复时传入，缺失则 Edge 生成
- Gateway 列表：来自 etcd Watch `/beehive-im/{env}/services/gateway/`
- Gateway 过滤条件：`status=online`、Lease 有效、`session_count < max_sessions`、region/zone 匹配

算法：

1. 如果 Presence 中存在同一 `session_id` 的健康 `gateway_id`，优先复用。
2. 否则对 `user_id + device_id` 做 rendezvous hash 或一致性哈希。
3. 对候选 Gateway 按负载进行轻量惩罚，避免把所有新连接压到单点。
4. Gateway attach 失败时在同一次握手内重试下一个候选，重试次数必须有限。

### 4.5 Edge 连接模型

```mermaid
erDiagram
    User ||--o{ EdgeConnection : owns
    Edge ||--o{ EdgeConnection : hosts
    Gateway ||--o{ UpstreamSession : serves

    EdgeConnection {
        string conn_id PK
        string session_id
        bigint user_id
        string device_id
        string edge_id
        string gateway_id
        int64 last_client_seq
        int64 last_delivered_seq
        timestamptz connected_at
    }
```

| 概念 | 说明 |
|------|------|
| `conn_id` | Edge 内唯一，表示客户端到 Edge 的 WebSocket 连接 |
| `session_id` | 应用层会话 ID，Gateway rebind/resume 的核心标识 |
| `edge_id` | 当前承载客户端连接的 Edge 实例 |
| `gateway_id` | 当前绑定的内网上游 Gateway，可在连接生命周期内切换 |
| `device_id` | 客户端设备标识，同一用户可多设备在线 |

### 4.6 Edge 代理要求

- 客户端读写和 Gateway 上游读写必须分离 goroutine，并通过有界 channel 传递。
- 上游 Gateway 写阻塞不得阻塞客户端心跳处理。
- 每条连接必须有独立背压策略，慢客户端或慢 Gateway 都必须被限流或断开。
- Edge 关闭连接时必须先调用 Presence 清理在线态，再关闭本地资源。
- Edge 进程退出时先进入 draining，停止接受新连接，尝试将现有连接通知客户端重连。

### 4.7 Edge 服务注册

Edge 也需要注册到 etcd，供运维观测、Notification push 队列声明和后续跨 Edge 调度使用。

```text
/beehive-im/{env}/services/edge/{edge_id}
```

Value 示例：

```json
{
  "schema_version": 1,
  "instance_id": "edge-01",
  "service": "edge",
  "public_ws_url": "wss://im.example.com/ws",
  "status": "online",
  "conn_count": 2048,
  "max_conn": 50000,
  "region": "cn-east",
  "zone": "cn-east-a",
  "version": "v1.1.0",
  "updated_at": "2026-06-20T08:01:00Z"
}
```

---

## 5. Gateway 服务设计

### 5.1 内网上游端点

Gateway 不暴露公网 WebSocket 地址，只暴露内网 upstream 端点：

```text
gateway.internal:9100
```

v1 可使用 internal WebSocket 或 gRPC bidirectional stream。生产推荐 gRPC stream，便于 mTLS、流控、deadline 和结构化错误。

### 5.2 Gateway attach/resume

Edge 向 Gateway 建立会话时传递：

| 字段 | 说明 |
|------|------|
| `session_id` | 应用层会话 ID |
| `conn_id` | Edge 连接 ID |
| `edge_id` | Edge 实例 ID |
| `user_id` | JWT `sub` |
| `device_id` | 客户端设备 ID |
| `jti` | JWT ID，用于黑名单校验 |
| `last_client_seq` | Edge 已接收的最大客户端帧序号 |
| `last_delivered_seq` | Edge 已投递到客户端的最大服务端帧序号 |

Gateway 必须校验：

- Edge 的 mTLS 身份或内部服务 token。
- auth context 签名或 JWT 是否有效。
- `session_id` 是否被同一 Edge 合法持有。
- 本机 `session_count < max_sessions`。

### 5.3 Gateway 会话模型

Gateway 保存的是可重建的上游会话状态，不能作为客户端在线事实源。Gateway 崩溃后，Edge 可以在其他 Gateway 上重建会话。

| 状态 | 存放位置 | 是否权威 |
|------|----------|----------|
| 客户端 WebSocket 连接 | Edge 内存 | 是 |
| 在线态索引 | Presence（Redis 内部存储） | 是，短期运行态 |
| Gateway 当前会话 | Gateway 内存 | 否，可重建 |
| 消息事实 | PostgreSQL | 是 |

### 5.4 Gateway 注册与心跳（etcd）

Gateway 启动后向 **etcd** 注册租约，并周期性 KeepAlive 上报负载；客户端在线态由 **Presence** 管理，Edge 只调用 Presence API。详见 [`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md)。

**生命周期**

```mermaid
stateDiagram-v2
    [*] --> Starting
    Starting --> Online: etcd Put + Lease
    Online --> Online: KeepAlive 更新 session_count
    Online --> Draining: 收到 SIGTERM
    Draining --> Offline: 会话归零
    Online --> Offline: Lease 过期
    Offline --> [*]
```

| 阶段 | etcd `status` | Edge 行为 |
|------|---------------|-----------|
| 正常运行 | `online` | 可选择为新连接或恢复目标 |
| 优雅下线 | `draining` | 不分配新会话，已有上游会话逐步迁移或自然结束 |
| 宕机 | Lease 过期 key 删除 | 从 Watch 视图移除，并触发受影响会话 rebind |

**KeepAlive 间隔**：10s（Lease TTL 30s）；字段包括 `session_count`、`upstream_addr`、`max_sessions`。

---

## 6. 存储模型

服务注册走 **etcd**，客户端在线态走 **Presence 服务**，下行推送走 **RabbitMQ 到 Edge**。Redis key 是 Presence 的内部实现细节，Edge 和 Gateway 不直接读写。分工见 [`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md) 与 [`docs/presence/DESIGN.md`](../presence/DESIGN.md)。

### 6.1 etcd：Gateway 服务注册

```
/beehive-im/{env}/services/gateway/{gateway_id}
```

Value 为 JSON，含 `upstream_addr`、`status`、`session_count`、`max_sessions`、`region` 等；绑定 **Lease TTL 30s**，KeepAlive 10s。

Edge 通过 **Watch** 该前缀维护内存 Gateway 视图。

### 6.2 etcd：Edge 服务注册

```
/beehive-im/{env}/services/edge/{edge_id}
```

Value 为 JSON，含 `public_ws_url`、`status`、`conn_count`、`max_conn`、`region` 等；绑定 **Lease TTL 30s**，KeepAlive 10s。

### 6.3 Presence：在线态

#### Key 一览

以下 key 由 Presence 服务独占写入，Edge 只能通过 Presence API 操作。

| Key | 类型 | TTL | 说明 |
|-----|------|-----|------|
| `conn:user:{user_id}` | HASH | 无 | `device_id` -> `{edge_id}:{conn_id}:{session_id}`，只作为索引 |
| `conn:edge:{edge_id}` | SET | 无 | 当前 Edge 上的 `conn_id` 集合，只作为索引 |
| `conn:meta:{conn_id}` | HASH | 90s（ping 续期） | 连接元数据；在线判定以该 key 存在为准 |
| `session:route:{session_id}` | HASH | 90s（ping 续期） | 当前 `edge_id`、`conn_id`、`gateway_id` 路由 |

#### 字段定义

**`conn:meta:{conn_id}`**

| 字段 | 说明 |
|------|------|
| `user_id` | 用户 ID |
| `device_id` | 设备 ID |
| `edge_id` | 所属 Edge |
| `gateway_id` | 当前绑定 Gateway |
| `session_id` | 应用层会话 ID |
| `connected_at` | 连接建立时间 |
| `last_seen_at` | 最近心跳或业务帧时间 |
| `last_client_seq` | Edge 已接收客户端最大序号 |
| `last_delivered_seq` | Edge 已投递客户端最大序号 |

### 6.4 在线态写入（连接建立）

Edge 必须调用 Presence `UpsertConnection`。Presence 通过 Redis Lua 脚本原子完成：

1. 读取同一 `user_id + device_id` 的旧连接路由。
2. 写入新的 `device_id -> edge_id:conn_id:session_id` 索引。
3. 写入 `conn:edge:{edge_id}` 集合。
4. 写入 `conn:meta:{conn_id}` 和 `session:route:{session_id}` 并设置 TTL。
5. 返回旧连接路由，由对应 Edge 关闭旧连接或等待 TTL 清理。

### 6.5 Gateway rebind

Gateway 故障恢复时，Edge 调用 Presence `RebindGateway`，Presence 使用 compare-and-set Lua 更新 `gateway_id`：

```text
if HGET session:route:{session_id} edge_id == {edge_id}
and HGET session:route:{session_id} conn_id == {conn_id}
then HSET session:route:{session_id} gateway_id {new_gateway_id}
     HSET conn:meta:{conn_id} gateway_id {new_gateway_id}
     EXPIRE session:route:{session_id} {presence_ttl}
     EXPIRE conn:meta:{conn_id} {presence_ttl}
else return stale_session
```

### 6.6 在线态清理（连接断开）

Edge 必须调用 Presence `RemoveConnection`。Presence 通过 compare-and-delete Lua 脚本完成：只有 `conn:user:{user_id}` 中当前值仍等于 `{edge_id}:{conn_id}:{session_id}` 时，才允许删除该 `device_id` 字段，避免同设备快速重连时旧连接误删新连接。

### 6.7 消息推送路由（RabbitMQ）

Notification 服务向在线用户推送时：

1. 从 RabbitMQ 消费 `message.created.#` 或 `conversation.updated.#` 领域事件。
2. 调用 Conversation 或使用事件快照解析收件人和通知偏好。
3. 调用 Presence `GetLiveRoutes(user_id)` 获取在线设备路由。
4. 当前按在线 route 逐条发布到目标 `edge_id`；后续可按 `edge_id` 聚合减少 publish 次数。
5. 向 RabbitMQ exchange `beehive.im.push` 发布在线推送通知，routing key `push.edge.{edge_id}`。
6. 目标 Edge 从队列 `edge.push.{edge_id}` 消费，经本地 `conn_id` 写入 WebSocket。

RabbitMQ push 只作为在线通知，消息事实源仍是 PostgreSQL；Edge 或客户端断线后必须能通过 Message 服务按会话序号同步缺失消息。

详见 [`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md) 第 5 节。

---

## 7. WebSocket 消息帧

### 7.1 信封格式

所有应用层消息使用 JSON 信封：

```json
{
  "type": "ping | pong | ack | error | session.resumed | message | ...",
  "seq": 1,
  "payload": {}
}
```

| 字段 | 说明 |
|------|------|
| `type` | 消息类型 |
| `seq` | 单调递增序号，用于去重、ack 和恢复 |
| `payload` | 业务载荷 |

### 7.2 系统帧

**ping / pong**

```json
{ "type": "ping", "seq": 42, "payload": {} }
{ "type": "pong", "seq": 42, "payload": {} }
```

**session.resumed**

```json
{
  "type": "session.resumed",
  "seq": 0,
  "payload": {
    "session_id": "sess-01",
    "gateway_id": "gw-03",
    "last_delivered_seq": 1024
  }
}
```

**error（服务端主动关闭前）**

```json
{
  "type": "error",
  "seq": 0,
  "payload": {
    "code": "KICKED_DUPLICATE_DEVICE",
    "message": "Same device connected elsewhere"
  }
}
```

| `code` | 场景 |
|--------|------|
| `KICKED_DUPLICATE_DEVICE` | 同设备新连接踢掉旧连接 |
| `TOKEN_EXPIRED` | 服务端强制校验失败（v1.1） |
| `SERVER_DRAINING` | Edge 或 Gateway 优雅下线 |
| `UPSTREAM_RECOVER_FAILED` | Edge 无法重新绑定 Gateway |
| `INVALID_FRAME` | 帧格式错误 |
| `MESSAGE_SERVICE_UNAVAILABLE` | Gateway 未配置或无法访问 Message 服务 |
| `MESSAGE_SEND_FAILED` | 调用 Message `SendMessage` 失败 |
| `MESSAGE_ACK_FAILED` | 调用 Message `AckMessages` 失败 |

### 7.3 业务帧

当前 Gateway 已实现 `message.send` 和 `message.ack`，其余未知业务帧仍按 echo 返回，用于接入链路验证。

**message.send**

```json
{
  "type": "message.send",
  "seq": 101,
  "payload": {
    "conversation_id": "conv_01",
    "client_msg_id": "client_msg_01",
    "content_type": "text",
    "content": {
      "text": "hello"
    }
  }
}
```

Gateway 从本机 session 中读取 `user_id` 和 `device_id`，客户端不能通过 payload 覆盖发送者身份。

**message.persisted**

```json
{
  "type": "message.persisted",
  "seq": 201,
  "payload": {
    "message_id": "msg_01",
    "conversation_id": "conv_01",
    "message_seq": 1024,
    "sender_id": "user_01",
    "content_type": "text",
    "created_at": "2026-06-21T00:00:00Z",
    "duplicate": false
  }
}
```

**message.ack**

```json
{
  "type": "message.ack",
  "seq": 202,
  "payload": {
    "conversation_id": "conv_01",
    "ack_type": "read",
    "seqs": [1024, 1025]
  }
}
```

成功响应：

```json
{
  "type": "message.ack",
  "seq": 203,
  "payload": {
    "conversation_id": "conv_01",
    "ack_type": "read",
    "updated": 2
  }
}
```

### 7.4 认证与消息分离

- **认证**：Web 端通过一次性 `ws_ticket` 在 HTTP Upgrade 阶段完成
- **业务消息**：握手成功后才开始收发；不在 WS 帧中传递 token
- **上游恢复**：Gateway rebind 使用 Edge 内部 auth context，不向客户端暴露 Gateway 地址

---

## 8. 重连与故障转移

| 场景 | 客户端行为 | Edge 行为 | Gateway 行为 |
|------|------------|-----------|--------------|
| 客户端到 Edge 网络闪断 | 重新申请 `ws_ticket` 后重连 `wss://im.example.com/ws` | 清理旧连接或等待 TTL | 无 |
| Gateway 上游断开 | 保持连接，等待恢复或收到错误帧 | 选择新 Gateway，执行 `resume` | 新 Gateway 重建会话 |
| Gateway draining | 无感或收到 `session.resumed` | 主动 rebind 到健康 Gateway | 停止接新会话 |
| Edge 进程退出 | 客户端重连 Edge 域名 | draining 后关闭连接 | 上游会话释放 |
| access_token 过期 | 刷新 token、重新申请 `ws_ticket` 后重连 Edge | 握手阶段拒绝旧 ticket | 无 |
| RabbitMQ 推送延迟 | 客户端按 seq 同步缺失消息 | 背压和告警 | 无 |

**客户端兜底重连策略**

1. 等待 `1s -> 2s -> 4s -> 8s`，上限 30s。
2. 始终重连统一 Edge 域名，不请求 Gateway 地址。
3. 每次重连都重新申请 `ws_ticket`，不复用旧 ticket。
4. 重连成功后由 Web 客户端按本地 cursor 调用 Message 同步接口补齐缺失消息。

---

## 9. 安全要求

| 项 | 要求 |
|----|------|
| 公网暴露 | 只暴露 Edge/API Gateway；Gateway、Auth gRPC、Presence、Notification、Redis、etcd、RabbitMQ 均为内网 |
| 传输 | Client -> Edge 使用 HTTPS/WSS；Edge -> Gateway 使用 mTLS |
| Token 传递 | Web 客户端使用 HTTPS 认证会话换取一次性 `ws_ticket`；生产禁止 query access token |
| 入口验签 | Edge 在 Upgrade 前校验 Origin、消费 `ws_ticket` 并还原 auth context |
| 内部零信任 | Gateway 校验 Edge mTLS 身份和 auth context |
| 连接上限 | Edge `max_conn`、Gateway `max_sessions` 双重限制 |
| 黑名单 | v1.1 支持 `jwt:bl:{jti}` 实现登出后即时失效 |
| 日志 | 日志使用英文，禁止记录 token、OAuth code、secret 和完整用户隐私字段 |

---

## 10. 配置项

### 10.1 Edge

| 环境变量 / 配置 | 说明 | 示例 |
|-----------------|------|------|
| `ETCD_ENDPOINTS` | etcd 端点（bootstrap） | `127.0.0.1:2379` |
| `BEEHIVE_ENV` | 环境名 | `dev` |
| `EDGE_ID` | 实例唯一 ID | `edge-01` |
| `EDGE_LISTEN` | 公网 HTTP/WSS 监听地址 | `:8080` |
| `config/edge/public_ws_url` | 公网 WebSocket 地址 | `wss://im.example.com/ws` |
| `config/edge/ws.ping_interval` | 期望客户端 ping 间隔 | `30s` |
| `config/edge/ws.read_timeout` | 读超时 | `60s` |
| `config/edge/ws.ticket_ttl` | WebSocket 一次性 ticket TTL | `30s` |
| `config/edge/max_conn` | 单实例最大客户端连接数 | `50000` |
| `config/edge/upstream.connect_timeout` | 连接 Gateway 超时 | `2s` |
| `config/edge/gateway.resume_timeout` | Gateway 切换恢复窗口 | `5s` |
| `config/edge/upstream_buffer.messages` | 单连接上游恢复缓冲条数 | `128` |
| `config/presence/target` | Presence gRPC 目标地址 | `presence.internal:9200` |
| `secrets/global/jwt.secret` | JWT 验签密钥（etcd） | — |
| `RABBITMQ_URL` | 消费 Edge push 队列 | `amqp://guest:guest@127.0.0.1:5672/` |
| `PRESENCE_GRPC_TARGET` | 本地开发 Presence 地址回退 | `127.0.0.1:9200` |

### 10.2 Gateway

| 环境变量 / 配置 | 说明 | 示例 |
|-----------------|------|------|
| `ETCD_ENDPOINTS` | etcd 端点（bootstrap） | `127.0.0.1:2379` |
| `BEEHIVE_ENV` | 环境名 | `dev` |
| `GATEWAY_ID` | 实例唯一 ID | `gw-02` |
| `GATEWAY_UPSTREAM_LISTEN` | 内网上游监听地址 | `:9100` |
| `config/gateway/max_sessions` | 最大上游会话数 | `20000` |
| `config/gateway/session.idle_timeout` | 上游会话空闲超时 | `120s` |
| `config/gateway/message.target` | Message zRPC 目标地址 | `127.0.0.1:9400` |
| `secrets/global/jwt.secret` | Gateway 校验 auth context 使用 | — |

本地基础设施见 [`docker/Infrastructure/docker-compose.yaml`](../../docker/Infrastructure/docker-compose.yaml)（PostgreSQL、Redis、etcd、RabbitMQ）。

---

## 11. 服务目录建议

```
services/edge/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── server/
│   │   └── http.go
│   ├── ws/
│   │   ├── upgrade.go
│   │   ├── connection.go
│   │   └── pump.go
│   ├── upstream/
│   │   ├── gateway_client.go
│   │   ├── router.go
│   │   └── resume.go
│   ├── presence/
│   │   └── client.go
│   ├── push/
│   │   └── rabbitmq.go
│   └── auth/
│       └── jwt.go                # 本地验签
└── pkg/

services/gateway/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── upstream/
│   │   ├── server.go
│   │   ├── attach.go
│   │   └── resume.go
│   ├── session/
│   │   └── manager.go
│   ├── registry/
│   │   └── heartbeat.go          # etcd 租约注册与 KeepAlive
│   └── frame/
│       └── codec.go              # JSON 信封编解码
└── pkg/
```

### 11.1 关键接口

```go
// GatewayRegistry watches internal gateway instances from etcd.
// GatewayRegistry 从 etcd 监听内网 Gateway 实例。
type GatewayRegistry interface {
    Watch(ctx context.Context) (<-chan GatewayEvent, error)
    List(ctx context.Context) ([]GatewayNode, error)
}

// GatewayRouter picks an internal gateway for attach or resume.
// GatewayRouter 为 attach 或 resume 选择内网 Gateway。
type GatewayRouter interface {
    Pick(ctx context.Context, session SessionContext, nodes []GatewayNode) (*GatewayNode, error)
}

// PresenceClient tracks client connections through Presence service APIs.
// PresenceClient 通过 Presence 服务 API 维护客户端连接状态。
type PresenceClient interface {
    UpsertConnection(ctx context.Context, conn EdgeConnection) (previous *ConnectionRoute, err error)
    RebindGateway(ctx context.Context, sessionID, edgeID, connID, gatewayID string) error
    RemoveConnection(ctx context.Context, conn EdgeConnection) (removed bool, err error)
    RefreshConnection(ctx context.Context, edgeID, connID string, ttl time.Duration) error
    GetLiveRoutes(ctx context.Context, userID string) ([]ConnectionRoute, error)
}

// UpstreamSession proxies frames between Edge and Gateway.
// UpstreamSession 在 Edge 和 Gateway 之间代理帧。
type UpstreamSession interface {
    Attach(ctx context.Context, session SessionContext) error
    Resume(ctx context.Context, session SessionContext) error
    Send(ctx context.Context, frame Frame) error
    Close(ctx context.Context) error
}
```

---

## 12. 与其他服务协作

```mermaid
flowchart LR
    Client[Client]
    Auth[Auth Service]
    Edge[Edge]
    Gateway[Gateway]
    User[User Service]
    Msg[Message Service]
    Presence[Presence Service]
    Notification[Notification Service]

    Client --> Edge
    Edge --> Auth
    Edge --> Gateway
    Edge --> Presence
    Presence --> Redis[(Redis)]
    Gateway --> Etcd[(etcd)]
    Edge --> Etcd
    Msg --> PG[(PostgreSQL)]
    Msg -.->|message.created| RMQ[(RabbitMQ)]
    RMQ -.->|message.created| Notification
    Notification --> Presence
    Notification -.->|push.edge| RMQ
    RMQ -.->|push.edge| Edge
    User --> PG
```

| 服务 | 协作方式 |
|------|----------|
| **Auth** | Edge/API Gateway 代理公网 auth 请求；Auth gRPC 内网暴露 |
| **User** | 不直接参与连接；资料查询独立走 gRPC |
| **Message** | 持久化消息并发布领域事件；客户端按 seq 补偿同步 |
| **Presence** | 管理在线态和 session route；Edge 负责写入，Notification 负责查询 |
| **Notification** | 消费消息/会话事件，查询 Presence，并通过 RabbitMQ 向 Edge 推送在线通知 |
| **Redis** | Presence 内部短 TTL 在线态存储，不暴露给 Edge/Gateway |
| **etcd** | Gateway/Edge/Auth 服务注册；运行时配置与密钥 |
| **RabbitMQ** | 领域事件；Notification -> Edge 下行通知 |

中间件职责详见 [`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md)。

详见 [`docs/auth/DESIGN.md`](../auth/DESIGN.md)。

---

## 13. 运维

| 任务 | 方式 |
|------|------|
| Edge 扩容 | 新增实例，注册 etcd，前置 LB 自动分流新连接 |
| Edge 缩容 | 进入 draining，停止接新连接，通知客户端重连或等待连接自然结束 |
| Gateway 扩容 | 新增实例，注册 etcd，Edge Watch 后可选择为新 attach/resume 目标 |
| Gateway 优雅下线 | `status=draining`，Edge 不再分配新会话，并逐步 rebind 现有会话 |
| Gateway 异常 | Lease 过期后 Edge 移除实例，受影响 session 执行 rebind/resume |
| 僵尸连接清理 | `conn:meta` TTL 过期 + Edge 读超时 + Presence 定时修复任务 |
| 监控指标 | `edge_conn_count`、`gateway_session_count`、`upstream_rebind_total`、`resume_fail_total`、`presence_refresh_latency`、`push_ack_latency` |

---

## 14. 演进路线

| 阶段 | 内容 |
|------|------|
| v1（当前） | Edge 代理 WebSocket；Gateway 内网上游；Presence 在线态；Message 持久化/同步；Notification 在线 push |
| v1.1 | JWT 黑名单；Presence 在线态清理任务；Notification DLQ/指标；基础观测面板 |
| v2 | Gateway 选择引入 rendezvous hash + 权重；Web Push / 移动 provider 扩展；mTLS 自动证书轮换 |
| v3 | RS256 + JWKS；跨 region Edge；Gateway 多活；事件总线升级评估 |

---

## 15. 附录：HTTP / WS 错误码

### Edge HTTP / WebSocket 握手

| 状态码 | `error_code` | 说明 |
|--------|--------------|------|
| 400 | `INVALID_DEVICE_ID` | device_id 缺失或非法 |
| 401 | `INVALID_TOKEN` | JWT 无效或过期 |
| 403 | `INVALID_ORIGIN` | Origin 不在白名单 |
| 503 | `NO_GATEWAY_AVAILABLE` | 无健康节点 |
| 503 | `EDGE_CAPACITY_EXCEEDED` | Edge 连接数已满 |

### Gateway 内部 attach/resume

| code | 说明 |
|------|------|
| `UNAUTHENTICATED_EDGE` | Edge 内部身份无效 |
| `SESSION_CAPACITY_EXCEEDED` | Gateway 会话数已满 |
| `SESSION_STALE` | session 路由已过期或被新连接替换 |
| `RESUME_REJECTED` | Gateway 拒绝恢复 |

### 应用层 `error` 帧 `code`

| code | 说明 |
|------|------|
| `KICKED_DUPLICATE_DEVICE` | 同设备被新连接踢下线 |
| `SERVER_DRAINING` | Edge 或 Gateway 优雅下线 |
| `UPSTREAM_RECOVER_FAILED` | Edge 无法恢复上游 Gateway |
| `INVALID_FRAME` | 帧格式错误 |
