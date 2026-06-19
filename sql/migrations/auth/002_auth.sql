-- OAuth provider bindings for local users.
-- 本地用户与第三方 OAuth 账号的绑定关系。

CREATE TABLE IF NOT EXISTS user_oauth_identities (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         VARCHAR(32) NOT NULL,
    provider_user_id VARCHAR(64) NOT NULL,
    provider_login   VARCHAR(255) NOT NULL,
    provider_email   VARCHAR(255),
    avatar_url       VARCHAR(512),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_oauth_identities_user_id
    ON user_oauth_identities (user_id);

COMMENT ON TABLE user_oauth_identities IS '用户 OAuth 绑定表';
COMMENT ON COLUMN user_oauth_identities.provider IS 'OAuth 提供方，如 github';
COMMENT ON COLUMN user_oauth_identities.provider_user_id IS '第三方用户 ID';
COMMENT ON COLUMN user_oauth_identities.provider_login IS '第三方登录名';
COMMENT ON COLUMN user_oauth_identities.provider_email IS '第三方邮箱';
COMMENT ON COLUMN user_oauth_identities.avatar_url IS '第三方头像 URL';

-- Short-lived OAuth authorization state for CSRF protection.
-- OAuth 授权流程短期 state，用于 CSRF 防护。

CREATE TABLE IF NOT EXISTS oauth_states (
    state        VARCHAR(64) PRIMARY KEY,
    redirect_uri TEXT NOT NULL,
    scope        TEXT NOT NULL DEFAULT '',
    intent       VARCHAR(16) NOT NULL DEFAULT 'login',
    user_id      BIGINT REFERENCES users (id) ON DELETE CASCADE,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_states_expires_at
    ON oauth_states (expires_at);

COMMENT ON TABLE oauth_states IS 'OAuth 授权 state 表';
COMMENT ON COLUMN oauth_states.intent IS '授权意图：login 或 link';
COMMENT ON COLUMN oauth_states.user_id IS 'link 流程下绑定的本地用户 ID';

-- Refresh token persistence (hash only, never store plaintext).
-- 刷新令牌持久化（仅存哈希，不落明文）。

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  CHAR(64) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
    ON refresh_tokens (user_id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
    ON refresh_tokens (expires_at)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE refresh_tokens IS '刷新令牌表';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'SHA-256 十六进制哈希';
