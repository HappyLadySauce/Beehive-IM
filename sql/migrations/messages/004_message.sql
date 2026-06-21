-- Message facts.
-- 消息事实表。

CREATE TABLE IF NOT EXISTS messages (
    message_id      VARCHAR(64) PRIMARY KEY,
    conversation_id VARCHAR(64) NOT NULL REFERENCES conversations (conversation_id) ON DELETE RESTRICT,
    seq             BIGINT NOT NULL,
    sender_id       VARCHAR(64) NOT NULL,
    device_id       VARCHAR(128) NOT NULL,
    client_msg_id   VARCHAR(128) NOT NULL,
    client_seq      BIGINT NOT NULL DEFAULT 0,
    content_type    VARCHAR(32) NOT NULL,
    content_json    JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT chk_messages_seq CHECK (seq > 0),
    CONSTRAINT chk_messages_content_type CHECK (content_type IN ('text'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_messages_conversation_seq
    ON messages (conversation_id, seq);

CREATE UNIQUE INDEX IF NOT EXISTS uk_messages_sender_device_client_msg
    ON messages (sender_id, device_id, client_msg_id);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
    ON messages (conversation_id, created_at);

COMMENT ON TABLE messages IS '消息事实表';
COMMENT ON COLUMN messages.seq IS '会话内单调递增消息序列号';
COMMENT ON COLUMN messages.client_msg_id IS '客户端生成的幂等消息 ID';
COMMENT ON COLUMN messages.content_json IS '消息内容 JSON，不在日志中输出原文';

-- Message delivery/read receipts.
-- 消息送达/已读回执。

CREATE TABLE IF NOT EXISTS message_receipts (
    conversation_id VARCHAR(64) NOT NULL,
    user_id         VARCHAR(64) NOT NULL,
    message_seq     BIGINT NOT NULL,
    delivered_at    TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id, message_seq),
    CONSTRAINT fk_message_receipts_message
        FOREIGN KEY (conversation_id, message_seq)
        REFERENCES messages (conversation_id, seq)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_message_receipts_user_updated
    ON message_receipts (user_id, updated_at);

CREATE TRIGGER trigger_message_receipts_updated_at
BEFORE UPDATE ON message_receipts
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE message_receipts IS '消息回执表';
COMMENT ON COLUMN message_receipts.delivered_at IS '送达时间';
COMMENT ON COLUMN message_receipts.read_at IS '已读时间';

-- Transactional outbox for domain events.
-- 领域事件事务 outbox。

CREATE TABLE IF NOT EXISTS outbox_events (
    event_id        VARCHAR(64) PRIMARY KEY,
    aggregate_type  VARCHAR(64) NOT NULL,
    aggregate_id    VARCHAR(64) NOT NULL,
    event_type      VARCHAR(64) NOT NULL,
    routing_key     VARCHAR(255) NOT NULL,
    payload_json    JSONB NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until    TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_outbox_events_status CHECK (status IN ('pending', 'published', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events (next_attempt_at, created_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id);

CREATE TRIGGER trigger_outbox_events_updated_at
BEFORE UPDATE ON outbox_events
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE outbox_events IS '事务 outbox 事件表';
COMMENT ON COLUMN outbox_events.routing_key IS 'RabbitMQ topic routing key';
COMMENT ON COLUMN outbox_events.locked_until IS 'dispatcher 短租约，避免多 worker 重复发布';
