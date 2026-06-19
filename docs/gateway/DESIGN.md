# Beehive-IM 网关模块设计文档

> 版本：v1.0  
> 适用范围：Edge 边缘路由服务、Gateway 多实例 WebSocket 接入  
> 关联文件：[`docs/auth/DESIGN.md`](../auth/DESIGN.md)、[`docs/infrastructure/infrastructure.md`](../infrastructure/infrastructure.md)、[`proto/auth.proto`](../../proto/auth.proto)

---

## 1. 目标与范围

### 1.1 目标

网关模块负责 IM 客户端的**长连接接入**与**在线态管理**，采用两层架构：

| 组件 | 职责 |
|------|------|
| **Edge** | 无状态边缘路由；Watch etcd Gateway 注册表；按 `user_id` 一致性哈希分配 `ws_url` |
| **Gateway** | 多实例部署；WebSocket 升级、JWT 认证；etcd 租约注册；Redis 在线态与心跳 |

客户端连接路径：**Auth 登录 → Edge 获取路由 → 直连 Gateway WebSocket**。

### 1.2 非目标（v1 不做）

- Edge 代理 WebSocket 流量（统一入口转发）
- 消息业务协议与 Message 服务完整对接（仅预留推送路径）
- 跨机房多活与异地路由
- 应用层端到端加密

### 1.3 已确认架构决策

| 决策 | 选择 |
|------|------|
| 连接模型 | Edge **分配** Gateway 地址，客户端 **直连** Gateway WebSocket |
| 路由策略 | **一致性哈希**（`user_id` 为 key，同一用户优先落到同一 Gateway） |
| 认证位置 | **Gateway** 在 WebSocket 握手阶段校验 JWT；Edge 分配路由时验签以解析 `user_id` |

---

## 2. 总体架构

```mermaid
flowchart TB
    subgraph clients [Clients]
        Client[IM Client]
    end

    subgraph edge_layer [Edge Layer]
        Edge1[Edge Instance]
        Edge2[Edge Instance]
    end

    subgraph gateway_layer [Gateway Layer]
        GW1[Gateway A]
        GW2[Gateway B]
        GW3[Gateway C]
    end

    subgraph backend [Backend]
        Auth[Auth Service]
        Etcd[(etcd)]
        Redis[(Redis)]
        RMQ[(RabbitMQ)]
        Msg[Message Service - future]
    end

    Client -->|"1 Login gRPC/HTTP"| Auth
    Client -->|"2 POST /v1/route + JWT"| Edge1
    Edge1 --> Etcd
    Client -->|"3 WSS /ws + JWT"| GW2
    GW2 --> Etcd
    GW2 --> Redis
    GW2 --> RMQ
    Msg -.->|"4 push"| RMQ
    RMQ -.-> GW2
```

### 2.1 与 Auth 模块的衔接

- 登录、注册、刷新令牌仍走 [`proto/auth.proto`](../../proto/auth.proto) 的 Auth 服务，获取 `access_token`（JWT）
- Edge 与 Gateway 从 etcd `secrets/global/jwt.secret` 读取同一验签密钥，**本地验签**，无需每次握手回调 Auth；v1 密钥变更通过滚动重启生效
- JWT 载荷约定见 [`docs/auth/DESIGN.md`](../auth/DESIGN.md) 第 4.1 节：`sub`（user_id）、`username`、`iat`、`exp`、`jti`

### 2.2 组件职责边界

| 组件 | 做 | 不做 |
|------|----|------|
| Edge | 验签 JWT、一致性哈希选路、返回 `ws_url` | 持有 WebSocket、转发消息 |
| Gateway | WS 升级、认证、心跳、在线态 | 登录/OAuth、消息持久化 |
| Auth | 签发/撤销 JWT、refresh token | 长连接管理 |

---

## 3. 连接全流程

```mermaid
sequenceDiagram
    participant C as Client
    participant A as Auth
    participant E as Edge
    participant EC as etcd
    participant R as Redis
    participant G as Gateway

    C->>A: Login / RefreshToken
    A-->>C: access_token

    C->>E: POST /v1/route Authorization Bearer
    E->>E: 验签 JWT 解析 user_id
    E->>EC: Watch/List services/gateway
    E->>E: consistent_hash user_id
    E-->>C: gateway_id ws_url expires_at

    C->>G: WSS /ws token=access_token
    G->>G: 验签 JWT
    G->>R: 注册在线连接
    G->>EC: KeepAlive 租约 + 更新 conn_count
    G-->>C: 101 Switching Protocols

    loop 保活
        C->>G: ping
        G-->>C: pong
    end
```

### 3.1 客户端连接状态机

```mermaid
stateDiagram-v2
    [*] --> Disconnected
    Disconnected --> Authenticating: Auth Login OK
    Authenticating --> Routing: POST /v1/route
    Routing --> Connecting: 获得 ws_url
    Connecting --> Connected: WS 握手成功
    Connected --> Connected: ping/pong
    Connected --> Disconnected: 断线/踢下线
    Disconnected --> Connecting: 重连原 ws_url
    Disconnected --> Routing: 原 Gateway 不可达
    Connecting --> Authenticating: 401 token 过期
    Authenticating --> Connecting: RefreshToken 成功
```

---

## 4. Edge 服务设计

### 4.1 HTTP API

#### `POST /v1/route`

获取 Gateway 接入地址。需在请求中携带有效 `access_token`，Edge 验签后提取 `user_id` 做一致性哈希。

**Request**

```http
POST /v1/route HTTP/1.1
Host: edge.im.example.com
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "device_id": "optional-client-device-uuid"
}
```

**Response 200**

```json
{
  "gateway_id": "gw-02",
  "ws_url": "wss://gw-02.im.example.com/ws",
  "expires_at": "2026-06-19T12:10:00Z"
}
```

| 字段 | 说明 |
|------|------|
| `gateway_id` | 分配的 Gateway 实例 ID |
| `ws_url` | 客户端直连的 WebSocket 地址（不含 token） |
| `expires_at` | 路由建议有效期（默认 60s）；过期后应重新请求本接口 |

**错误响应**

| HTTP | 场景 |
|------|------|
| `401 Unauthorized` | token 缺失、无效或过期 |
| `503 Service Unavailable` | 无健康 Gateway 或全部达到连接上限 |

### 4.2 一致性哈希路由

**输入**

- `user_id`：从 JWT `sub` 解析
- 健康 Gateway 列表：来自 etcd Watch `/beehive-im/{env}/services/gateway/`，过滤 `status=online` 且 Lease 有效、`conn_count < max_conn`

**算法（v1）**

1. 对活跃 Gateway ID 列表按字典序排序（保证各 Edge 实例视图一致）
2. 使用 32 位哈希：`slot = crc32(user_id) % len(gateways)`
3. 选中 `gateways[slot]` 作为目标节点

**v2 演进**：虚拟节点环（每个 Gateway 映射 100~200 个 vnode），减少节点上下线时的槽位抖动。

**节点下线**

- Gateway Lease 过期（默认 30s）后 key 自动删除
- 仅影响**新连接**的路由；已建立 WebSocket 不受影响
- 客户端重连失败后重新请求 `/v1/route`，自动映射到环上其他节点

### 4.3 Edge 部署

- **无状态**：可多实例水平扩展，前置 L7 负载均衡
- **共享 etcd**：所有 Edge 实例 Watch 同一份 Gateway 注册前缀
- **不缓存路由结果**：每次 `/v1/route` 实时计算（开销极低）

---

## 5. Gateway 服务设计

### 5.1 WebSocket 端点

```
WSS /ws?token={access_token}
```

或使用标准 Upgrade 头（推荐，避免 token 出现在 URL 日志中）：

```http
GET /ws HTTP/1.1
Host: gw-02.im.example.com
Upgrade: websocket
Connection: Upgrade
Authorization: Bearer {access_token}
X-Device-Id: {device_id}
```

### 5.2 握手认证流程

```mermaid
flowchart TD
    A[收到 WS Upgrade 请求] --> B{解析 token}
    B -->|缺失| R401[401 Unauthorized]
    B -->|存在| C{JWT 验签}
    C -->|失败/过期| R401
    C -->|成功| D{可选: jti 黑名单}
    D -->|命中| R401
    D -->|未命中| E{device_id 有效?}
    E -->|否| R400[400 Bad Request]
    E -->|是| F{conn_count < max_conn?}
    F -->|否| R503[503 Service Unavailable]
    F -->|是| G[创建 Connection]
    G --> H[注册 Redis 在线态]
    H --> I[101 Switching Protocols]
```

1. 解析 `Authorization: Bearer` 或 query `token`
2. 本地验签 JWT，提取 `sub`（user_id）、`jti`
3. （v1.1）查询 Redis `jwt:bl:{jti}` 黑名单，命中则拒绝
4. 读取 `X-Device-Id` 或 query `device_id`；缺失时使用默认值 `default`
5. 检查本机连接数未达 `max_conn`
6. 若同 `(user_id, device_id)` 已有连接，**踢掉旧连接**（last-writer-wins）
7. 写入 Redis 在线表，返回 `101`

认证失败在 **HTTP 阶段**返回，不进入 WebSocket 协议。

### 5.3 连接模型

```mermaid
erDiagram
    User ||--o{ Connection : has
    Gateway ||--o{ Connection : hosts

    Connection {
        string conn_id PK
        bigint user_id
        string device_id
        string gateway_id
        timestamptz connected_at
    }
```

| 概念 | 说明 |
|------|------|
| `conn_id` | Gateway 内唯一，UUID |
| `device_id` | 客户端设备标识；同一用户可多设备同时在线 |
| 踢重 | 同一 `(user_id, device_id)` 新连接建立时，旧连接发送 `error` 帧后关闭 |

### 5.4 心跳保活

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 客户端 ping 间隔 | 30s | 应用层 JSON 或 WS Ping frame |
| 服务端读超时 | 60s | 超时未收到 ping 则断开 |
| Redis TTL 续期 | 每次 ping | `conn:meta:{conn_id}` 续期 90s |

**应用层心跳帧**

```json
{ "type": "ping", "seq": 1, "payload": {} }
{ "type": "pong", "seq": 1, "payload": {} }
```

### 5.5 Gateway 注册与心跳（etcd）

Gateway 启动后向 **etcd** 注册租约，并周期性 KeepAlive 上报负载；在线连接仍写 **Redis**。详见 [`docs/infrastructure/infrastructure.md`](../infrastructure/infrastructure.md)。

**生命周期**

```mermaid
stateDiagram-v2
    [*] --> Starting
    Starting --> Online: etcd Put + Lease
    Online --> Online: KeepAlive 更新 conn_count
    Online --> Draining: 收到 SIGTERM
    Draining --> Offline: 连接归零
    Online --> Offline: Lease 过期
    Offline --> [*]
```

| 阶段 | etcd `status` | Edge 行为 |
|------|---------------|-----------|
| 正常运行 | `online` | 参与一致性哈希 |
| 优雅下线 | `draining` | 跳过分配；已连接继续服务 |
| 宕机 | Lease 过期 key 删除 | 从 Watch 视图移除 |

**KeepAlive 间隔**：10s（Lease TTL 30s）；字段包括 `conn_count`、`ws_url`、`max_conn`。

---

## 6. 存储模型

服务注册走 **etcd**，用户在线态走 **Redis**，下行推送走 **RabbitMQ**。分工见 [`docs/infrastructure/infrastructure.md`](../infrastructure/infrastructure.md)。

### 6.1 etcd：Gateway 服务注册

```
/beehive-im/{env}/services/gateway/{gateway_id}
```

Value 为 JSON，含 `ws_url`、`status`、`conn_count`、`max_conn`、`region` 等；绑定 **Lease TTL 30s**，KeepAlive 10s。

Edge 通过 **Watch** 该前缀维护内存路由表，不缓存 `/v1/route` 结果。

### 6.2 Redis：在线态

#### Key 一览

| Key | 类型 | TTL | 说明 |
|-----|------|-----|------|
| `conn:user:{user_id}` | HASH | 无 | `device_id` → `{gateway_id}:{conn_id}`，只作为索引 |
| `conn:gateway:{gateway_id}` | SET | 无 | 本 Gateway 上的 `conn_id` 集合，只作为索引 |
| `conn:meta:{conn_id}` | HASH | 90s（ping 续期） | 连接元数据；在线判定以该 key 存在为准 |

#### 字段定义

**`conn:meta:{conn_id}`**

| 字段 | 说明 |
|------|------|
| `user_id` | 用户 ID |
| `device_id` | 设备 ID |
| `gateway_id` | 所属 Gateway |
| `connected_at` | 连接建立时间 |

### 6.3 在线态写入（连接建立）

必须通过 Redis Lua 脚本原子完成：

1. 读取同一 `user_id + device_id` 的旧连接路由。
2. 写入新的 `device_id -> gateway_id:conn_id` 索引。
3. 写入 `conn:gateway:{gateway_id}` 集合。
4. 写入 `conn:meta:{conn_id}` 并设置 TTL。
5. 返回旧连接路由，由 Gateway 关闭本实例旧连接；跨实例旧连接由 TTL 或后续 kick 事件清理。

### 6.4 在线态清理（连接断开）

必须通过 compare-and-delete Lua 脚本完成：只有 `conn:user:{user_id}` 中当前值仍等于 `{gateway_id}:{conn_id}` 时，才允许删除该 `device_id` 字段，避免同设备快速重连时旧连接误删新连接。

### 6.5 消息推送路由（RabbitMQ）

Message 服务向在线用户推送时：

1. `HGETALL conn:user:{user_id}` 获取候选设备连接（Redis）
2. 批量校验 `conn:meta:{conn_id}` 存在且字段一致，过滤残留索引
3. 按 `gateway_id` 分组 payload
4. 向 RabbitMQ exchange `beehive.im.push` 发布在线推送通知，routing key `push.gateway.{gateway_id}`
5. 目标 Gateway 从队列 `gateway.push.{gateway_id}` 消费，经本地 `conn_id` 写入 WebSocket

RabbitMQ push 只作为在线通知，消息事实源仍是 PostgreSQL；客户端重连后必须能通过 Message 服务按会话序号同步缺失消息。

详见 [`docs/infrastructure/infrastructure.md`](../infrastructure/infrastructure.md) 第 5 节。

---

## 7. WebSocket 消息帧（v1 骨架）

### 7.1 信封格式

所有应用层消息使用 JSON 信封：

```json
{
  "type": "ping | pong | ack | error | ...",
  "seq": 1,
  "payload": {}
}
```

| 字段 | 说明 |
|------|------|
| `type` | 消息类型 |
| `seq` | 单调递增序号，用于去重与 ack |
| `payload` | 业务载荷；v1 仅系统帧，业务帧由 Message 模块扩展 |

### 7.2 系统帧

**ping / pong**

```json
{ "type": "ping", "seq": 42, "payload": {} }
{ "type": "pong", "seq": 42, "payload": {} }
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
| `SERVER_DRAINING` | Gateway 优雅下线 |

### 7.3 认证与消息分离

- **认证**：仅在 HTTP Upgrade 阶段完成
- **业务消息**：握手成功后才开始收发；不在 WS 帧中传递 token

---

## 8. 重连与故障转移

| 场景 | 客户端行为 | 服务端行为 |
|------|------------|------------|
| 网络闪断 | 指数退避重连**原 ws_url**（最多 3 次） | Gateway 超时清理 Redis |
| 原 Gateway 不可达 | 重新 `POST /v1/route` | Edge 可能分配到新 Gateway |
| Gateway 宕机 | 重试 Edge 获新路由 | etcd Lease 过期后服务注册 key 自动删除，Redis 残留在线态由 TTL 和清理任务修复 |
| access_token 过期 | 握手 `401` → Auth `RefreshToken` → 重连 | — |
| 一致性哈希环变化 | 仅新 `/v1/route` 受影响 | 已建立连接不变 |
| 用户登出 | 关闭 WS + Auth `Logout` | Gateway 清理 Redis；revoke refresh token |

**推荐客户端重连策略**

1. 等待 `1s → 2s → 4s → 8s`（上限 30s）重连当前 `ws_url`
2. 连续失败 3 次后，调用 `/v1/route` 获取新地址
3. token 过期时先刷新再重连

---

## 9. 安全要求

| 项 | 要求 |
|----|------|
| 传输 | 生产环境全链路 HTTPS / WSS |
| Token 传递 | 优先 `Authorization` 头；避免 query token 落日志 |
| 验签密钥 | 从 etcd `secrets/global/jwt.secret` 读取；v1 不热更新，变更需滚动重启 |
| 连接上限 | Gateway `max_conn` + Edge 分配前检查 `conn_count` |
| 域名白名单 | `ws_url` 仅返回配置白名单内的域名 |
| 踢重 | 防止同设备多连接导致消息重复推送 |
| 黑名单 | v1.1 支持 `jwt:bl:{jti}` 实现登出后即时失效 |

---

## 10. 配置项

### 10.1 Edge

| 环境变量 / 配置 | 说明 | 示例 |
|-----------------|------|------|
| `ETCD_ENDPOINTS` | etcd 端点（bootstrap） | `127.0.0.1:2379` |
| `BEEHIVE_ENV` | 环境名 | `dev` |
| `EDGE_LISTEN` | HTTP 监听地址 | `:8080` |
| `config/edge/route.ttl` | 路由建议有效期（etcd） | `60s` |
| `config/edge/gateway.heartbeat_timeout` | Lease 判定（etcd） | `30s` |
| `secrets/global/jwt.secret` | JWT 验签密钥（etcd） | — |

### 10.2 Gateway

| 环境变量 / 配置 | 说明 | 示例 |
|-----------------|------|------|
| `ETCD_ENDPOINTS` | etcd 端点（bootstrap） | `127.0.0.1:2379` |
| `BEEHIVE_ENV` | 环境名 | `dev` |
| `GATEWAY_ID` | 实例唯一 ID | `gw-02` |
| `GATEWAY_WS_URL` | 对外 WebSocket 基址 | `wss://gw-02.im.example.com/ws` |
| `GATEWAY_LISTEN` | WS 监听地址 | `:9000` |
| `REDIS_ADDR` | Redis 地址（在线态） | `127.0.0.1:6379` |
| `config/gateway/ws.ping_interval` | 期望客户端 ping 间隔 | `30s` |
| `config/gateway/ws.read_timeout` | 读超时 | `60s` |
| `config/gateway/max_conn` | 最大连接数 | `10000` |
| `secrets/global/jwt.secret` | JWT 验签密钥 | — |
| `RABBITMQ_URL` | 消费 push 队列 | `amqp://guest:guest@127.0.0.1:5672/` |

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
│   ├── handler/
│   │   └── route.go              # POST /v1/route
│   ├── router/
│   │   └── consistent.go         # 一致性哈希
│   ├── registry/
│   │   └── gateway.go            # etcd Watch Gateway 列表
│   └── auth/
│       └── jwt.go                # 本地验签
└── pkg/

services/gateway/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── server/
│   │   └── ws.go
│   ├── ws/
│   │   ├── upgrade.go
│   │   └── auth.go               # 握手 JWT 校验
│   ├── conn/
│   │   └── manager.go            # 连接池、踢重
│   ├── presence/
│   │   └── redis.go              # 在线态读写
│   ├── registry/
│   │   └── heartbeat.go          # etcd 租约注册与 KeepAlive
│   └── frame/
│       └── codec.go              # JSON 信封编解码
└── pkg/
```

### 11.1 关键接口

```go
// GatewayRegistry watches gateway instances from etcd.
// GatewayRegistry 从 etcd Watch Gateway 实例。
type GatewayRegistry interface {
    Watch(ctx context.Context) (<-chan GatewayEvent, error)
    List(ctx context.Context) ([]GatewayNode, error)
}

// ConsistentRouter picks a gateway for the given user ID.
// ConsistentRouter 按 user_id 一致性哈希选择 Gateway。
type ConsistentRouter interface {
    Route(userID string, nodes []GatewayNode) (*GatewayNode, error)
}

// PresenceStore tracks online connections in Redis.
// PresenceStore 在 Redis 中维护在线连接。
type PresenceStore interface {
    Register(ctx context.Context, conn Connection) error
    Unregister(ctx context.Context, connID string) error
    LookupByUser(ctx context.Context, userID string) ([]Connection, error)
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

    Client --> Auth
    Client --> Edge
    Client --> Gateway
    Gateway --> Etcd[(etcd)]
    Edge --> Etcd
    Gateway --> Redis[(Redis)]
    Msg --> Redis
    Msg -.-> RMQ[(RabbitMQ)]
    RMQ -.->|push notification| Gateway
    User --> PG[(PostgreSQL)]
```

| 服务 | 协作方式 |
|------|----------|
| **Auth** | 客户端登录获取 JWT；Gateway/Edge 本地验签 |
| **User** | 不直接参与连接；资料查询独立走 gRPC |
| **Message**（未来） | 经 Redis 查在线态，经 RabbitMQ 向 Gateway 推送 |
| **Redis** | 用户在线态 |
| **etcd** | Gateway/Edge/Auth 服务注册；运行时配置与密钥 |
| **RabbitMQ** | 领域事件；Message → Gateway 下行推送 |

中间件职责详见 [`docs/infrastructure/infrastructure.md`](../infrastructure/infrastructure.md)。

详见 [`docs/auth/DESIGN.md`](../auth/DESIGN.md)。

---

## 13. 运维

| 任务 | 方式 |
|------|------|
| Gateway 扩缩容 | 增减实例；新实例 etcd 注册后 Edge Watch 自动感知 |
| Gateway 优雅下线 | `status=draining` → 等待连接归零 → 撤销 Lease |
| 僵尸连接清理 | `conn:meta:{conn_id}` TTL 过期 + Gateway 读超时双保险 |
| Edge 扩容 | 无状态加实例 + LB |
| 监控指标 | `conn_count`、`route_latency`、`ws_handshake_fail`、`heartbeat_timeout` |

---

## 14. 演进路线

| 阶段 | 内容 |
|------|------|
| v1（当前） | Edge 路由 + Gateway WS 接入 + etcd 注册 + Redis 在线态 |
| v1.1 | JWT 黑名单；Gateway 强制 token 续期校验 |
| v2 | Message 服务对接 RabbitMQ push；虚拟节点一致性哈希 |
| v2 | 虚拟节点一致性哈希；按 `region` 就近路由 |
| v3 | RS256 + JWKS；Edge 前置 GeoDNS |
| v3 | 跨机房 Gateway 联邦 |

---

## 15. 附录：HTTP / WS 错误码

### Edge HTTP

| 状态码 | `error_code` | 说明 |
|--------|--------------|------|
| 401 | `INVALID_TOKEN` | JWT 无效或过期 |
| 503 | `NO_GATEWAY_AVAILABLE` | 无健康节点 |
| 503 | `GATEWAY_CAPACITY_EXCEEDED` | 目标节点连接已满 |

### Gateway WebSocket 握手

| 状态码 | 说明 |
|--------|------|
| 401 | token 无效或过期 |
| 400 | 缺少 device_id（若配置为必填） |
| 503 | 本机连接数已达上限 |

### 应用层 `error` 帧 `code`

| code | 说明 |
|------|------|
| `KICKED_DUPLICATE_DEVICE` | 同设备被新连接踢下线 |
| `SERVER_DRAINING` | Gateway 优雅下线 |
| `INVALID_FRAME` | 帧格式错误 |
