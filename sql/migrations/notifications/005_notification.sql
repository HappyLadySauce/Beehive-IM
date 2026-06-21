-- Notification online delivery records.
-- 通知在线投递记录表。

CREATE TABLE IF NOT EXISTS notification_deliveries (
    delivery_id     VARCHAR(64) PRIMARY KEY,
    event_id        VARCHAR(64) NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    user_id         VARCHAR(64) NOT NULL,
    device_id       VARCHAR(128) NOT NULL DEFAULT '',
    edge_id         VARCHAR(128) NOT NULL DEFAULT '',
    conn_id         VARCHAR(128) NOT NULL DEFAULT '',
    session_id      VARCHAR(128) NOT NULL DEFAULT '',
    status          VARCHAR(20) NOT NULL,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_notification_deliveries_status CHECK (status IN ('pushed', 'dropped', 'failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_notification_deliveries_event_conn
    ON notification_deliveries (event_id, conn_id);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_event
    ON notification_deliveries (event_id);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_user_created
    ON notification_deliveries (user_id, created_at DESC);

CREATE TRIGGER trigger_notification_deliveries_updated_at
BEFORE UPDATE ON notification_deliveries
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE notification_deliveries IS '通知在线投递记录表';
COMMENT ON COLUMN notification_deliveries.event_id IS '来源 outbox event ID';
COMMENT ON COLUMN notification_deliveries.conn_id IS 'Presence 返回的目标连接 ID';
