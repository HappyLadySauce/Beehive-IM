CREATE SCHEMA IF NOT EXISTS user;

-- 创建用户表
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,                  -- 自增主键
    username VARCHAR(50) NOT NULL UNIQUE,      -- 用户名，唯一且非空
    email VARCHAR(255) NOT NULL UNIQUE,        -- 邮箱，唯一且非空
    password_hash VARCHAR(255) NOT NULL,       -- 存储加密后的密码
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 用户状态（active, inactive, banned）
    last_login_at TIMESTAMPTZ,                 -- 上次登录时间（带时区）
    created_at TIMESTAMPTZ DEFAULT NOW(),      -- 创建时间
    updated_at TIMESTAMPTZ DEFAULT NOW(),      -- 更新时间
    deleted_at TIMESTAMPTZ                     -- 删除时间（软删除）
);

-- 创建自动更新 updated_at 的触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为 users 表绑定触发器
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
