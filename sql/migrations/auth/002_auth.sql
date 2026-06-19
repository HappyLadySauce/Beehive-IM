-- OAuth provider bindings for local users.
-- 本地用户与第三方 OAuth 账号的绑定关系。

CREATE TABLE IF NOT EXISTS user_oauth_identities (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         VARCHAR(32) NOT NULL,
    provider_user_id VARCHAR(128) NOT NULL,
    provider_login   VARCHAR(255) NOT NULL,
    provider_email   VARCHAR(255),
    avatar_url       VARCHAR(512),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 同一第三方账号全局唯一
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_identities_provider_user_id
    ON user_oauth_identities (provider, provider_user_id);

-- 同一用户同一提供方仅允许一条绑定
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_identities_user_provider
    ON user_oauth_identities (user_id, provider);

-- 创建 updated_at 触发器
CREATE TRIGGER trigger_user_oauth_identities_updated_at
BEFORE UPDATE ON user_oauth_identities
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE user_oauth_identities IS '用户 OAuth 绑定表';
COMMENT ON COLUMN user_oauth_identities.id IS '绑定记录 ID';
COMMENT ON COLUMN user_oauth_identities.user_id IS '本地用户 ID';
COMMENT ON COLUMN user_oauth_identities.provider IS 'OAuth 提供方，如 github';
COMMENT ON COLUMN user_oauth_identities.provider_user_id IS '第三方用户 ID';
COMMENT ON COLUMN user_oauth_identities.provider_login IS '第三方登录名';
COMMENT ON COLUMN user_oauth_identities.provider_email IS '第三方邮箱';
COMMENT ON COLUMN user_oauth_identities.avatar_url IS '第三方头像 URL';
COMMENT ON COLUMN user_oauth_identities.created_at IS '创建时间';
COMMENT ON COLUMN user_oauth_identities.updated_at IS '更新时间';

-- Refresh token persistence (hash only, never store plaintext).
-- 刷新令牌持久化（仅存哈希，不落明文）。

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  CHAR(64) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- token 哈希全局唯一
CREATE UNIQUE INDEX IF NOT EXISTS uk_refresh_tokens_token_hash
    ON refresh_tokens (token_hash);

-- 按用户查询会话
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
    ON refresh_tokens (user_id);

-- 查询用户未撤销且未过期的 token
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_active
    ON refresh_tokens (user_id, expires_at)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE refresh_tokens IS '刷新令牌表';
COMMENT ON COLUMN refresh_tokens.id IS '令牌记录 ID';
COMMENT ON COLUMN refresh_tokens.user_id IS '本地用户 ID';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'SHA-256 十六进制哈希';
COMMENT ON COLUMN refresh_tokens.expires_at IS '过期时间';
COMMENT ON COLUMN refresh_tokens.revoked_at IS '撤销时间';
COMMENT ON COLUMN refresh_tokens.created_at IS '创建时间';
