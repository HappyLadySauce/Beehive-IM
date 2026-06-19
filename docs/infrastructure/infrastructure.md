# Beehive-IM 基础设施与中间件

> 版本：v1.0  
> 适用范围：PostgreSQL、Redis、etcd、RabbitMQ 的职责划分与协作约定  
> 关联文件：[`docs/auth/DESIGN.md`](../auth/DESIGN.md)、[`docs/gateway/DESIGN.md`](../gateway/DESIGN.md)、[`docker/Infrastructure/docker-compose.yaml`](../../docker/Infrastructure/docker-compose.yaml)

---

## 1. 概述

Beehive-IM 使用四类中间件，各司其职、**禁止职责重叠**：

```mermaid
flowchart TB
    subgraph persist [持久化]
        PG[(PostgreSQL)]
    end

    subgraph coord [协调与配置]
        Etcd[(etcd)]
    end

    subgraph cache [缓存与在线态]
        Redis[(Redis)]
    end

    subgraph async [异步消息]
        RMQ[(RabbitMQ)]
    end

    Auth --> PG
    Auth --> Redis
    Auth --> Etcd
    Gateway --> Etcd
    Gateway --> Redis
    Gateway --> RMQ
    Edge --> Etcd
    Message --> PG
    Message --> RMQ
    RMQ -->|push consume| Gateway
```

| 中间件 | 职责 | 不做 |
|--------|------|------|
| **PostgreSQL** | 用户、认证、消息等业务持久化 | 在线态、服务注册、异步投递 |
| **Redis** | OAuth state、JWT 黑名单、用户在线态 `conn:*` | 服务注册、Gateway 下行推送 |
| **etcd** | 服务注册（Lease + Watch）、运行时配置与 secrets | 业务数据、消息队列 |
| **RabbitMQ** | 领域事件；Message → Gateway 下行推送 | 长连接、服务发现 |

### 1.1 禁止事项

- Gateway 实例注册**只写 etcd**，不写 Redis `gw:registry` / `gw:alive`
- Gateway 下行推送**只走 RabbitMQ**，不使用 Redis Pub/Sub
- 业务持久化数据**只写 PostgreSQL**，不经 etcd / Redis 长期保存

---

## 2. PostgreSQL

### 2.1 用途

| 域 | 表 / 说明 |
|----|-----------|
| 用户 | `users`、`user_profiles` — 见 [`sql/migrations/users/001_user.sql`](../../sql/migrations/users/001_user.sql) |
| 认证 | `user_oauth_identities`、`refresh_tokens` — 见 [`sql/migrations/auth/002_auth.sql`](../../sql/migrations/auth/002_auth.sql) |
| 消息（未来） | `conversations`、`messages` 等 |

### 2.2 迁移

- CLI：[`sql/migrate/main.go`](../../sql/migrate/main.go)
- 目录：[`sql/migrations/`](../../sql/migrations/)（按域分子目录 `users/`、`auth/` 等）
- 入口脚本：`sql/migrate.ps1` / `sql/migrate.sh`

### 2.3 连接

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `DB_DSN` | PostgreSQL 连接串 | `postgres://user:pass@127.0.0.1:5432/Beehive-IM?sslmode=disable` |

生产可将 `config/auth/db.dsn` 等写入 etcd；本地开发可直接使用 `DB_DSN`。

### 2.4 部署

| 环境 | 拓扑 |
|------|------|
| 本地 | 单实例（docker-compose `postgresql:16`） |
| 生产 | 主从或云 RDS；定期备份 |

---

## 3. Redis

### 3.1 用途

高频读写、短 TTL、IM 运行时态数据。

| 数据 | Key 模式 | TTL | 服务 |
|------|----------|-----|------|
| OAuth 授权 state | `oauth:state:{state}` | 600s | Auth |
| JWT 黑名单（v1.1） | `jwt:bl:{jti}` | 至 token 过期 | Auth / Gateway |
| 用户设备在线映射 | `conn:user:{user_id}` | 无 | Gateway |
| Gateway 连接集合 | `conn:gateway:{gateway_id}` | 无 | Gateway |
| 连接元数据 | `conn:meta:{conn_id}` | 90s（ping 续期） | Gateway |

**OAuth state Value（JSON）**

```json
{
  "redirect_uri": "https://app.example.com/oauth/github/callback",
  "scope": "read:user user:email",
  "intent": "login",
  "user_id": null
}
```

`intent=link` 时 `user_id` 为本地用户 ID 字符串。

### 3.2 在线态操作

连接建立：

```
HSET conn:user:{user_id} {device_id} {gateway_id}:{conn_id}
SADD conn:gateway:{gateway_id} {conn_id}
HSET conn:meta:{conn_id} user_id ... device_id ... gateway_id ... connected_at ...
EXPIRE conn:meta:{conn_id} 90
```

连接断开：

```
HDEL conn:user:{user_id} {device_id}
SREM conn:gateway:{gateway_id} {conn_id}
DEL conn:meta:{conn_id}
```

### 3.3 连接

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `REDIS_ADDR` | Redis 地址 | `127.0.0.1:6379` |

### 3.4 部署

| 环境 | 拓扑 |
|------|------|
| 本地 | 单实例 AOF（docker-compose `redis:8-alpine`） |
| 生产 | 哨兵或集群；在线态可接受短暂不一致 |

---

## 4. etcd

### 4.1 用途

| 职责 | 说明 |
|------|------|
| **服务注册中心** | 实例租约注册、健康摘除、Edge Watch Gateway |
| **配置中心** | 运行时配置与 secrets；Watch 热更新 |

### 4.2 Bootstrap

仅以下两项允许纯环境变量注入，其余从 etcd 加载：

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `ETCD_ENDPOINTS` | 逗号分隔端点 | `127.0.0.1:2379` |
| `BEEHIVE_ENV` | 环境名，参与 key 前缀 | `dev` / `staging` / `prod` |

本地 etcd 无配置时，可回退读 `configs/{env}/*.yaml`（实现层可选）。

### 4.3 Key 命名

统一前缀：`/beehive-im/{env}/...`

**服务注册**

```
/beehive-im/{env}/services/{service_name}/{instance_id}
```

**配置**

```
/beehive-im/{env}/config/global/{key}
/beehive-im/{env}/config/{service_name}/{key}
/beehive-im/{env}/secrets/{service_name}/{key}
```

**服务注册 Value（JSON）示例**

```json
{
  "instance_id": "gw-02",
  "service": "gateway",
  "host": "10.0.1.12",
  "http_addr": "10.0.1.12:9000",
  "grpc_addr": "10.0.1.12:9100",
  "ws_url": "wss://gw-02.im.example.com/ws",
  "status": "online",
  "conn_count": 128,
  "max_conn": 10000,
  "region": "cn-east",
  "version": "v1.0.0",
  "started_at": "2026-06-19T08:00:00Z"
}
```

### 4.4 租约注册

| 参数 | 默认值 |
|------|--------|
| Lease TTL | 30s |
| KeepAlive 间隔 | 10s |

```mermaid
sequenceDiagram
    participant S as ServiceInstance
    participant E as etcd
    participant C as Edge

    S->>E: Grant Lease TTL=30s
    S->>E: Put service key with Lease
    loop every 10s
        S->>E: KeepAlive
    end
    C->>E: Watch services/gateway/
    Note over S,E: 进程崩溃则 Lease 过期 key 自动删除
```

优雅下线：`status=draining` → 等待连接归零 → 撤销 Lease。

**Edge 发现 Gateway**：Watch `/beehive-im/{env}/services/gateway/`；冷启动 `Get` 全量 + `Watch` 增量。

### 4.5 各服务接入

| 服务 | 注册 | Watch 配置 | Watch 其他服务 |
|------|------|------------|----------------|
| gateway | 是 | 是 | 否 |
| edge | 是 | 是 | `services/gateway` |
| auth | 是 | 是 | 可选 `services/user` |
| user | 是 | 是 | 否 |
| message | 是 | 是 | `services/gateway` |

### 4.6 配置热更新

| 配置 | 热更新 |
|------|--------|
| `route.ttl`、`ws.ping_interval` | 是 |
| `jwt.access_ttl`（新签发 token） | 是 |
| `jwt.secret` | 否，需滚动重启 |
| 监听端口 | 否 |

**JWT 验签密钥 canonical key**（全局唯一）：

```
/beehive-im/{env}/secrets/global/jwt.secret
```

### 4.7 建议初始 config seed

| Key | 默认值 |
|-----|--------|
| `config/global/jwt.access_ttl` | `15m` |
| `config/global/jwt.refresh_ttl` | `720h` |
| `config/edge/route.ttl` | `60s` |
| `config/edge/gateway.heartbeat_timeout` | `30s` |
| `config/gateway/ws.ping_interval` | `30s` |
| `config/gateway/ws.read_timeout` | `60s` |
| `config/gateway/max_conn` | `10000` |
| `config/auth/oauth.state_ttl` | `600s` |
| `secrets/global/jwt.secret` | （部署时写入） |
| `secrets/auth/github.client_id` | （部署时写入） |
| `secrets/auth/github.client_secret` | （部署时写入） |

### 4.8 部署与安全

| 环境 | 拓扑 |
|------|------|
| 本地 | 单节点（docker-compose `etcd:v3.5.16`，端口 `2379`） |
| 生产 | 3 节点奇数集群；TLS + RBAC；`secrets/*` 按服务限制读写 |

推荐 Go 客户端：`go.etcd.io/etcd/client/v3`。

---

## 5. RabbitMQ

### 5.1 用途

| 场景 | 说明 |
|------|------|
| **领域事件** | 消息已存、会话变更等异步通知，供审计/搜索/统计等消费 |
| **Gateway 下行推送** | Message 查 Redis 在线态后，向目标 Gateway 投递 push 消息 |

**不使用 Redis Pub/Sub 做 Gateway 推送**——RabbitMQ 提供持久化、ack、DLQ，更适合 IM 投递。

### 5.2 Exchange 规划

| Exchange | 类型 | 说明 |
|----------|------|------|
| `beehive.im.events` | topic | 领域事件 |
| `beehive.im.push` | topic | Message → Gateway 下行 |
| `beehive.im.dlq` | topic | 死信（失败消息） |

### 5.3 路由键

| 路由键 | 用途 |
|--------|------|
| `message.created.{conversation_id}` | 消息已持久化事件 |
| `conversation.updated.{conversation_id}` | 会话元数据变更 |
| `user.online.{user_id}` | 用户上线（可选） |
| `push.gateway.{gateway_id}` | 定向推送到 Gateway 实例 |

### 5.4 Gateway 下行推送流

```mermaid
sequenceDiagram
    participant Msg as Message
    participant RMQ as RabbitMQ
    participant Redis as Redis
    participant GW as Gateway
    participant C as Client

    Msg->>Msg: 消息落库 PostgreSQL
    Msg->>Redis: HGETALL conn:user:user_id
    Msg->>RMQ: publish push.gateway.gw-02
    RMQ->>GW: deliver gateway.push.gw-02
    GW->>C: WebSocket 下行帧
    GW->>RMQ: ack
```

**队列绑定**

- 每个 Gateway 实例声明独占队列：`gateway.push.{gateway_id}`
- 绑定：`beehive.im.push` exchange，routing key `push.gateway.{gateway_id}`
- 消息 `delivery_mode=2`（持久化）；手动 ack；失败 nack 进 `beehive.im.dlq`

**Message 侧逻辑**

1. 确定收件人 `user_id` 列表
2. `HGETALL conn:user:{user_id}` 得 `device_id → gateway_id:conn_id`
3. 按 `gateway_id` 聚合 payload
4. 对每个 `gateway_id` 发布 `push.gateway.{gateway_id}`

### 5.5 领域事件消费（后续）

| 消费者 | 订阅示例 | 用途 |
|--------|----------|------|
| 审计服务 | `message.created.#` | 审计日志 |
| 搜索索引 | `message.created.#` | 全文索引 |
| 推送编排 | `message.created.#` | 触发 Gateway push |

各服务声明独立队列绑定 `beehive.im.events`，避免广播争抢。

### 5.6 连接

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `RABBITMQ_URL` | AMQP 连接串 | `amqp://guest:guest@127.0.0.1:5672/` |

### 5.7 部署

| 环境 | 拓扑 |
|------|------|
| 本地 | 单节点 + management UI（docker-compose，端口 `5672` / `15672`） |
| 生产 | 3 节点镜像队列或 Quorum Queue；启用 TLS |

---

## 6. 本地开发（docker-compose）

[`docker/Infrastructure/docker-compose.yaml`](../../docker/Infrastructure/docker-compose.yaml) 提供：

| 服务 | 镜像 | 端口 |
|------|------|------|
| postgresql | postgres:16 | 5432 |
| redis | redis:8-alpine | 6379 |
| etcd | etcd:v3.5.16 | 2379 |
| rabbitmq | rabbitmq:3-management-alpine | 5672, 15672 |

启动后环境变量示例：

```bash
DB_DSN=postgres://Beehive-Blog:Beehive-Blog@127.0.0.1:5432/Beehive-Blog?sslmode=disable
REDIS_ADDR=127.0.0.1:6379
ETCD_ENDPOINTS=127.0.0.1:2379
BEEHIVE_ENV=dev
RABBITMQ_URL=amqp://guest:guest@127.0.0.1:5672/
```

---

## 7. 环境变量汇总

| 变量 | 中间件 | Bootstrap |
|------|--------|-----------|
| `DB_DSN` | PostgreSQL | 是 |
| `REDIS_ADDR` | Redis | 是 |
| `ETCD_ENDPOINTS` | etcd | 是 |
| `BEEHIVE_ENV` | etcd（key 前缀） | 是 |
| `RABBITMQ_URL` | RabbitMQ | 是 |
| `secrets/global/jwt.secret` 等 | etcd | 否，从 etcd 加载 |

---

## 8. 服务与中间件依赖矩阵

| 服务 | PostgreSQL | Redis | etcd | RabbitMQ |
|------|:----------:|:-----:|:----:|:--------:|
| Auth | 读写 | OAuth state | 注册 + 配置 | — |
| User | 读写 | — | 注册 + 配置 | 可选消费事件 |
| Edge | — | — | 注册 + 配置 + Watch gateway | — |
| Gateway | — | 在线态 | 注册 + 配置 | 消费 push 队列 |
| Message | 读写 | 查在线态 | 注册 + 配置 | 发布 events + push |

---

## 9. 共享 pkg 目录建议

```
pkg/
├── etcd/
│   ├── registry/     # Register + Watch
│   └── config/       # Load + Watch
└── rabbitmq/
    ├── publisher.go
    └── consumer.go
```

---

## 10. 演进路线

| 阶段 | 内容 |
|------|------|
| v1 | 四类中间件 docker-compose；etcd 注册；Redis 在线态；RabbitMQ exchange/queue 约定 |
| v1.1 | JWT 黑名单（Redis）；RabbitMQ Gateway push 联调 |
| v2 | etcd 3 节点 + TLS；RabbitMQ Quorum Queue；gRPC etcd resolver |
| v3 | 跨机房；配置灰度；RabbitMQ 联邦（如需要） |
