# Beehive-IM Message 服务与 Web 客户端同步设计文档

> 版本：v1.1
> 适用范围：Message 服务、消息持久化、Outbox、消息同步、Web 客户端重连与补偿协议
> 关联文件：[`docs/gateway/DESIGN.md`](../gateway/DESIGN.md)、[`docs/conversation/DESIGN.md`](../conversation/DESIGN.md)、[`docs/notification/DESIGN.md`](../notification/DESIGN.md)、[`docs/presence/DESIGN.md`](../presence/DESIGN.md)、[`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md)

---

## 1. 目标与范围

Message 服务是 IM 消息事实边界，负责消息写入、会话内序号、消息同步、回执和 outbox 事件发布。Notification 只消费消息事件做在线/离线通知，不能成为消息事实源；Edge 和 Gateway 只承载连接与协议转发，不保存长期消息状态。

现阶段客户端只考虑 Web 端。Web 客户端协议必须处理浏览器限制、多标签页、断网重连、重复提交、乱序推送和本地缓存恢复。

### 1.1 职责

| 职责 | 说明 |
|------|------|
| 消息写入 | 校验发送权限，分配会话内 `seq`，持久化消息 |
| 幂等发送 | 使用 `sender_id + device_id + client_msg_id` 保证重复发送返回同一结果 |
| 消息同步 | 按 `conversation_id + seq` 提供增量拉取和缺口补齐 |
| 消息回执 | 接收 Web 客户端 delivered/read ack，写入回执表 |
| Outbox | 消息落库同事务写入 `outbox_events`，由 dispatcher 可靠发布领域事件 |
| Web 协议 | 定义浏览器连接、重连、IndexedDB 游标和多标签页协作规则 |

### 1.2 非职责

- 不管理 WebSocket 连接，连接由 Edge/Gateway 负责。
- 不查询 Presence，也不直接发布 `push.edge`。
- 不解析 Notification 离线 provider 策略。
- 不维护会话成员、免打扰和管理权限，这些属于 Conversation。
- v1 不做移动端原生协议、端到端加密、附件大文件存储和全文检索。

### 1.3 关键决策

| 决策 | 选择 |
|------|------|
| 消息事实源 | PostgreSQL `messages` |
| 会话序号 | 每个 `conversation_id` 内单调递增 `seq` |
| 发送幂等 | `sender_id + device_id + client_msg_id` 唯一 |
| 推送可靠性 | WebSocket push 是通知，不是可靠事实 |
| 补偿同步 | Web 客户端按 `conversation_id + last_contiguous_seq` 拉取缺失消息 |
| WebSocket 鉴权 | Web 端使用一次性短 TTL `ws_ticket`，不把 JWT 放入 WS query |
| HTTP 鉴权 | Edge HTTP API 默认使用 `Authorization: Bearer {access_token}`，dev/test 可按配置回退 `X-Debug-*` |
| 多标签页 | 同一浏览器 profile 只保留一个 WebSocket leader tab |

---

## 2. 总体架构

```mermaid
flowchart LR
    Web[Web Client]
    Edge[Edge / API Gateway]
    Gateway[Gateway]
    Message[Message Service]
    Conversation[Conversation Service]
    PG[(PostgreSQL)]
    RMQ[(RabbitMQ)]
    Notification[Notification Service]

    Web -->|HTTPS send/sync or WSS frames| Edge
    Edge --> Gateway
    Gateway --> Message
    Message -->|permission / seq| Conversation
    Message -->|Tx messages + outbox| PG
    Message -->|outbox publish message.created| RMQ
    RMQ --> Notification
    Notification -->|push.edge| RMQ
    RMQ --> Edge
    Edge -->|message frame| Web
```

### 2.1 数据流闭环

```mermaid
sequenceDiagram
    participant W as Web Client
    participant E as Edge
    participant G as Gateway
    participant M as Message
    participant C as Conversation
    participant PG as PostgreSQL
    participant RMQ as RabbitMQ
    participant N as Notification

    W->>E: message.send(client_msg_id)
    E->>G: upstream frame
    G->>M: SendMessage
    M->>C: CheckSendPermission + AllocateMessageSeq
    M->>PG: Tx insert message + outbox event
    M-->>G: persisted(message_id, seq)
    G-->>E: message.persisted
    E-->>W: message.persisted
    M->>RMQ: outbox dispatcher publishes message.created
    RMQ->>N: message.created
    N->>RMQ: push.edge.{edge_id}
    RMQ->>E: edge push
    E-->>W: message.new
    W->>E: message.ack(delivered/read)
```

发送方收到 `message.persisted` 只代表服务端已持久化，不代表其他客户端已接收。接收方收到 `message.new` 后仍要按 `seq` 校验是否有缺口。

---

## 3. 服务 API 契约

Web 客户端通过 Edge/API Gateway 暴露的 HTTP JSON API 或 WebSocket frame 访问；内部服务建议使用 gRPC。HTTP 和 WebSocket 入口必须复用同一组 Message 应用服务，避免两套语义。

### 3.1 Web HTTP API

| API | 说明 |
|-----|------|
| `POST /v1/auth/register` | 本地账号注册，返回 access token 与 refresh token |
| `POST /v1/auth/login` | 本地账号登录 |
| `POST /v1/auth/refresh` | 轮换 refresh token 并签发新 access token |
| `POST /v1/auth/logout` | 撤销 refresh token |
| `POST /v1/ws/ticket` | 使用已认证 HTTP 会话换取一次性 WebSocket ticket |
| `POST /v1/conversations/direct` | 创建或返回同一用户对唯一单聊 |
| `POST /v1/conversations/group` | 创建群聊，creator 为 owner |
| `GET /v1/conversations` | 获取会话列表，包含 last message、unread、成员 read state |
| `GET /v1/conversations/{conversation_id}/messages?after_seq=&before_seq=&direction=&limit=` | 拉取某会话历史或增量消息 |
| `POST /v1/messages/sync` | 批量按多个会话 cursor 拉取缺失消息 |
| `POST /v1/messages/ack` | 上报 delivered/read 回执 |

`POST /v1/ws/ticket` 返回：

```json
{
  "ticket": "wst_opaque_random",
  "expires_in": 30,
  "session_id": "sess_01",
  "device_id": "web_device_01"
}
```

生产 ticket 要求：

- TTL 默认 30s，最多 60s。
- 单次使用，消费后立即失效。
- 绑定 `user_id`、`device_id`、`session_id`、origin 和 user-agent hash。
- ticket 可放入 WebSocket query；JWT/access token 禁止放入 query。

当前 MVP 实现状态：

| 能力 | 当前状态 |
|------|----------|
| 签发入口 | `POST /v1/ws/ticket` 已在 Edge 服务实现 |
| Web 端连接 | `GET /ws?ticket=...` 已在 Edge 服务实现 WebSocket upgrade |
| ticket 存储 | 当前为 Edge 进程内内存 store，只适合单 Edge 本地开发 |
| 鉴权来源 | Edge 默认校验 Bearer JWT；仅 `Env=dev/test` 且 `DevAuth.Enabled=true` 时允许 `X-Debug-*` 回退 |
| 绑定字段 | 当前绑定 `user_id`、`device_id`、`session_id`、Origin；user-agent hash 后续补齐 |
| 在线态 | Edge 建连后调用 Presence，Presence 将 session route 写入 Redis |
| Gateway 恢复 | Edge 已有基础 rebind/resume 骨架，Gateway stream 断开后可在同一 WebSocket 内切换上游；消息缺口仍依赖后续 Message 同步补偿 |
| 在线 push | Notification 已消费 `message.created.#`，查询 Conversation + Presence，并发布 `push.edge.{edge_id}`；Edge 从 `edge.push.{edge_id}` 队列写本机 WebSocket |
| Conversation | 已实现单聊唯一、群聊基础生命周期、成员可见范围、read state、成员权限校验和 `AllocateMessageSeq` |
| 会话列表 | Edge 组合 Conversation `ListConversations` 与 Message `GetConversationSummaries`，返回 last message 和精确 unread count |
| Message | 已实现 `SendMessage`、`AckMessages`、`ListMessages`、`SyncMessages`、`GetConversationSummaries`、发送幂等和 outbox 写入 |
| Gateway 业务帧 | Gateway 已接入 `message.send` 和 `message.ack`，通过 Message zRPC 返回 `message.persisted` / `message.ack` |
| Outbox | Message 内置 dispatcher，发布 `message.created.{conversation_id}` 到 RabbitMQ exchange `beehive.im.events`，使用 publisher confirm 和有界重试 |
| 消息同步 | Message `ListMessages` / `SyncMessages` 已按 Conversation 可见 seq 范围过滤，Edge 已暴露 Web HTTP 同步、历史分页和 ack 入口 |

### 3.2 内部 gRPC API

| RPC | 调用方 | 说明 |
|-----|--------|------|
| `SendMessage` | Gateway | 已实现：发送消息并持久化，写入同事务 outbox |
| `AckMessages` | Gateway | 已实现：写 delivered/read 回执 |
| `SyncMessages` | Edge/API Gateway | 已实现：按 cursor 批量同步缺失消息 |
| `ListMessages` | Edge/API Gateway | 已实现：拉取单会话消息列表 |
| `GetConversationSummaries` | Edge/API Gateway | 已实现：按可见范围返回 last message 和精确 unread count |

### 3.3 SendMessage 请求

```json
{
  "conversation_id": "conv_01",
  "client_msg_id": "01J4WEB7Y6G7Y4V5S8K6Z9G8T1",
  "device_id": "web_device_01",
  "content_type": "text",
  "content": {
    "text": "hello"
  },
  "reply_to_message_id": null
}
```

字段要求：

| 字段 | 要求 |
|------|------|
| `conversation_id` | 必填，服务端校验用户是否是成员 |
| `client_msg_id` | 必填，Web 端生成 ULID/UUID，单设备内唯一 |
| `device_id` | 来自服务端登记或 Web 本地持久化，禁止由请求任意覆盖用户身份 |
| `content_type` | 当前仅支持 `text`；附件、system 消息后续扩展 |
| `content` | JSONB，当前必须是 `{ "text": "..." }`，服务端限制文本和 JSON 大小 |

成功响应：

```json
{
  "message_id": "msg_01",
  "conversation_id": "conv_01",
  "client_msg_id": "01J4WEB7Y6G7Y4V5S8K6Z9G8T1",
  "seq": 1024,
  "sender_id": 1001,
  "created_at": "2026-06-20T04:00:00Z",
  "status": "persisted"
}
```

重复提交同一 `sender_id + device_id + client_msg_id` 必须返回第一次持久化的结果，不能插入第二条消息。

---

## 4. 数据模型

### 4.1 PostgreSQL 表

| 表 | 说明 |
|----|------|
| `messages` | 消息事实表 |
| `message_receipts` | 用户维度 delivered/read 回执 |
| `outbox_events` | 待发布领域事件 |

`messages` 建议字段：

| 字段 | 说明 |
|------|------|
| `message_id` | 消息 ID，UUID |
| `conversation_id` | 会话 ID |
| `seq` | 会话内单调递增序号 |
| `sender_id` | 发送者用户 ID |
| `device_id` | 发送设备 ID |
| `client_msg_id` | Web 客户端生成的幂等 ID |
| `content_type` | 消息类型 |
| `content_json` | JSONB 内容 |
| `client_seq` | WebSocket 客户端帧序号，辅助排查与恢复 |
| `created_at` / `deleted_at` | 生命周期时间 |

唯一约束：

| 约束 | 说明 |
|------|------|
| `UNIQUE(conversation_id, seq)` | 保证会话内序号唯一 |
| `UNIQUE(sender_id, device_id, client_msg_id)` | 保证发送幂等 |

`message_receipts` 建议字段：

| 字段 | 说明 |
|------|------|
| `conversation_id` | 会话 ID |
| `user_id` | 回执用户 |
| `message_seq` | 会话内消息序列号 |
| `delivered_at` | Web 客户端持久化到本地后的时间 |
| `read_at` | 用户实际读到该消息的时间 |

`outbox_events` 的 `payload` 必须包含 `event_id`、`message_id`、`conversation_id`、`seq`、`sender_id`、`created_at`。

### 4.2 序号分配

v1 使用 Conversation `AllocateMessageSeq` 分配会话内序号，Message 随后在本服务事务中写入 `messages + outbox_events`。由于 seq 分配和 Message 写库跨服务，Message 写入失败时可能产生会话内 seq gap；客户端不能把 gap 直接视为消息丢失，必须通过后续 `SyncMessages` 补偿接口确认缺失范围。Message 仍必须依赖 `UNIQUE(conversation_id, seq)` 兜底，处理重试或并发下的重复序号。

如果后续把序号分配移动到 Message 内部，必须保证会话行锁、唯一索引和事务边界仍然清晰，不能让 Gateway 或客户端参与序号分配。

---

## 5. Outbox 与事件

消息持久化和 outbox 写入必须在同一数据库事务中完成。

```mermaid
flowchart LR
    Message[Message Service] -->|Tx| PG[(PostgreSQL)]
    PG --> Messages[(messages)]
    PG --> Outbox[(outbox_events)]
    Dispatcher[Outbox Dispatcher] --> Outbox
    Dispatcher -->|publisher confirm| RMQ[(RabbitMQ)]
```

### 5.1 message.created 事件

Routing key：

```text
message.created.{conversation_id}
```

Payload：

```json
{
  "event_id": "evt_01",
  "event_type": "message.created",
  "message_id": "msg_01",
  "conversation_id": "conv_01",
  "seq": 1024,
  "sender_id": 1001,
  "content_type": "text",
  "preview": "short safe preview",
  "created_at": "2026-06-20T04:00:00Z"
}
```

当前实现的 `message.created` 事件包含 `content_type` 和 text JSON content，用于 v1 Web 在线 push；Message/PostgreSQL 仍是最终事实源。后续引入敏感内容、端到端加密或复杂权限后，事件 payload 应收敛为最小通知载荷，接收端通过 Message 同步接口拉取完整内容。

### 5.2 Dispatcher 要求

- 使用 `SELECT ... FOR UPDATE SKIP LOCKED` 支持多实例并发。
- RabbitMQ publisher confirm 成功后才能标记 outbox `published`。
- 失败使用指数退避和最大重试次数，超过阈值进入 DLQ 或人工处理队列。
- 发布重试必须幂等，消费者以 `event_id` 去重。

---

## 6. WebSocket 帧

Gateway 文档中的 WebSocket 信封 `seq` 是传输帧序号；Message 的 `payload.message.seq` 是会话内消息序号。实现和前端状态管理中必须区分这两类序号。

### 6.1 message.send

```json
{
  "type": "message.send",
  "seq": 101,
  "request_id": "req_01",
  "payload": {
    "conversation_id": "conv_01",
    "client_msg_id": "01J4WEB7Y6G7Y4V5S8K6Z9G8T1",
    "content_type": "text",
    "content": {
      "text": "hello"
    }
  }
}
```

### 6.2 message.persisted

```json
{
  "type": "message.persisted",
  "seq": 201,
  "request_id": "req_01",
  "payload": {
    "message_id": "msg_01",
    "conversation_id": "conv_01",
    "client_msg_id": "01J4WEB7Y6G7Y4V5S8K6Z9G8T1",
    "message_seq": 1024,
    "created_at": "2026-06-20T04:00:00Z"
  }
}
```

### 6.3 message.new

```json
{
  "type": "message.new",
  "seq": 202,
  "payload": {
    "message": {
      "message_id": "msg_02",
      "conversation_id": "conv_01",
      "seq": 1025,
      "sender_id": 1002,
      "content_type": "text",
      "content": {
        "text": "world"
      },
      "created_at": "2026-06-20T04:00:01Z"
    }
  }
}
```

### 6.4 message.ack

Web 客户端在消息写入 IndexedDB 后发送 delivered ack；用户实际打开会话并看到消息后发送 read ack。

```json
{
  "type": "message.ack",
  "seq": 203,
  "payload": {
    "conversation_id": "conv_01",
    "ack_type": "delivered",
    "seqs": [1025]
  }
}
```

当前实现支持按明确 `seqs` 批量上报 delivered/read ack。Message 只会为同时满足以下条件的 seq 写入 receipt：

- 消息真实存在于当前 `conversation_id`。
- 当前用户拥有 Conversation 读权限。
- seq 落在成员 `visible_from_seq` / `visible_to_seq` 可见范围内。

`AckMessages` 返回的 `updated` 是实际成功写入或幂等更新的 receipt 数量。Conversation cursor 只使用“实际成功 ack 的最大 seq”推进：read ack 同时推进 `last_read_seq` 和 `last_delivered_seq`，delivered ack 只推进 `last_delivered_seq`。不存在、不可见或越界的 seq 不会推进 unread cursor；全部无效时返回 `accepted=true, updated=0`。

```json
{
  "type": "message.ack",
  "seq": 204,
  "payload": {
    "conversation_id": "conv_01",
    "ack_type": "read",
    "seqs": [1024, 1025]
  }
}
```

---

## 7. Web 客户端同步协议

### 7.1 本地状态

Web 客户端必须使用 IndexedDB 保存可恢复状态。

| 数据 | 存储 | 说明 |
|------|------|------|
| `device_id` | IndexedDB 或 localStorage | 浏览器 profile 级设备 ID，登出不必删除 |
| `session_id` | IndexedDB | 最近一次 Edge session ID，用于重连恢复 |
| `conversation_cursor` | IndexedDB | `conversation_id -> last_contiguous_seq` |
| `messages` | IndexedDB | 最近消息缓存，用于首屏和离线展示 |
| `pending_outbox` | IndexedDB | 未持久化或未确认的本地发送消息 |

`last_contiguous_seq` 只在本地已经连续保存所有小于等于该序号的消息后推进。收到更大的 `seq` 时不能直接跳跃推进。

### 7.2 多标签页模型

同一浏览器 profile 只允许一个 leader tab 维护 WebSocket：

1. 使用 Web Locks API 竞争 `beehive-im-ws-leader`。
2. leader tab 建立 WebSocket、接收 push、写 IndexedDB。
3. leader tab 通过 `BroadcastChannel("beehive-im")` 通知其他 tab 更新 UI。
4. follower tab 发送消息时写入 IndexedDB `pending_outbox`，再通过 BroadcastChannel 请求 leader 发送。
5. leader tab 关闭或失去锁后，其他 tab 重新竞选。

如果浏览器不支持 Web Locks API，降级为 localStorage lease，但 lease 必须有短 TTL，避免异常关闭后长期无 leader。

### 7.3 首次加载

```mermaid
sequenceDiagram
    participant W as Web Client
    participant API as Edge/API Gateway
    participant M as Message

    W->>W: load IndexedDB cursors
    W->>API: POST /v1/messages/sync cursors
    API->>M: SyncMessages
    M-->>API: missing messages + latest cursors
    API-->>W: sync result
    W->>W: write messages and advance cursors
```

首屏可以先展示 IndexedDB 缓存，再异步同步服务端缺失消息。同步接口必须支持分页，单次返回上限默认 100 到 500 条。

### 7.4 在线 push 处理

收到 `message.new` 时：

1. 读取本地 `last_contiguous_seq`。
2. 如果 `message.seq <= last_contiguous_seq`，视为重复 push，直接丢弃或刷新消息状态。
3. 如果 `message.seq == last_contiguous_seq + 1`，写入 IndexedDB 并推进 cursor。
4. 如果 `message.seq > last_contiguous_seq + 1`，先调用 `SyncMessages(after_seq=last_contiguous_seq)` 补齐缺口，再应用当前 push。
5. 写入成功后发送 delivered ack。

### 7.5 断线重连

```mermaid
sequenceDiagram
    participant W as Web Client
    participant API as Edge/API Gateway
    participant E as Edge
    participant M as Message

    W--xE: WebSocket disconnected
    W->>W: backoff with jitter
    W->>API: refresh token if needed
    W->>API: POST /v1/ws/ticket
    API-->>W: one-time ticket
    W->>E: WSS /ws?ticket=...
    E-->>W: session.connected
    W->>API: POST /v1/messages/sync cursors
    API->>M: SyncMessages
    M-->>API: missing messages
    API-->>W: sync result
```

重连策略：

| 项 | 要求 |
|----|------|
| 退避 | `1s -> 2s -> 4s -> 8s -> 16s -> 30s`，带随机 jitter |
| ticket | 每次重连重新申请，不复用旧 ticket |
| token | access token 过期时先刷新，再申请 ticket |
| session | 尽量携带旧 `session_id`，但不能依赖它保证消息完整 |
| sync | WebSocket 连接成功后必须按本地 cursor 执行补偿同步 |

### 7.6 离线发送

Web 客户端断网或无 leader 时：

1. 生成 `client_msg_id`。
2. 把消息写入 IndexedDB `pending_outbox`，UI 显示 `pending`。
3. 网络恢复或 leader 恢复后按创建时间发送。
4. 服务端返回 `message.persisted` 后，把本地状态改为 `sent` 并记录 `message_id + seq`。
5. 超时或失败保留 `client_msg_id`，用户重试仍使用同一个 `client_msg_id`。

禁止在重试时生成新的 `client_msg_id`，否则会产生重复消息。

---

## 8. 错误码

| code | 场景 | Web 行为 |
|------|------|----------|
| `INVALID_ARGUMENT` | 内容为空、过长、类型不支持或必要字段缺失 | 标记失败，提示用户修改 |
| `CONVERSATION_NOT_FOUND` | 会话不存在或不可见 | 停止重试，刷新会话列表 |
| `PERMISSION_DENIED` | 非成员或成员状态不可用 | 停止重试，刷新权限状态 |
| `CONVERSATION_CLOSED` | 会话关闭 | 停止重试，刷新会话状态 |
| `MEMBER_MUTED` | 成员被禁言 | 停止重试，提示禁言状态 |
| `duplicate=true` | 幂等命中，不作为错误码返回 | 使用返回的已持久化消息更新本地状态 |
| `MESSAGE_SERVICE_UNAVAILABLE` | Gateway 未配置或无法访问 Message | 保持本地 pending，稍后重试 |
| `MESSAGE_SEND_FAILED` / `MESSAGE_ACK_FAILED` | Message RPC 故障 | 保持本地 pending 或稍后重试 ack |
| `MESSAGE_GAP_DETECTED` | 服务端检测 cursor 缺口 | 触发 `SyncMessages` |
| `WS_TICKET_EXPIRED` | ticket 过期或已使用 | 重新申请 ticket |
| `TOKEN_EXPIRED` | access token 过期 | 刷新 token 后重新申请 ticket |
| `RATE_LIMITED` | 发送或同步过快 | 按 `retry_after` 延迟 |

---

## 9. 安全要求

| 项 | 要求 |
|----|------|
| WebSocket 鉴权 | 浏览器使用一次性 `ws_ticket`；JWT/access token 禁止进入 WS query |
| Origin | Edge 必须校验 Origin 白名单 |
| XSS 风险 | refresh token 必须使用 HttpOnly Cookie；生产环境必须 Secure，access token 尽量只保存在内存 |
| 内容校验 | Message 服务按 `content_type` 校验大小、字段和危险内容 |
| 权限 | 每次发送、同步、ack 都必须校验用户会话成员关系 |
| 日志 | 日志使用英文，禁止记录 token、ticket、完整消息内容和隐私字段 |
| 限流 | 按用户、设备、会话分别限制发送和同步 QPS |

---

## 10. 可观测性

| 指标 | 说明 |
|------|------|
| `message_send_latency_ms` | 发送到持久化完成延迟 |
| `message_send_idempotent_hit_total` | 幂等命中次数 |
| `message_sync_latency_ms` | 同步接口延迟 |
| `message_sync_gap_total` | 客户端缺口补齐次数 |
| `message_outbox_pending` | 待发布 outbox 数量 |
| `message_outbox_publish_failed_total` | outbox 发布失败次数 |
| `message_ack_latency_ms` | delivered/read ack 延迟 |

Web 客户端建议上报：

| 指标 | 说明 |
|------|------|
| `web_ws_reconnect_total` | WebSocket 重连次数 |
| `web_ws_leader_switch_total` | 多标签页 leader 切换次数 |
| `web_message_pending_count` | 本地 pending outbox 数量 |
| `web_message_gap_detected_total` | 本地检测到消息缺口次数 |

---

## 11. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| DB commit 成功但 MQ publish 失败 | 消息已存但无在线通知 | Outbox + publisher confirm + 客户端 sync 补偿 |
| Web 多标签页重复连接 | 同设备连接互踢或重复消息 | leader tab + BroadcastChannel |
| WebSocket push 丢失 | 客户端漏消息 | 按 `conversation_id + seq` 补偿同步 |
| 客户端重复发送 | 重复消息 | `sender_id + device_id + client_msg_id` 唯一 |
| 大会话同步压力 | DB 查询压力升高 | 分页、覆盖索引、限流和游标同步 |
| XSS 窃取 token | 账号被盗用 | HttpOnly refresh cookie、access token 内存化、CSP、输入消毒 |

---

## 12. 验收清单

- [x] Message 写入 `messages + outbox_events` 同事务。
- [x] `UNIQUE(conversation_id, seq)` 和 `UNIQUE(sender_id, device_id, client_msg_id)` 已落库。
- [x] 发送接口重复 `client_msg_id` 返回同一条已持久化消息。
- [x] WebSocket push 丢失时，Web 客户端可通过 Edge HTTP `SyncMessages` 补齐。
- [ ] Web 多标签页只保留一个 WebSocket leader。
- [x] WebSocket 鉴权使用一次性 `ws_ticket`，JWT/access token 不进入 WS query。
- [x] delivered/read ack 支持按 seqs 批量、幂等和权限校验。
- [x] `AckMessages(read)` 只按真实存在且用户可见的最大成功 ack seq 推进 Conversation `last_read_seq`，会话列表返回精确 unread count。
- [x] `ListMessages` / `SyncMessages` 已按成员 `visible_from_seq` / `visible_to_seq` 过滤。
- [ ] 指标覆盖发送延迟、同步缺口、outbox pending、重连次数和本地 pending 数。
