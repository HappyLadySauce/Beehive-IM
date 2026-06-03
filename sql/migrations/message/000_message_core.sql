CREATE TABLE conversations (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(20) NOT NULL DEFAULT 'direct',
    title VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE conversation_members (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT conversation_members_unique_member UNIQUE (conversation_id, user_id)
);

CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    message_id VARCHAR(64) NOT NULL UNIQUE,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    client_message_id VARCHAR(128) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'created',
    sequence BIGINT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT messages_unique_client_message UNIQUE (sender_user_id, client_message_id),
    CONSTRAINT messages_unique_conversation_sequence UNIQUE (conversation_id, sequence),
    CONSTRAINT messages_content_not_empty CHECK (length(btrim(content)) > 0)
);

CREATE TABLE message_deliveries (
    id BIGSERIAL PRIMARY KEY,
    message_id VARCHAR(64) NOT NULL REFERENCES messages(message_id) ON DELETE CASCADE,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    delivered_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT message_deliveries_unique_recipient UNIQUE (message_id, recipient_user_id)
);

CREATE INDEX idx_conversation_members_user_id ON conversation_members(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_messages_conversation_sequence ON messages(conversation_id, sequence DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_messages_sender_client ON messages(sender_user_id, client_message_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_message_deliveries_recipient_status ON message_deliveries(recipient_user_id, status) WHERE deleted_at IS NULL;

CREATE TRIGGER update_conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_conversation_members_updated_at
    BEFORE UPDATE ON conversation_members
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_messages_updated_at
    BEFORE UPDATE ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_message_deliveries_updated_at
    BEFORE UPDATE ON message_deliveries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
