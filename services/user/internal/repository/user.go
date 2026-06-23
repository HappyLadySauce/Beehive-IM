package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

// User is the read model returned by User service.
// User 是 User 服务返回的读取模型。
type User struct {
	ID        int64
	Name      string
	Email     string
	Phone     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserRepository reads user facts from PostgreSQL.
// UserRepository 从 PostgreSQL 读取用户事实。
type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (User, error) {
	if r == nil || r.db == nil {
		return User{}, errors.New("user repository is not initialized")
	}
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		return User{}, fmt.Errorf("invalid user id: %s", id)
	}

	var user User
	err = r.db.QueryRow(ctx, `
SELECT u.id,
       COALESCE(NULLIF(p.nickname, ''), u.username) AS name,
       COALESCE(u.email, '') AS email,
       COALESCE(u.phone, '') AS phone,
       u.created_at,
       u.updated_at
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) BatchGetByIDs(ctx context.Context, ids []string) ([]User, []string, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("user repository is not initialized")
	}
	orderedIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	missing := make([]string, 0)
	for _, raw := range ids {
		idText := strings.TrimSpace(raw)
		if idText == "" {
			continue
		}
		userID, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || userID <= 0 {
			missing = append(missing, idText)
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		orderedIDs = append(orderedIDs, userID)
	}
	if len(orderedIDs) == 0 {
		return nil, missing, nil
	}

	rows, err := r.db.Query(ctx, `
SELECT u.id,
       COALESCE(NULLIF(p.nickname, ''), u.username) AS name,
       COALESCE(u.email, '') AS email,
       COALESCE(u.phone, '') AS phone,
       u.created_at,
       u.updated_at
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE u.id = ANY($1::bigint[]) AND u.deleted_at IS NULL`, orderedIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	found := make(map[int64]User, len(orderedIDs))
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Phone, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan user: %w", err)
		}
		found[user.ID] = user
	}
	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan users: %w", err)
	}

	users := make([]User, 0, len(found))
	for _, id := range orderedIDs {
		user, ok := found[id]
		if !ok {
			missing = append(missing, strconv.FormatInt(id, 10))
			continue
		}
		users = append(users, user)
	}
	return users, missing, nil
}
