-- Conversation product read state and direct uniqueness.
-- 会话产品化读取状态与单聊唯一约束。

ALTER TABLE conversation_members
    ADD COLUMN IF NOT EXISTS visible_from_seq BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS visible_to_seq BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_read_seq BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_delivered_seq BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMPTZ;

ALTER TABLE conversation_members
    ADD CONSTRAINT chk_conversation_members_visible_from_seq CHECK (visible_from_seq >= 1),
    ADD CONSTRAINT chk_conversation_members_visible_to_seq CHECK (visible_to_seq >= 0),
    ADD CONSTRAINT chk_conversation_members_last_read_seq CHECK (last_read_seq >= 0),
    ADD CONSTRAINT chk_conversation_members_last_delivered_seq CHECK (last_delivered_seq >= 0);

CREATE INDEX IF NOT EXISTS idx_conversation_members_user_inbox
    ON conversation_members (user_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversation_members_visible_range
    ON conversation_members (conversation_id, user_id, visible_from_seq, visible_to_seq);

CREATE TABLE IF NOT EXISTS direct_conversations (
    conversation_id VARCHAR(64) PRIMARY KEY REFERENCES conversations (conversation_id) ON DELETE CASCADE,
    user_low        VARCHAR(64) NOT NULL,
    user_high       VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_direct_conversations_pair CHECK (user_low <> user_high)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_direct_conversations_pair
    ON direct_conversations (user_low, user_high);

COMMENT ON TABLE direct_conversations IS '单聊唯一映射表';
COMMENT ON COLUMN direct_conversations.user_low IS '按字典序排序后的较小用户 ID';
COMMENT ON COLUMN direct_conversations.user_high IS '按字典序排序后的较大用户 ID';
COMMENT ON COLUMN conversation_members.visible_from_seq IS '成员可见消息起始 seq，包含该 seq';
COMMENT ON COLUMN conversation_members.visible_to_seq IS '成员可见消息结束 seq，0 表示无上界';
COMMENT ON COLUMN conversation_members.last_read_seq IS '成员已读到的会话 seq';
COMMENT ON COLUMN conversation_members.last_delivered_seq IS '成员已送达到客户端的会话 seq';
