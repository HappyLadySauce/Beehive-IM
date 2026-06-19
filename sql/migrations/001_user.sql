-- Users table: supports local registration and OAuth-only accounts.
-- 用户表：支持本地注册与纯 OAuth 账号。

CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(50) NOT NULL,
    email           VARCHAR(255),
    phone           VARCHAR(20),
    password_hash   VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username
    ON users (username)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email
    ON users (email)
    WHERE deleted_at IS NULL AND email IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone
    ON users (phone)
    WHERE deleted_at IS NULL AND phone IS NOT NULL;

COMMENT ON TABLE users IS '用户信息表';
COMMENT ON COLUMN users.id IS '用户ID';
COMMENT ON COLUMN users.username IS '用户名';
COMMENT ON COLUMN users.email IS '邮箱';
COMMENT ON COLUMN users.phone IS '手机号';
COMMENT ON COLUMN users.password_hash IS '密码哈希';
COMMENT ON COLUMN users.created_at IS '创建时间';
COMMENT ON COLUMN users.updated_at IS '更新时间';
COMMENT ON COLUMN users.deleted_at IS '删除时间';

-- User profile extension table.
-- 用户详情扩展表。

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id         BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    avatar          VARCHAR(255),
    nickname        VARCHAR(50),
    gender          SMALLINT,
    birthday        DATE,
    address         VARCHAR(255),
    city            VARCHAR(50),
    province        VARCHAR(50),
    country         VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

COMMENT ON TABLE user_profiles IS '用户详情表';
COMMENT ON COLUMN user_profiles.user_id IS '用户ID';
COMMENT ON COLUMN user_profiles.avatar IS '头像';
COMMENT ON COLUMN user_profiles.nickname IS '昵称';
COMMENT ON COLUMN user_profiles.gender IS '性别';
COMMENT ON COLUMN user_profiles.birthday IS '生日';
COMMENT ON COLUMN user_profiles.address IS '地址';
COMMENT ON COLUMN user_profiles.city IS '城市';
COMMENT ON COLUMN user_profiles.province IS '省份';
COMMENT ON COLUMN user_profiles.country IS '国家';
COMMENT ON COLUMN user_profiles.created_at IS '创建时间';
COMMENT ON COLUMN user_profiles.updated_at IS '更新时间';
COMMENT ON COLUMN user_profiles.deleted_at IS '删除时间';
