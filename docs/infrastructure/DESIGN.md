# Beehive-IM 基础设施与中间件设计

> 版本：v1.2
> 适用范围：PostgreSQL、Redis、etcd、RabbitMQ、Conversation、Presence、Notification 的生产级职责划分、接口契约、数据流、容灾与运维约定
> 关联文件：[`docs/auth/DESIGN.md`](../auth/DESIGN.md)、[`docs/gateway/DESIGN.md`](../gateway/DESIGN.md)、[`docs/conversation/DESIGN.md`](../conversation/DESIGN.md)、[`docs/message/DESIGN.md`](../message/DESIGN.md)、[`docs/presence/DESIGN.md`](../presence/DESIGN.md)、[`docs/notification/DESIGN.md`](../notification/DESIGN.md)、[`docker/Infrastructure/docker-compose.yaml`](../../docker/Infrastructure/docker-compose.yaml)

---

## 1. 目标与设计原则

Beehive-IM 的基础设施层负责为 Auth、User、Edge、Gateway、Conversation、Message、Presence、Notification 等服务提供稳定的持久化、运行态缓存、服务发现、配置管理和异步投递能力。基础设施层必须服务于 IM 系统的高并发、低延迟、可恢复和可观测目标。

### 1.1 生产级目标

| 目标 | 要求 |
|------|------|
| 职责清晰 | 每类中间件只承担明确职责，禁止把同一类状态写入多个来源 |
| 故障可恢复 | 单实例崩溃、网络闪断、消息发布失败时，系统能靠事实源恢复 |
| 高并发 | 在线态和下行推送必须使用异步非阻塞 IO、批量操作和背压控制 |
| 一致性可解释 | PostgreSQL 是业务事实源；Redis 和 RabbitMQ 的状态允许短暂不一致，但必须可校验、可清理 |
| 安全可审计 | 生产环境必须启用 TLS、最小权限、secret 隔离、审计日志和敏感信息脱敏 |
| 可观测 | 所有关键路径必须有指标、结构化日志、trace 和告警阈值 |

### 1.2 基础设施职责

```mermaid
flowchart TB
    subgraph persist [持久化事实源]
        PG[(PostgreSQL)]
    end

    subgraph coord [协调与配置]
        Etcd[(etcd)]
    end

    subgraph runtime [运行态缓存]
        Redis[(Redis)]
    end

    subgraph async [异步通知与事件]
        RMQ[(RabbitMQ)]
    end

    Auth --> PG
    Auth --> Redis
    Auth --> Etcd
    User --> PG
    User --> Etcd
    Edge --> Etcd
    Edge --> Presence
    Edge --> RMQ
    Gateway --> Etcd
    Presence --> Redis
    Presence --> Etcd
    Notification --> Etcd
    Notification --> RMQ
    Notification --> Presence
    Message --> PG
    Message --> RMQ
    Conversation --> PG
    Conversation --> Etcd
    RMQ -->|message events| Notification
    RMQ -->|push notification| Edge
```

| 中间件 | 做什么 | 不做什么 |
|--------|--------|----------|
| **PostgreSQL** | 用户、认证、消息、会话、outbox 等业务事实源 | 在线态、服务注册、长连接推送 |
| **Redis** | OAuth state、JWT 黑名单、Presence 在线态索引、Notification 去重/限流等短 TTL 运行态 | 服务注册、可靠消息队列、长期业务数据 |
| **etcd** | 服务注册、运行时配置、secrets 分发 | 业务数据、在线态、消息队列 |
| **RabbitMQ** | 领域事件通知、Edge 在线推送通知 | 业务事实源、服务发现、WebSocket 连接管理 |

### 1.3 禁止事项

- Gateway 实例注册只写 etcd，禁止写 Redis `gw:registry`、`gw:alive` 或同类服务发现 key。
- 客户端下行推送只走 RabbitMQ 到 Edge，禁止使用 Redis Pub/Sub 承载下行推送。
- 业务持久化数据只写 PostgreSQL，禁止在 etcd 或 Redis 中长期保存业务事实。
- 消息落库后禁止直接依赖“同步 publish 成功”作为唯一投递路径；必须使用 outbox 或等价可靠发布机制。
- secret 禁止写入日志、panic、metrics label、URL query 或前端响应。

---

## 2. 服务模块与接口边界

### 2.1 服务依赖矩阵

| 服务 | PostgreSQL | Redis | etcd | RabbitMQ |
|------|:----------:|:-----:|:----:|:--------:|
| Auth | 用户、OAuth 绑定、refresh token | OAuth state、JWT blacklist | 注册、配置、secrets | 可选发布认证事件 |
| User | 用户资料 | 可选热点缓存 | 注册、配置 | 可选消费领域事件 |
| Edge | 不访问 | 不访问 | 注册、配置、Watch Gateway | 消费 Edge push |
| Gateway | 不访问 | 不访问 | 注册、配置、secrets | 不访问 |
| Conversation | 会话、成员、权限、会话设置 | 可选热点缓存 | 注册、配置 | 发布会话事件 |
| Message | 消息、会话序号、outbox | 不访问 | 注册、配置 | 发布消息事件 |
| Presence | 不访问 | 在线态、会话路由、订阅状态 | 注册、配置 | 可选发布在线事件 |
| Notification | 设备 token、通知设置、投递记录 | 去重、限流、短 TTL 状态 | 注册、配置、secrets | 消费消息/会话事件；发布 Edge push |

### 2.2 共享基础设施包

基础设施接入代码必须集中在 `pkg/`，业务服务通过接口依赖，不直接散落中间件 SDK 调用。

```text
pkg/
├── config/
│   ├── provider.go        # etcd + local dev fallback
│   └── secret.go          # secret loading and redaction
├── etcd/
│   ├── registry/          # service register, watch, drain
│   └── client.go
├── presence/
│   ├── client.go          # Presence service client
│   └── types.go
├── notification/
│   ├── publisher.go       # Edge push publisher client
│   └── types.go
├── rabbitmq/
│   ├── publisher.go       # publisher confirm, retry, backpressure
│   ├── consumer.go        # ack, nack, prefetch, shutdown
│   └── topology.go
└── postgres/
    ├── pool.go
    └── outbox.go
```

### 2.3 关键接口契约

```go
// Registry registers service instances and watches service membership.
// Registry 注册服务实例并监听服务成员变化。
type Registry interface {
    Register(ctx context.Context, instance ServiceInstance) (Lease, error)
    Watch(ctx context.Context, service string) (<-chan ServiceEvent, error)
    List(ctx context.Context, service string) ([]ServiceInstance, error)
}

// ConfigProvider loads runtime config and secrets from authoritative sources.
// ConfigProvider 从权威来源加载运行时配置和密钥。
type ConfigProvider interface {
    Load(ctx context.Context, key string, target any) error
    Watch(ctx context.Context, key string) (<-chan ConfigEvent, error)
}

// PresenceClient owns online connection state through Presence service APIs.
// PresenceClient 通过 Presence 服务 API 管理在线连接状态。
type PresenceClient interface {
    UpsertConnection(ctx context.Context, conn ConnectionMeta) (previous *ConnectionRoute, err error)
    RemoveConnection(ctx context.Context, conn ConnectionMeta) (removed bool, err error)
    RefreshConnection(ctx context.Context, edgeID, connID string, ttl time.Duration) error
    RebindGateway(ctx context.Context, sessionID, edgeID, connID, gatewayID string) error
    GetLiveRoutes(ctx context.Context, userID string) ([]ConnectionRoute, error)
    CleanupEdge(ctx context.Context, edgeID string, batchSize int) error
}

// EventPublisher publishes events with confirms and bounded retries.
// EventPublisher 使用确认机制和有界重试发布事件。
type EventPublisher interface {
    Publish(ctx context.Context, msg Message) error
}
```

### 2.4 核心数据流

**连接接入**

```mermaid
sequenceDiagram
    participant C as Client
    participant E as Edge
    participant ET as etcd
    participant G as Gateway
    participant P as Presence

    C->>E: WSS /ws with JWT
    E->>ET: Read in-memory gateway view from Watch
    E->>G: internal attach(session_id, auth_context)
    G->>ET: Keep service lease and load config
    E->>P: UpsertConnection(edge_id, gateway_id, session_id)
    E-->>C: 101 Switching Protocols
```

**消息写入与下行通知**

```mermaid
sequenceDiagram
    participant C as Client
    participant M as Message
    participant PG as PostgreSQL
    participant D as OutboxDispatcher
    participant RMQ as RabbitMQ
    participant N as Notification
    participant CV as Conversation
    participant P as Presence
    participant E as Edge

    C->>M: SendMessage
    M->>PG: Tx insert messages + outbox_events
    M-->>C: ACK persisted message_id
    D->>PG: lock unsent outbox events
    D->>RMQ: publish message.created.{conversation_id} with confirm
    RMQ->>N: deliver message event
    N->>CV: Resolve recipients and notification policy
    N->>P: GetLiveRoutes(user_id...)
    N->>RMQ: publish push.edge.{edge_id} with confirm
    RMQ->>E: deliver notification
    E->>E: validate local conn_id
    E-->>C: WebSocket message frame
    E->>RMQ: ack
```

---

## 3. 配置、环境变量与 secrets

### 3.1 配置分层

生产环境只有 etcd 端点和环境名允许通过环境变量启动，其余配置必须从 etcd 读取。

| 层级 | 用途 | 允许项 |
|------|------|--------|
| Bootstrap env | 服务启动前定位配置中心 | `ETCD_ENDPOINTS`、`BEEHIVE_ENV` |
| Runtime config | 可配置业务参数和中间件连接信息 | `/beehive-im/{env}/config/...` |
| Runtime secrets | JWT secret、OAuth secret、数据库密码等敏感信息 | `/beehive-im/{env}/secrets/...` |
| Dev override env | 本地开发未启动 etcd 或未 seed 配置时的回退 | `DB_DSN`、`REDIS_ADDR`、`RABBITMQ_URL`、`PRESENCE_GRPC_TARGET` 等 |

本地开发允许环境变量覆盖；生产、预发环境禁止依赖 dev override env。

### 3.2 Bootstrap 环境变量

| 环境变量 | 必填 | 示例 | 说明 |
|----------|------|------|------|
| `ETCD_ENDPOINTS` | 是 | `127.0.0.1:2379` | 逗号分隔 etcd client endpoints |
| `BEEHIVE_ENV` | 是 | `dev` / `staging` / `prod` | 参与 key 前缀和 Redis key 前缀 |

### 3.3 etcd key 规范

统一前缀：

```text
/beehive-im/{env}/...
```

| 类别 | Key 模式 | 示例 |
|------|----------|------|
| 服务注册 | `/services/{service_name}/{instance_id}` | `/beehive-im/prod/services/gateway/gw-02` |
| 全局配置 | `/config/global/{key}` | `/beehive-im/prod/config/global/jwt.access_ttl` |
| 服务配置 | `/config/{service_name}/{key}` | `/beehive-im/prod/config/gateway/max_sessions` |
| 全局密钥 | `/secrets/global/{key}` | `/beehive-im/prod/secrets/global/jwt.secret` |
| 服务密钥 | `/secrets/{service_name}/{key}` | `/beehive-im/prod/secrets/auth/github.client_secret` |

### 3.4 初始配置 seed

| Key | 默认值 | 热更新 | 说明 |
|-----|--------|--------|------|
| `config/global/jwt.access_ttl` | `15m` | 是 | 只影响新签发 token |
| `config/global/jwt.refresh_ttl` | `720h` | 是 | 只影响新 refresh token |
| `config/auth/db.dsn` | 部署写入 | 否 | 数据库连接池重建需滚动重启 |
| `config/auth/oauth.state_ttl` | `600s` | 是 | OAuth state Redis TTL |
| `config/edge/public_ws_url` | `wss://im.example.com/ws` | 否 | 公网 WebSocket 入口 |
| `config/edge/ws.ping_interval` | `30s` | 是 | 不得大于 read timeout 的 1/2 |
| `config/edge/ws.read_timeout` | `60s` | 是 | 客户端连接读超时 |
| `config/edge/ws.ticket_ttl` | `30s` | 是 | WebSocket 一次性 ticket TTL |
| `config/edge/max_conn` | `50000` | 是 | 单 Edge 最大客户端连接数 |
| `config/edge/gateway.resume_timeout` | `5s` | 是 | Gateway rebind 恢复窗口 |
| `config/gateway/max_sessions` | `20000` | 是 | 单 Gateway 最大上游会话数 |
| `config/presence/presence_ttl` | `90s` | 是 | `conn:meta` 和 `session:route` TTL |
| `config/presence/heartbeat_batch_size` | `512` | 是 | Presence 心跳续期批处理上限 |
| `config/presence/stream_window` | `1024` | 是 | Presence 订阅流单连接窗口 |
| `config/presence/cleanup.batch_size` | `1000` | 是 | 在线态孤儿索引清理批量 |
| `config/message/outbox.batch_size` | `200` | 是 | 单批 outbox dispatch 数量 |
| `config/message/sync.batch_size` | `200` | 是 | 单次消息同步返回上限 |
| `config/message/max_content_bytes` | `8192` | 是 | 单条文本消息内容上限 |
| `config/message/send.rate_limit_per_min` | `120` | 是 | 单用户发送限流 |
| `config/notification/online_push_ttl` | `60s` | 是 | Edge 在线推送通知 TTL |
| `config/notification/offline.enabled` | `true` | 是 | 是否启用离线厂商推送 |
| `config/notification/provider.timeout` | `2s` | 是 | 可选离线 provider 单次调用超时 |
| `config/notification/dedupe_ttl` | `24h` | 是 | 通知事件去重窗口 |
| `config/rabbitmq/url` | 部署写入 | 否 | 连接重建需滚动重启 |
| `config/redis/addr` | 部署写入 | 否 | 连接重建需滚动重启 |
| `secrets/global/jwt.secret` | 部署写入 | 否 | v1 使用 HS256，需滚动重启 |
| `secrets/auth/github.client_id` | 部署写入 | 否 | OAuth App ID |
| `secrets/auth/github.client_secret` | 部署写入 | 否 | OAuth App Secret |

### 3.5 Secret 轮换策略

v1 使用 HS256 单密钥，`jwt.secret` 不支持热更新。轮换必须按以下流程执行：

1. 写入新 secret 到 etcd。
2. 按服务滚动重启 Auth、Edge、Gateway、Message。
3. 保持旧 access token TTL 较短，等待旧 token 自然过期。
4. 若需要紧急失效，启用 Redis `jwt:bl:{jti}` 黑名单或强制客户端重新登录。

v2 建议升级为 `kid + keyring` 或 RS256/JWKS，支持新旧 key 并存到旧 token 过期。

---

## 4. PostgreSQL

### 4.1 职责

PostgreSQL 是唯一业务事实源。

| 域 | 表 / 说明 |
|----|-----------|
| 用户 | `users`、`user_profiles`，见 [`sql/migrations/users/001_user.sql`](../../sql/migrations/users/001_user.sql) |
| 认证 | `user_oauth_identities`、`refresh_tokens`，见 [`sql/migrations/auth/002_auth.sql`](../../sql/migrations/auth/002_auth.sql) |
| 会话 | `conversations`、`conversation_members`、`conversation_settings` 等 |
| 消息 | `messages`、`message_receipts` 等 |
| 通知 | `notification_devices`、`notification_preferences`、`notification_deliveries` 等 |
| 可靠发布 | `outbox_events`，保存待发布领域事件和通知请求 |

### 4.2 迁移

- CLI：[`sql/migrate/main.go`](../../sql/migrate/main.go)
- 目录：[`sql/migrations/`](../../sql/migrations/)（按域分子目录）
- 入口脚本：`sql/migrate.ps1` / `sql/migrate.sh`
- 迁移必须向前兼容数据库已有数据，禁止在无备份情况下执行破坏性 DDL。
- 生产迁移必须先在 staging 跑通，并记录执行耗时、锁表风险和回滚方案。

### 4.3 Outbox 可靠发布

消息写入和事件写入必须在同一数据库事务内完成。

```mermaid
flowchart LR
    App[Message Service] -->|Tx| PG[(PostgreSQL)]
    PG -->|messages| M[(messages)]
    PG -->|outbox_events| O[(outbox_events)]
    D[Outbox Dispatcher] --> O
    D -->|publisher confirm| RMQ[(RabbitMQ)]
```

`outbox_events` 最小字段建议：

| 字段 | 说明 |
|------|------|
| `id` | 事件 ID，UUID 或雪花 ID |
| `aggregate_type` | `message`、`conversation` 等 |
| `aggregate_id` | 聚合 ID，例如 `message_id` |
| `event_type` | `message.created`、`push.requested` |
| `routing_key` | RabbitMQ routing key |
| `payload` | JSONB payload |
| `status` | `pending`、`publishing`、`published`、`failed` |
| `attempts` | 发布次数 |
| `next_retry_at` | 下次重试时间 |
| `created_at` / `published_at` | 生命周期时间 |

Dispatcher 要求：

- 使用 `SELECT ... FOR UPDATE SKIP LOCKED` 多实例并发拉取。
- 使用 RabbitMQ publisher confirm；没有 confirm 不得标记 `published`。
- 发布失败使用指数退避和最大重试次数。
- 事件 payload 必须包含 `event_id`、`message_id`、`conversation_id`、`seq`，消费者按 `event_id` 或业务唯一键幂等。

### 4.4 连接池与事务

| 项 | 要求 |
|----|------|
| 连接池 | 每服务独立池，`max_conns` 按实例数和数据库上限计算 |
| 超时 | 每个 SQL 操作必须带 `context.Context` deadline |
| 事务 | 业务写入、outbox 写入必须同事务 |
| 隔离级别 | 默认 `READ COMMITTED`；涉及账户唯一性和会话序号时使用唯一索引保证并发正确 |
| 慢查询 | 超过 200ms 记录 warn 日志，超过 1s 进入告警指标 |

### 4.5 部署与安全

| 环境 | 拓扑 |
|------|------|
| 本地 | 单实例 PostgreSQL 16 |
| 生产 | 云 RDS 或主从高可用；开启自动备份、PITR、监控和 TLS |

生产要求：

- 数据库账号按服务拆分权限，Auth 不应拥有 Message 表写权限。
- 密码只存 etcd secrets 或专用 secret manager，禁止提交到仓库。
- 定期备份并演练恢复，恢复演练至少覆盖用户表、消息表和 outbox。

---

## 5. Redis

### 5.1 职责

Redis 只保存短生命周期或可从事实源恢复的运行态数据。

物理 key 必须带环境前缀：

```text
beehive:{env}:{logical_key}
```

下文表格展示 logical key，代码实现必须统一拼接前缀，避免多环境共用 Redis 时污染数据。

| 数据 | Logical key | 类型 | TTL | 服务 |
|------|-------------|------|-----|------|
| OAuth 授权 state | `oauth:state:{state}` | String / JSON | `oauth.state_ttl` | Auth |
| JWT 黑名单 | `jwt:bl:{jti}` | String | 至 access token 过期 | Auth / Edge / Gateway |
| 用户设备在线映射 | `conn:user:{user_id}` | Hash | 无 | Presence |
| Edge 连接集合 | `conn:edge:{edge_id}` | Set | 无 | Presence |
| 连接元数据 | `conn:meta:{conn_id}` | Hash | `presence_ttl` | Presence |
| 会话路由 | `session:route:{session_id}` | Hash | `presence_ttl` | Presence |
| 在线事件去重 | `notify:dedupe:{event_id}` | String | `notification.dedupe_ttl` | Notification |
| 用户通知限流 | `notify:rate:{user_id}` | Counter | 短 TTL | Notification |

### 5.2 OAuth state

Value 使用 JSON：

```json
{
  "redirect_uri": "https://app.example.com/oauth/github/callback",
  "scope": "read:user user:email",
  "intent": "login",
  "user_id": null,
  "created_at": "2026-06-19T08:00:00Z"
}
```

要求：

- `state` 使用加密安全随机数，至少 32 字节熵。
- 消费必须是原子 `GETDEL` 或 Lua `GET` + `DEL`。
- `redirect_uri` 必须和服务端白名单匹配，禁止直接信任客户端传入值。

### 5.3 在线态不变量

在线态由 Presence 服务统一管理，Redis 只是 Presence 的短 TTL 运行态存储。Edge 是客户端 WebSocket 连接拥有者，但不直接写 Redis；Notification、Message 和其他服务也不直接读取 Redis 在线态。

```text
conn:user:{user_id}              HASH device_id -> edge_id:conn_id:session_id
conn:edge:{edge_id}              SET conn_id
conn:meta:{conn_id}              HASH user_id, device_id, edge_id, gateway_id, session_id, connected_at, last_seen_at
session:route:{session_id}       HASH edge_id, conn_id, gateway_id, user_id, device_id
```

必须满足的不变量：

| 不变量 | 说明 |
|--------|------|
| `conn:meta` 是活跃连接判定依据 | `conn:user` 和 `conn:edge` 只是索引，读取时必须校验 meta 存在 |
| 同设备后登录覆盖先登录 | 同一 `user_id + device_id` 只保留最新 `edge_id:conn_id:session_id` |
| 删除必须 compare-and-delete | 断开时只有当前 value 等于 `edge_id:conn_id:session_id` 才允许删除 `conn:user` 字段 |
| Gateway 崩溃不改变客户端在线事实 | Edge 保持客户端连接，并通过 Presence 更新 `session:route.gateway_id` |
| Edge 崩溃允许索引残留 | 残留索引通过 `conn:meta` TTL、Presence cleanup 和定期修复任务清理 |
| 所有写入必须原子 | 连接建立、心跳续期、Gateway rebind、断开清理必须由 Presence 使用 Lua 或 Redis 事务完成 |

### 5.4 连接建立

连接建立时由 Edge 调用 Presence `UpsertConnection`，Presence 使用 Lua 原子脚本完成：

1. 读取旧的 `conn:user:{user_id}` 中 `device_id` 对应 value。
2. 写入新的 `edge_id:conn_id:session_id`。
3. 将 `conn_id` 加入 `conn:edge:{edge_id}`。
4. 写入 `conn:meta:{conn_id}` 和 `session:route:{session_id}` 并设置 TTL。
5. 返回旧连接路由，由对应 Edge 关闭旧连接或等待 TTL 清理。

伪代码：

```text
old = HGET conn:user:{user_id} {device_id}
HSET conn:user:{user_id} {device_id} {edge_id}:{conn_id}:{session_id}
SADD conn:edge:{edge_id} {conn_id}
HSET conn:meta:{conn_id} user_id ... device_id ... edge_id ... gateway_id ... session_id ... connected_at ... last_seen_at ...
HSET session:route:{session_id} edge_id ... conn_id ... gateway_id ... user_id ... device_id ...
EXPIRE conn:meta:{conn_id} {presence_ttl}
EXPIRE session:route:{session_id} {presence_ttl}
return old
```

### 5.5 心跳续期

Edge 收到客户端 ping 或业务帧时调用 Presence `RefreshConnection`。Presence 校验连接仍属于当前 Edge 后刷新 `conn:meta:{conn_id}` 和 `session:route:{session_id}`：

```text
if HGET conn:meta:{conn_id} edge_id == {edge_id}
then HSET last_seen_at now; EXPIRE conn:meta:{conn_id} {presence_ttl}; EXPIRE session:route:{session_id} {presence_ttl}
else return stale_connection
```

心跳续期失败时 Presence 返回 `stale_connection`，Edge 必须关闭本地 WebSocket，避免向已被新连接覆盖的旧连接继续下发消息。

### 5.6 Gateway rebind

Gateway 故障恢复时，Edge 调用 Presence `RebindGateway`。Presence 使用 compare-and-set Lua 更新当前上游：

```text
if HGET session:route:{session_id} edge_id == {edge_id}
and HGET session:route:{session_id} conn_id == {conn_id}
then HSET session:route:{session_id} gateway_id {new_gateway_id}
     HSET conn:meta:{conn_id} gateway_id {new_gateway_id}
     EXPIRE session:route:{session_id} {presence_ttl}
     EXPIRE conn:meta:{conn_id} {presence_ttl}
else return stale_session
```

### 5.7 连接断开

断开时 Edge 调用 Presence `RemoveConnection`，Presence 必须 compare-and-delete：

```text
current = HGET conn:user:{user_id} {device_id}
if current == {edge_id}:{conn_id}:{session_id}
then HDEL conn:user:{user_id} {device_id}
SREM conn:edge:{edge_id} {conn_id}
DEL conn:meta:{conn_id}
DEL session:route:{session_id}
```

禁止无条件 `HDEL conn:user:{user_id} {device_id}`，否则用户同设备快速重连时会误删新连接。

### 5.8 在线路由查询

Notification 或其他内部服务必须通过 Presence `GetLiveRoutes` 查询在线路由：

1. Presence 对 `conn:user:{user_id}` 执行 `HGETALL`，获取候选连接。
2. Presence 批量校验 `conn:meta:{conn_id}` 和 `session:route:{session_id}` 是否存在，且 `edge_id`、`user_id`、`device_id` 与索引一致。
3. Presence 过滤不存在或不一致的连接。
4. Presence 对残留索引做异步清理，不阻塞主查询路径。
5. Notification 按 `edge_id` 聚合后发布 RabbitMQ push notification。

### 5.9 清理任务

| 触发 | 行为 |
|------|------|
| Edge 优雅下线 | 停止接新连接，等待本地连接归零或通知客户端重连，并调用 Presence `CleanupEdge` |
| Edge 崩溃恢复后 | Presence 扫描 `conn:edge:{edge_id}`，删除没有 meta 的残留 conn |
| Gateway 崩溃 | Edge 执行 rebind/resume，并调用 Presence 更新 `session:route.gateway_id` |
| Presence 查询发现残留 | 异步 compare-and-delete 对应索引 |
| 定时修复任务 | Presence 分批扫描 `conn:edge:*`，校验 meta TTL，清理孤儿索引 |

### 5.10 部署与安全

| 环境 | 拓扑 |
|------|------|
| 本地 | 单实例 Redis 8，AOF 开启 |
| 生产 | Redis Cluster 或 Sentinel；开启 TLS、ACL、内存告警和慢日志 |

生产要求：

- `maxmemory-policy` 不得使用会随机淘汰在线态的策略；推荐为在线态 Redis 使用独立实例或独立 DB。
- OAuth state 和 JWT blacklist 必须设置 TTL，禁止无 TTL 写入。
- 在线态允许短暂不一致，业务补偿依赖消息事实源和客户端同步。

---

## 6. etcd

### 6.1 职责

etcd 用于服务注册、配置分发和 secrets 分发。它不是业务数据库，也不是在线态存储。

### 6.2 服务注册 value

服务注册 key：

```text
/beehive-im/{env}/services/{service_name}/{instance_id}
```

Gateway value 示例：

```json
{
  "schema_version": 1,
  "instance_id": "gw-02",
  "service": "gateway",
  "host": "10.0.1.12",
  "upstream_addr": "10.0.1.12:9100",
  "status": "online",
  "session_count": 128,
  "max_sessions": 20000,
  "region": "cn-east",
  "zone": "cn-east-a",
  "version": "v1.1.0",
  "started_at": "2026-06-20T08:00:00Z",
  "updated_at": "2026-06-20T08:01:00Z"
}
```

Edge value 示例：

```json
{
  "schema_version": 1,
  "instance_id": "edge-01",
  "service": "edge",
  "host": "10.0.1.20",
  "public_ws_url": "wss://im.example.com/ws",
  "status": "online",
  "conn_count": 2048,
  "max_conn": 50000,
  "region": "cn-east",
  "zone": "cn-east-a",
  "version": "v1.1.0",
  "started_at": "2026-06-20T08:00:00Z",
  "updated_at": "2026-06-20T08:01:00Z"
}
```

### 6.3 租约与 Watch

| 参数 | 默认值 | 要求 |
|------|--------|------|
| Lease TTL | `30s` | 大于 KeepAlive 间隔 3 倍 |
| KeepAlive 间隔 | `10s` | 失败后立即重试，连续失败进入降级 |
| Watch 启动 | `Get` 全量 + `Watch` 增量 | 避免冷启动漏事件 |
| Watch 异常 | 从 last revision 恢复 | revision compacted 时重新全量加载 |

```mermaid
sequenceDiagram
    participant S as ServiceInstance
    participant E as etcd
    participant C as Edge

    S->>E: Grant Lease TTL=30s
    S->>E: Put service key with Lease
    loop every 10s
        S->>E: KeepAlive
        S->>E: Put updated load with same Lease
    end
    C->>E: Get services/gateway prefix
    C->>E: Watch from revision + 1
    Note over S,E: 进程崩溃后 Lease 过期，key 自动删除
```

### 6.4 Edge 与 Gateway 优雅下线

Gateway 下线：

1. Gateway 将注册 value 更新为 `status=draining`。
2. Edge Watch 到 `draining` 后停止分配新会话。
3. Edge 将已绑定该 Gateway 的 session rebind 到其他健康 Gateway，或等待自然结束。
4. Gateway 到达 `drain_timeout` 后关闭剩余上游会话。
5. Revoke lease，删除 etcd 注册 key。

Edge 下线：

1. Edge 将注册 value 更新为 `status=draining`。
2. 前置 LB 停止向该 Edge 分配新连接。
3. Edge 通知客户端重连或等待连接自然结束。
4. Edge 调用 Presence 清理本实例在线态。
5. Revoke lease，删除 etcd 注册 key。

### 6.5 配置热更新

| 配置 | 热更新 | 说明 |
|------|--------|------|
| `edge.max_conn` | 是 | Edge 入口限流生效 |
| `edge.ws.ping_interval` | 是 | 只影响新连接或下一轮心跳调度 |
| `edge.ws.read_timeout` | 是 | 只影响新连接 |
| `edge.gateway.resume_timeout` | 是 | 下一次 Gateway rebind 生效 |
| `gateway.max_sessions` | 是 | Edge 过滤和 Gateway 限流生效 |
| `presence.presence_ttl` | 是 | 下一次连接写入或心跳续期生效 |
| `presence.cleanup.batch_size` | 是 | 下一轮清理任务生效 |
| `notification.online_push_ttl` | 是 | 下一次在线通知发布生效 |
| `notification.offline.enabled` | 是 | 下一次通知编排生效 |
| `jwt.access_ttl` | 是 | 只影响新签发 token |
| `jwt.secret` | 否 | v1 需要滚动重启 |
| 数据库、Redis、RabbitMQ 地址 | 否 | 连接重建需要滚动重启 |
| 监听端口 | 否 | 需要重启 |

### 6.6 安全

生产环境要求：

- etcd client 和 peer 通信必须启用 TLS。
- 启用 RBAC，不同服务账号只允许读写自身所需 key。
- `secrets/*` 只允许被需要的服务读取。
- 开启审计日志，记录 secret 写入、删除、权限变更。
- 定期 compact 和 defrag，避免磁盘无限增长。

---

## 7. RabbitMQ

### 7.1 职责和可靠性边界

RabbitMQ 承担事件通知和 Edge 在线推送通知。Message 持久化后的真实消息内容以 PostgreSQL 为准；RabbitMQ push 失败时，客户端必须能通过消息同步接口补齐缺失消息。

换言之：

- RabbitMQ 保证服务间通知尽量可靠。
- PostgreSQL 保证消息事实不丢。
- WebSocket 下行不作为唯一可靠投递证明。

### 7.2 Exchange 规划

| Exchange | 类型 | 持久化 | 说明 |
|----------|------|--------|------|
| `beehive.im.events` | topic | durable | 领域事件 |
| `beehive.im.push` | topic | durable | Notification 到 Edge 的在线推送通知 |
| `beehive.im.retry` | topic | durable | 延迟重试 |
| `beehive.im.dlq` | topic | durable | 死信 |

### 7.3 Routing key

| Routing key | 用途 |
|-------------|------|
| `message.created.{conversation_id}` | 消息已持久化事件 |
| `conversation.updated.{conversation_id}` | 会话元数据变更 |
| `user.online.{user_id}` | 用户上线事件，可选 |
| `push.edge.{edge_id}` | 定向推送通知到 Edge 实例 |

### 7.4 Edge push 队列

v1 将 Edge push 定义为 Notification 到 Edge 的在线通知，不作为消息事实源。

| 项 | 要求 |
|----|------|
| 队列名 | `edge.push.{edge_id}` |
| 队列类型 | 非 durable、exclusive、auto-delete |
| 绑定 | `beehive.im.push` + `push.edge.{edge_id}` |
| 消息 TTL | 建议 30s 到 120s，超过在线窗口直接丢弃或进入 DLQ |
| Ack | Edge 写入本地 WebSocket buffer 成功后 ack |
| Prefetch | 按 Edge 连接数和写 buffer 容量配置，必须有限 |
| 幂等 | payload 必须携带 `event_id`、`notification_id`、`message_id`、`conversation_id`、`seq` |

Edge 崩溃时该实例队列删除，未投递通知可能丢失；这是可接受的，因为客户端重连后必须通过消息同步接口按 `conversation_id + seq` 拉取缺失消息。

如果未来需要“服务端推送通知本身也强可靠”，必须升级为稳定 shard queue 或 user-device queue，并补齐 ACK 回执、重试、去重和存储成本评估。

### 7.5 领域事件队列

领域事件消费者使用独立 durable queue，生产环境推荐 Quorum Queue。

| 消费者 | Queue | 绑定 | 用途 |
|--------|-------|------|------|
| Notification | `notification.message.events` | `message.created.#`、`conversation.updated.#` | 在线/离线通知编排 |
| Audit | `audit.message.events` | `message.created.#` | 审计日志 |
| Search | `search.message.events` | `message.created.#` | 全文索引 |
| Analytics | `analytics.events` | `message.created.#`、`conversation.updated.#` | 统计分析 |

不同消费者必须使用不同 queue，避免广播事件被竞争消费。

Notification 消费 `message.created.#`、`conversation.updated.#` 等领域事件后，必须先完成事件去重、通知偏好判断、在线路由查询和离线策略判断，再向 `beehive.im.push` 发布 `push.edge.{edge_id}`。Notification 发布 Edge push 也必须启用 publisher confirm；发布失败不得影响 Message 已持久化事实，只能进入重试或 DLQ。

### 7.6 发布要求

- 所有发布必须设置 `message_id`、`correlation_id`、`content_type=application/json`。
- Outbox dispatcher 必须启用 publisher confirm。
- 发布失败不得丢弃，必须回写 outbox 状态并重试。
- RabbitMQ 连接断开时 dispatcher 进入退避重连，禁止无限制 goroutine 重试。
- 所有消费者必须设置有限 `prefetch`，避免瞬时推送压垮 Edge。

### 7.7 部署与安全

| 环境 | 拓扑 |
|------|------|
| 本地 | 单节点 + management UI |
| 生产 | 3 节点 RabbitMQ；领域事件使用 Quorum Queue；启用 TLS、用户权限隔离和磁盘告警 |

生产要求：

- Edge 只允许声明和消费自己的 `edge.push.{edge_id}`。
- Message / Outbox Dispatcher 和 Notification 只允许 publish 指定 exchange。
- DLQ 必须有告警和人工处理流程。
- 队列堆积、confirm 延迟、consumer ack 延迟必须进入监控。

---

## 8. 本地开发

[`docker/Infrastructure/docker-compose.yaml`](../../docker/Infrastructure/docker-compose.yaml) 提供本地依赖：

| 服务 | 镜像 | 端口 |
|------|------|------|
| postgresql | `postgres:16` | `5432` |
| redis | `redis:8-alpine` | `6379` |
| etcd | `quay.io/coreos/etcd:v3.5.16` | `2379` |
| rabbitmq | `rabbitmq:3-management-alpine` | `5672`、`15672` |

推荐本地开发环境变量：

```bash
ETCD_ENDPOINTS=127.0.0.1:2379
BEEHIVE_ENV=dev
DB_DSN=postgres://Beehive-IM:Beehive-IM@127.0.0.1:5432/Beehive-IM?sslmode=disable
REDIS_ADDR=127.0.0.1:6379
RABBITMQ_URL=amqp://guest:guest@127.0.0.1:5672/
PRESENCE_GRPC_TARGET=127.0.0.1:9200
```

docker compose、迁移脚本和本地环境变量必须使用同一组数据库名、用户名和密码；推荐统一为 `Beehive-IM`，避免本地迁移连接到错误数据库。

### 8.1 当前本地启动顺序

当前代码已接入第一版中间件基座，本地验证顺序：

```powershell
docker compose -f docker\Infrastructure\docker-compose.yaml up -d
.\sql\migrate.ps1
go run .\services\presence -f .\services\presence\etc\beehiveim.presence.yaml
go run .\services\gateway -f .\services\gateway\etc\beehiveim.gateway.yaml
go run .\services\user -f .\services\user\etc\beehiveim.user.yaml
$env:RABBITMQ_URL='amqp://guest:guest@127.0.0.1:5672/'
go run .\services\edge -f .\services\edge\etc\beehiveim.edge.yaml
```

实现状态：

| 中间件 | 当前接入点 |
|--------|------------|
| PostgreSQL | `pkg/postgres` 连接池；User `GetUser` 读取真实用户表 |
| Redis | `pkg/redis` 客户端；Presence 独占在线态 key 写入 |
| etcd | `pkg/etcd` 服务注册/发现；Gateway 注册，Edge watch Gateway |
| RabbitMQ | `pkg/rabbitmq` 拓扑/publish 基础封装；Edge 消费 `edge.push.{edge_id}` |

为避免与 go-zero 内置 `Etcd` / `Redis` 配置字段冲突，业务中间件配置使用 `Registry`、`RedisStore` 等显式名称。

---

## 9. 可观测性与告警

### 9.1 日志

- 日志内容统一使用英文。
- 必须包含 `trace_id`、`service`、`instance_id`、`operation`、`duration_ms`、`error_code`。
- 禁止记录 access token、refresh token、OAuth code、OAuth state、JWT secret、数据库密码。
- Redis key 中包含用户 ID 时，生产日志应只记录 hash 或尾号。

### 9.2 Metrics

| 组件 | 指标 |
|------|------|
| PostgreSQL | pool in-use、query latency、slow query count、outbox pending/failed |
| Redis | command latency、used memory、expired keys、presence stale index count |
| etcd | lease keepalive failures、watch reconnects、watch lag、leader changes |
| RabbitMQ | publish confirm latency、queue depth、consumer ack latency、DLQ count |
| Edge | active connections、write buffer usage、push ack/nack、disconnect reason、upstream rebind count |
| Gateway | active sessions、attach latency、resume latency、session reject count |
| Message | send latency、sync latency、sync gap count、idempotent hit count、outbox pending/failed |
| Presence | upsert latency、refresh lag、route lookup latency、stale route count、Redis script errors、cleanup duration |
| Notification | event lag、dedupe hit count、online push success/fail、optional provider latency、rate limited count、DLQ count |

### 9.3 告警建议

| 告警 | 阈值建议 |
|------|----------|
| Outbox pending 堆积 | 连续 5 分钟增长且未下降 |
| RabbitMQ DLQ | 任意新增即告警 |
| Edge 可用实例数 | 小于期望副本数或为 0 |
| Gateway 可用实例数 | 小于期望副本数或为 0，或 resume 失败率持续升高 |
| etcd Watch 重建频繁 | 5 分钟内超过 10 次 |
| Redis stale presence | 残留连接比例超过 1% |
| Presence route lookup latency | P95 超过 30ms 或 P99 超过 100ms |
| Notification event lag | 连续 5 分钟超过在线推送 TTL 的 50% |
| Notification optional provider failures | 启用 provider 时，5 分钟错误率超过 5% |
| PostgreSQL 慢查询 | P95 超过 200ms 或 P99 超过 1s |

---

## 10. 安全要求

| 领域 | 要求 |
|------|------|
| 网络 | 生产环境服务间通信启用 TLS；公网只暴露 Edge/API Gateway 必要入口 |
| 权限 | PostgreSQL、Redis、etcd、RabbitMQ 使用最小权限账号 |
| Secret | 只存 etcd secrets 或专用 secret manager，禁止提交仓库 |
| Token | Access token TTL 短；refresh token 只存 hash；登出撤销 refresh token |
| OAuth | state 一次性消费；redirect URI 白名单；client secret 只在服务端 |
| WebSocket | 握手验签；origin 白名单；连接限流；消息大小限制 |
| 数据 | 用户隐私字段按业务要求脱敏；备份加密 |

---

## 11. 容量与性能策略

### 11.1 Edge

- 单实例最大连接数由 `config/edge/max_conn` 控制。
- Edge 路由 Gateway 时过滤 `status != online`、`session_count >= max_sessions`、region 不匹配的 Gateway。
- Edge 需要本地客户端写队列和上游 Gateway 写队列，队列满时应断开慢消费者或降级推送。
- 下行写入必须异步非阻塞，禁止在 RabbitMQ consumer goroutine 中直接阻塞写客户端连接。

### 11.2 Gateway

- 单实例最大上游会话数由 `config/gateway/max_sessions` 控制。
- Gateway attach/resume 必须有 deadline 和并发限流。
- Gateway 内存会话状态必须可重建，禁止作为消息事实源。
- Gateway draining 时停止接新会话，并配合 Edge rebind 现有 session。

### 11.3 Redis

- Presence 在线态写入使用 Lua，减少多 RTT 和竞态。
- Presence 查询在线态时使用 pipeline 或 Lua 批量校验 meta。
- 定时清理任务必须限速，避免大 key 扫描影响线上请求。

### 11.4 Presence

- Presence gRPC API 必须异步非阻塞，连接写入、心跳续期和批量查询都要有 deadline。
- 心跳续期支持批量接口或内部合并，避免每个 ping 触发独立 Redis RTT。
- `GetLiveRoutes` 必须按用户和连接数做限流，避免大群通知放大 Redis 压力。
- 清理任务使用游标和小批量，禁止一次性扫描全量 key。

### 11.5 Notification

- Notification consumer 必须设置有限 prefetch，并用有界 worker pool 编排在线/离线通知。
- 在线 push 按 `edge_id` 聚合，减少 RabbitMQ publish 次数和 Edge 写放大。
- 可选离线 provider 调用必须有超时、熔断、限流和失败降级，禁止阻塞在线 push。
- 幂等去重以 `event_id + recipient_id + device_id` 为核心，避免 MQ 重投导致重复通知。

### 11.6 RabbitMQ

- Publisher 使用连接复用和 channel 池，禁止每条消息新建连接。
- Consumer 设置有限 prefetch，并按 Edge 写 buffer 能力动态调整。
- Outbox dispatcher 使用有界 worker pool。

### 11.7 PostgreSQL

- 消息表按会话、时间或业务增长情况规划分区。
- 高频查询必须有覆盖索引和分页游标。
- Outbox 表按 `status + next_retry_at` 建索引，并定期归档已发布事件。

---

## 12. 风险评估与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| DB commit 成功但 MQ publish 失败 | 消息已存但无推送通知 | Outbox + publisher confirm + 重试 |
| Presence 不可用 | 新连接注册、心跳续期和在线推送路由受影响 | 多副本、gRPC deadline、Redis 连接池隔离、读写降级和告警 |
| Edge 崩溃导致 Redis 索引残留 | Notification 发布到失效 Edge | Presence 查询时校验 `conn:meta` 和 `session:route`，定时清理 |
| Gateway 崩溃导致上游会话丢失 | 客户端连接仍在但业务上游中断 | Edge rebind/resume，失败后关闭连接并依赖消息同步补偿 |
| 同设备快速重连 | 旧连接断开误删新连接 | compare-and-delete Lua |
| etcd Watch 丢事件 | Edge 绑定到失效 Gateway | Get 全量 + Watch revision；compacted 后重建视图 |
| RabbitMQ 队列堆积 | 推送延迟升高 | Prefetch、背压、DLQ、告警、客户端同步补偿 |
| Notification 堆积或重复消费 | 在线通知延迟、离线通知重复 | 独立 durable queue、幂等去重、限流、DLQ 和重放工具 |
| 可选离线 provider 故障 | 离线通知缺失或延迟，不影响 Web 消息同步 | Provider 超时、熔断、备用通道、失败记录和补偿任务 |
| JWT secret 泄露 | Token 可伪造 | secret 最小权限、滚动重启轮换、短 TTL、黑名单 |
| Redis 内存淘汰 | 在线态丢失 | 独立 Redis、内存告警、合理 eviction policy |

---

## 13. 发布与运维流程

### 13.1 新环境初始化

1. 部署 PostgreSQL、Redis、etcd、RabbitMQ。
2. 配置 TLS、账号、权限和网络策略。
3. 写入 etcd config 和 secrets。
4. 执行数据库迁移。
5. 声明 RabbitMQ exchanges、DLX、领域事件队列。
6. 启动 Presence、Notification、Edge、Gateway、Message 等服务，确认注册到 etcd。
7. 执行健康检查、在线态写入、消息通知和端到端连通性测试。

### 13.2 Edge 与 Gateway 扩缩容

- Edge 扩容：新 Edge 启动并注册 etcd，前置 LB 分流新客户端连接。
- Edge 缩容：Edge 进入 `draining`，LB 停止分流新连接，旧连接重连或自然结束。
- Gateway 扩容：新 Gateway 启动并注册 etcd，Edge Watch 自动感知并可用于 attach/resume。
- Gateway 缩容：Gateway 进入 `draining`，Edge 停止分配新会话，并逐步 rebind 现有 session。
- 异常缩容：Lease 过期后 Edge 移除 Gateway 视图，受影响 session 执行 rebind/resume。

### 13.3 RabbitMQ 故障恢复

- Dispatcher 检测 publish confirm 超时后回写 outbox 重试。
- Edge consumer 断线后退避重连，重连后重新声明实例队列。
- DLQ 新增必须告警，人工或自动修复后可重新投递。

### 13.4 数据恢复

- PostgreSQL 恢复后必须校验 outbox 状态，避免重复发布不可幂等事件。
- Redis 在线态可清空重建，客户端重连后恢复；Gateway 内存会话由 Edge rebind/resume 重建。
- Presence 恢复后必须校验 Redis 连接池、Lua 脚本加载和清理任务游标，避免恢复后批量误删。
- Notification 恢复后从 RabbitMQ durable queue 继续消费，必须依赖幂等 key 避免重复通知。
- etcd 恢复后服务重新注册；配置和 secrets 必须从备份恢复。
- RabbitMQ 恢复后 dispatcher 可从 outbox 重新发布未完成事件。

---

## 14. 验收清单

进入实现或上线前必须满足：

- [ ] PostgreSQL 迁移可重复执行，staging 已验证。
- [ ] Message 写入使用 `messages + outbox_events` 同事务。
- [ ] Message 发送接口支持 `client_msg_id` 幂等，客户端可按 `conversation_id + seq` 补偿同步。
- [ ] Outbox dispatcher 使用 publisher confirm，并有重试、DLQ、指标。
- [ ] Presence 提供 `UpsertConnection`、`RefreshConnection`、`RebindGateway`、`RemoveConnection`、`GetLiveRoutes` API。
- [ ] Presence 在线态写入、Gateway rebind、断开、续期使用 Lua 或事务，断开为 compare-and-delete。
- [ ] Notification 消费 `message.created.#` 后完成幂等、偏好判断、Presence 路由查询和在线/离线分流。
- [ ] Notification 在线 push、去重、限流、重试和 DLQ 均有指标与测试；可选离线 provider 启用时补齐 provider 指标与测试。
- [ ] Gateway 服务注册只写 etcd，不写 Redis 服务发现 key。
- [ ] Edge Watch 使用全量加载 + revision 增量恢复。
- [ ] `jwt.secret` v1 明确为滚动重启生效，不做热更新。
- [ ] WebSocket 鉴权使用一次性 `ws_ticket`，生产禁止 query access token。
- [ ] RabbitMQ Edge push 明确为在线通知，客户端具备消息补偿同步。
- [ ] Edge -> Gateway 上游连接具备 deadline、背压、rebind/resume 和失败兜底重连。
- [ ] 生产环境 TLS、RBAC、最小权限、secret 脱敏已配置。
- [ ] 指标、日志、告警覆盖 PostgreSQL、Redis、etcd、RabbitMQ、Gateway、Edge、Presence、Notification。

---

## 15. 演进路线

| 阶段 | 内容 |
|------|------|
| v1 | 四类中间件本地 compose；Edge 代理长连接；Gateway 内网上游；Presence 在线态；Notification 在线 push；outbox 基线 |
| v1.1 | JWT 黑名单；Gateway rebind/resume；Presence 清理任务；基础观测面板 |
| v2 | etcd 3 节点 + TLS + RBAC；RabbitMQ Quorum Queue；gRPC etcd resolver；Message 同步接口完善；Web Push / 移动 provider 扩展 |
| v3 | RS256 + JWKS；配置灰度；跨 region 路由；RabbitMQ federation 或替代事件总线评估 |
