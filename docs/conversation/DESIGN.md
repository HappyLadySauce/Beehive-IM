# Beehive-IM Conversation 服务设计文档

> 版本：v1.0
> 适用范围：会话、成员、权限、会话设置、通知偏好、消息序号分配协作
> 关联文件：[`docs/message/DESIGN.md`](../message/DESIGN.md)、[`docs/notification/DESIGN.md`](../notification/DESIGN.md)、[`docs/gateway/DESIGN.md`](../gateway/DESIGN.md)、[`docs/infrastructure/DESIGN.md`](../infrastructure/DESIGN.md)

---

## 1. 目标与范围

Conversation 服务是 IM 系统的会话事实边界，负责维护会话、成员、权限和会话级设置。Message 服务负责消息事实；Presence 服务负责在线态；Notification 服务负责通知编排。Conversation 必须为 Message 和 Notification 提供一致的成员与权限判断，避免这些规则散落在多个服务里。

### 1.1 职责

| 职责 | 说明 |
|------|------|
| 会话生命周期 | 创建单聊、群聊、系统会话，维护会话状态 |
| 成员管理 | 加入、移除、角色变更、禁言、退出 |
| 权限判断 | 校验用户是否可发消息、拉取消息、管理成员 |
| 会话设置 | 置顶、免打扰、昵称、消息提醒策略 |
| 序号协作 | 为 Message 提供会话级序号分配或序号约束 |
| 事件发布 | 发布 `conversation.updated.{conversation_id}` 等领域事件 |

### 1.2 非职责

- 不保存消息正文。
- 不管理 WebSocket 连接和在线态。
- 不直接调用离线推送 provider。
- 不负责用户基础资料事实，用户资料仍属于 User 服务。

---

## 2. 总体架构

```mermaid
flowchart LR
    Client[Client]
    Gateway[Gateway]
    Conversation[Conversation Service]
    Message[Message Service]
    Notification[Notification Service]
    User[User Service]
    PG[(PostgreSQL)]
    Etcd[(etcd)]
    RMQ[(RabbitMQ)]

    Client --> Gateway
    Gateway --> Conversation
    Gateway --> Message
    Message -->|permission / next seq| Conversation
    Notification -->|recipients / preferences| Conversation
    Conversation --> User
    Conversation --> PG
    Conversation --> Etcd
    Conversation -->|conversation.updated| RMQ
```

---

## 3. API 契约

v1 推荐使用 gRPC。所有写接口必须带幂等 key 或业务唯一约束，所有读写必须带 deadline。

| RPC | 调用方 | 说明 |
|-----|--------|------|
| `CreateConversation` | Gateway / API | 创建单聊或群聊 |
| `GetConversation` | Gateway / Message / Notification | 查询会话基础信息 |
| `ListConversations` | Gateway / API | 查询用户会话列表 |
| `AddMembers` | Gateway / API | 添加成员 |
| `RemoveMembers` | Gateway / API | 移除成员 |
| `UpdateMemberRole` | Gateway / API | 变更成员角色 |
| `UpdateConversationSettings` | Gateway / API | 修改会话设置 |
| `CheckSendPermission` | Message | 校验发送权限 |
| `ResolveNotificationTargets` | Notification | 解析收件人和通知策略 |
| `AllocateMessageSeq` | Message | 分配会话内单调递增消息序号 |

### 3.1 权限语义

| 权限 | 判断依据 |
|------|----------|
| 发消息 | 成员存在、未被移除、未被禁言、会话未关闭 |
| 拉消息 | 成员存在，且查询范围不早于成员可见起点 |
| 管理成员 | 成员角色满足 owner/admin |
| 修改设置 | 用户级设置由本人修改；群级设置需管理员权限 |

---

## 4. 数据模型

### 4.1 PostgreSQL

| 表 | 说明 |
|----|------|
| `conversations` | 会话主表，保存类型、状态、当前消息序号 |
| `conversation_members` | 成员关系、角色、加入时间、可见起点 |
| `conversation_settings` | 用户级或会话级设置 |
| `conversation_events` | 可选审计事件表 |

`conversations` 建议字段：

| 字段 | 说明 |
|------|------|
| `id` | 会话 ID |
| `type` | `direct`、`group`、`system` |
| `status` | `active`、`archived`、`closed` |
| `current_seq` | 会话内最新消息序号 |
| `created_by` | 创建者 |
| `created_at` / `updated_at` | 生命周期时间 |

`conversation_members` 建议字段：

| 字段 | 说明 |
|------|------|
| `conversation_id` | 会话 ID |
| `user_id` | 用户 ID |
| `role` | `owner`、`admin`、`member` |
| `status` | `active`、`muted`、`removed` |
| `joined_at` | 加入时间 |
| `visible_from_seq` | 成员可见消息起点 |

### 4.2 序号分配

会话序号必须单调递增，建议二选一：

| 方案 | 说明 |
|------|------|
| Conversation 分配 | Message 写入前调用 `AllocateMessageSeq`，由 Conversation 原子递增 `current_seq` |
| Message 分配并回写 | Message 在同一事务内使用会话行锁或唯一索引分配序号，再发布事件 |

v1 推荐 Conversation 提供 `AllocateMessageSeq`，便于把会话级并发控制集中到一个边界。Message 写入仍必须以 PostgreSQL 唯一索引 `conversation_id + seq` 兜底。

---

## 5. 事件

| Routing key | 触发场景 | 消费方 |
|-------------|----------|--------|
| `conversation.created.{conversation_id}` | 创建会话 | Notification、Audit、Search |
| `conversation.updated.{conversation_id}` | 名称、头像、设置、成员变更 | Notification、Client sync |
| `conversation.member_changed.{conversation_id}` | 成员加入、移除、角色变更 | Notification、Audit |

事件 payload 必须包含 `event_id`、`conversation_id`、`operator_id`、`occurred_at` 和变更摘要。敏感字段不得进入日志或无权限消费者。

---

## 6. 与其他服务协作

| 服务 | 协作方式 |
|------|----------|
| Gateway | 转发客户端会话管理请求；不直接修改数据库 |
| Message | 发送前校验权限，获取或校验会话序号 |
| Notification | 解析收件人、免打扰、离线通知策略 |
| User | 查询用户基础资料快照，不拥有用户事实 |
| Presence | 无直接依赖；在线态由 Notification 查询 Presence |

---

## 7. 安全与并发

| 项 | 要求 |
|----|------|
| 权限 | 所有写操作必须校验操作者成员身份和角色 |
| 幂等 | 创建会话、添加成员、设置修改必须支持幂等 |
| 并发 | 成员变更和消息序号分配必须使用事务、行锁或唯一索引兜底 |
| 隐私 | 非成员不得查询会话详情和成员列表 |
| 日志 | 英文结构化日志，禁止输出完整手机号、邮箱、token 和 secret |

---

## 8. 可观测性

| 指标 | 说明 |
|------|------|
| `conversation_create_latency_ms` | 创建会话延迟 |
| `conversation_permission_check_latency_ms` | 权限校验延迟 |
| `conversation_seq_allocate_latency_ms` | 序号分配延迟 |
| `conversation_member_change_total` | 成员变更次数 |
| `conversation_event_publish_failed_total` | 事件发布失败次数 |

---

## 9. 验收清单

- [ ] Message 发送前通过 Conversation 校验成员权限。
- [ ] 会话序号具备单调递增保证，并有唯一索引兜底。
- [ ] Notification 不自行维护成员和免打扰规则，统一调用 Conversation。
- [ ] 成员变更发布 `conversation.updated.#` 或等价领域事件。
- [ ] 会话读写接口具备 deadline、幂等、权限校验和结构化日志。
