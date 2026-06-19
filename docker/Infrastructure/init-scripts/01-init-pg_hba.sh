#!/bin/bash
set -e

cat > "$PGDATA/pg_hba.conf" << EOF
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections inside the container.
# 容器内本地连接。
local   all             all                                     trust

# IPv4 TCP connections require password authentication.
# IPv4 TCP 连接必须使用密码认证。
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             0.0.0.0/0               scram-sha-256

# IPv6 TCP connections require password authentication.
# IPv6 TCP 连接必须使用密码认证。
host    all             all             ::1/128                 scram-sha-256
host    all             all             ::0/0                   scram-sha-256
EOF

# Reload PostgreSQL configuration.
# 重新加载 PostgreSQL 配置。
pg_ctl reload -D "$PGDATA" || true
