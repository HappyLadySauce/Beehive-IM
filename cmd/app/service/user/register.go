package user

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
	authsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/auth"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/passwd"
	"gorm.io/gorm"
)

var ErrUserAlreadyExists = errors.New("user already exists")

// Register handles user registration logic, including input validation, password hashing, and database storage.
// Register 处理用户注册逻辑，包括输入验证、密码哈希和数据库存储。
func (s *UserService) Register(ctx context.Context, req v1.RegisterRequest) (v1.AuthResponse, error) {
	if ctx == nil {
		return v1.AuthResponse{}, fmt.Errorf("context is nil")
	}
	if s == nil || s.DB == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil {
		return v1.AuthResponse{}, fmt.Errorf("user service is not fully initialized")
	}

	username := req.Username
	email := req.Email
	password := req.Password

	var existing model.User
	err := s.DB.WithContext(ctx).
		Where("username = ?", username).
		Or("email = ?", email).
		First(&existing).Error
	if err == nil {
		return v1.AuthResponse{}, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return v1.AuthResponse{}, fmt.Errorf("check existing user: %w", err)
	}

	passwordHash, err := passwd.HashPassword(password)
	if err != nil {
		return v1.AuthResponse{}, err
	}

	created := model.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       "active",
	}
	if err := s.DB.WithContext(ctx).Create(&created).Error; err != nil {
		return v1.AuthResponse{}, fmt.Errorf("create user: %w", err)
	}

	userID := strconv.FormatUint(uint64(created.ID), 10)
	authService := &authsvc.AuthService{
		Cache:  s.Cache,
		Config: s.Config,
	}
	return authService.CreateSessionToken(ctx, userID, username, req.DeviceID, req.Platform)
}

