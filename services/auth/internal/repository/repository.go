package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolationCode = "23505"

var (
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrAccountExists       = errors.New("account already exists")
	ErrInvalidCredentials  = errors.New("invalid account or password")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// User is the Auth service account read model.
// User 是 Auth 服务账号读取模型。
type User struct {
	ID           int64
	Username     string
	Email        string
	Phone        string
	Avatar       string
	PasswordHash string
}

// Repository owns local auth persistence.
// Repository 管理本地认证持久化。
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateLocalUser inserts a local user and profile in one transaction.
// CreateLocalUser 在一个事务中创建本地用户和详情。
func (r *Repository) CreateLocalUser(ctx context.Context, username, email, phone, passwordHash string) (User, error) {
	if r == nil || r.db == nil {
		return User{}, errors.New("auth repository is not initialized")
	}
	username = normalize(username)
	email = normalize(email)
	phone = normalize(phone)
	passwordHash = strings.TrimSpace(passwordHash)
	if username == "" || passwordHash == "" {
		return User{}, fmt.Errorf("%w: username and password_hash are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin register transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
INSERT INTO users (username, email, phone, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING id, username, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(password_hash, '')`,
		username, nullableString(email), nullableString(phone), passwordHash,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Phone, &user.PasswordHash)
	if isUniqueViolation(err) {
		return User{}, ErrAccountExists
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err = tx.Exec(ctx, `
INSERT INTO user_profiles (user_id, nickname)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET nickname = EXCLUDED.nickname`, user.ID, user.Username); err != nil {
		return User{}, fmt.Errorf("insert user profile: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit register transaction: %w", err)
	}
	return user, nil
}

// FindLocalUserByAccount loads a local account by username, email, or phone.
// FindLocalUserByAccount 按用户名、邮箱或手机号读取本地账号。
func (r *Repository) FindLocalUserByAccount(ctx context.Context, account string) (User, error) {
	if r == nil || r.db == nil {
		return User{}, errors.New("auth repository is not initialized")
	}
	account = normalize(account)
	if account == "" {
		return User{}, fmt.Errorf("%w: account is required", ErrInvalidArgument)
	}
	user, err := scanUser(r.db.QueryRow(ctx, `
SELECT u.id,
       u.username,
       COALESCE(u.email, ''),
       COALESCE(u.phone, ''),
       COALESCE(p.avatar, ''),
       COALESCE(u.password_hash, '')
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE u.deleted_at IS NULL
  AND (u.username = $1 OR u.email = $1 OR u.phone = $1)`, account))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("query local account: %w", err)
	}
	if strings.TrimSpace(user.PasswordHash) == "" {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

// CreateRefreshToken persists one refresh token hash.
// CreateRefreshToken 持久化一个刷新令牌哈希。
func (r *Repository) CreateRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("auth repository is not initialized")
	}
	tokenHash = normalize(tokenHash)
	if userID <= 0 || tokenHash == "" || expiresAt.IsZero() {
		return fmt.Errorf("%w: user_id, token_hash and expires_at are required", ErrInvalidArgument)
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// RotateRefreshToken revokes the old token and stores the replacement atomically.
// RotateRefreshToken 原子撤销旧令牌并写入新令牌。
func (r *Repository) RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (User, error) {
	if r == nil || r.db == nil {
		return User{}, errors.New("auth repository is not initialized")
	}
	oldHash = normalize(oldHash)
	newHash = normalize(newHash)
	if oldHash == "" || newHash == "" || expiresAt.IsZero() {
		return User{}, fmt.Errorf("%w: refresh token hashes and expires_at are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID int64
	var expires time.Time
	var revoked pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
SELECT user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE`, oldHash).Scan(&userID, &expires, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return User{}, fmt.Errorf("lock refresh token: %w", err)
	}
	now := time.Now().UTC()
	if revoked.Valid || !expires.After(now) {
		return User{}, ErrInvalidRefreshToken
	}
	user, err := scanUser(tx.QueryRow(ctx, `
SELECT u.id,
       u.username,
       COALESCE(u.email, ''),
       COALESCE(u.phone, ''),
       COALESCE(p.avatar, ''),
       COALESCE(u.password_hash, '')
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return User{}, fmt.Errorf("query refresh user: %w", err)
	}
	if _, err = tx.Exec(ctx, `
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE token_hash = $1`, oldHash); err != nil {
		return User{}, fmt.Errorf("revoke old refresh token: %w", err)
	}
	if _, err = tx.Exec(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)`, user.ID, newHash, expiresAt.UTC()); err != nil {
		return User{}, fmt.Errorf("insert rotated refresh token: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit refresh transaction: %w", err)
	}
	return user, nil
}

// RevokeRefreshToken marks one refresh token as revoked.
// RevokeRefreshToken 将一个刷新令牌标记为撤销。
func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	if r == nil || r.db == nil {
		return errors.New("auth repository is not initialized")
	}
	tokenHash = normalize(tokenHash)
	if tokenHash == "" {
		return fmt.Errorf("%w: refresh token is required", ErrInvalidArgument)
	}
	_, err := r.db.Exec(ctx, `
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, NOW())
WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func scanUser(scanner interface{ Scan(...any) error }) (User, error) {
	var user User
	if err := scanner.Scan(&user.ID, &user.Username, &user.Email, &user.Phone, &user.Avatar, &user.PasswordHash); err != nil {
		return User{}, err
	}
	return user, nil
}

func normalize(value string) string {
	return strings.TrimSpace(value)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
