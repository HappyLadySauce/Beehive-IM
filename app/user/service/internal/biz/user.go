package biz

import (
	"context"

	v1 "github.com/HappyLadySauce/Beehive-IM/api/user/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	// ErrUserNotFound is returned when a user does not exist.
	// ErrUserNotFound 在用户不存在时返回。
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// User is the user domain model.
// User 为用户领域模型。
type User struct {
	ID       string
	Username string
	Nickname string
	Avatar   string
}

// UserRepo defines user persistence operations.
// UserRepo 定义用户持久化接口。
type UserRepo interface {
	FindByID(context.Context, string) (*User, error)
}

// UserUsecase orchestrates user business logic.
// UserUsecase 编排用户业务逻辑。
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase creates a UserUsecase.
// NewUserUsecase 创建用户用例。
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

// GetUser returns a user by ID.
// GetUser 根据 ID 获取用户。
func (uc *UserUsecase) GetUser(ctx context.Context, id string) (*User, error) {
	if id == "" {
		return nil, ErrUserNotFound
	}
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}
