-- Conversation domain tables.
-- 会话领域表。

CREATE TABLE IF NOT EXISTS conversations (
    conversation_id VARCHAR(64) PRIMARY KEY,
    type            VARCHAR(20) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    title           VARCHAR(120) NOT NULL DEFAULT '',
    owner_user_id   VARCHAR(64) NOT NULL,
    current_seq     BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT chk_conversations_type CHECK (type IN ('direct', 'group')),
    CONSTRAINT chk_conversations_status CHECK (status IN ('active', 'closed')),
    CONSTRAINT chk_conversations_current_seq CHECK (current_seq >= 0)
);

CREATE INDEX IF NOT EXISTS idx_conversations_owner_user_id
    ON conversations (owner_user_id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trigger_conversations_updated_at
BEFORE UPDATE ON conversations
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE conversations IS '会话事实表';
COMMENT ON COLUMN conversations.conversation_id IS '会话 ID';
COMMENT ON COLUMN conversations.type IS '会话类型：direct/group';
COMMENT ON COLUMN conversations.status IS '会话状态：active/closed';
COMMENT ON COLUMN conversations.current_seq IS '会话内最新消息序列号';

-- Conversation member facts.
-- 会话成员事实。

CREATE TABLE IF NOT EXISTS conversation_members (
    conversation_id VARCHAR(64) NOT NULL REFERENCES conversations (conversation_id) ON DELETE CASCADE,
    user_id         VARCHAR(64) NOT NULL,
    role            VARCHAR(20) NOT NULL DEFAULT 'member',
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    muted_until     TIMESTAMPTZ,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id),
    CONSTRAINT chk_conversation_members_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT chk_conversation_members_status CHECK (status IN ('active', 'removed'))
);

CREATE INDEX IF NOT EXISTS idx_conversation_members_user_active
    ON conversation_members (user_id, conversation_id)
    WHERE status = 'active';

CREATE TRIGGER trigger_conversation_members_updated_at
BEFORE UPDATE ON conversation_members
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE conversation_members IS '会话成员表';
COMMENT ON COLUMN conversation_members.role IS '成员角色：owner/admin/member';
COMMENT ON COLUMN conversation_members.status IS '成员状态：active/removed';
COMMENT ON COLUMN conversation_members.muted_until IS '禁言截止时间，NULL 表示未禁言';

-- Per-user conversation settings.
-- 用户级会话设置。

CREATE TABLE IF NOT EXISTS conversation_settings (
    conversation_id VARCHAR(64) NOT NULL REFERENCES conversations (conversation_id) ON DELETE CASCADE,
    user_id         VARCHAR(64) NOT NULL,
    pinned          BOOLEAN NOT NULL DEFAULT FALSE,
    muted_until     TIMESTAMPTZ,
    remark          VARCHAR(120) NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_conversation_settings_user_id
    ON conversation_settings (user_id);

CREATE TRIGGER trigger_conversation_settings_updated_at
BEFORE UPDATE ON conversation_settings
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE conversation_settings IS '用户级会话设置表';
COMMENT ON COLUMN conversation_settings.pinned IS '是否置顶';
COMMENT ON COLUMN conversation_settings.muted_until IS '免打扰截止时间，NULL 表示未开启';
COMMENT ON COLUMN conversation_settings.remark IS '用户自定义会话备注';
