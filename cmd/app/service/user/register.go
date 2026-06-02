package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/passwd"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var ErrUserAlreadyExists = errors.New("user already exists")

// Register handles user registration logic, including input validation, password hashing, and database storage.
// Register 处理用户注册逻辑，包括输入验证、密码哈希和数据库存储。
func (s *UserService) Register(ctx context.Context, req v1.RegisterRequest) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	if s == nil || s.DB == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil {
		return "", fmt.Errorf("user service is not fully initialized")
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
		return "", ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("check existing user: %w", err)
	}

	passwordHash, err := passwd.HashPassword(password)
	if err != nil {
		return "", err
	}

	created := model.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       "active",
	}
	if err := s.DB.WithContext(ctx).Create(&created).Error; err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	userID := strconv.FormatUint(uint64(created.ID), 10)
	version, err := newTokenVersion()
	if err != nil {
		return "", err
	}

	tokenTTL := s.Config.JWT.AccessTTL
	if s.Config.JWT.RefreshTTL > tokenTTL {
		tokenTTL = s.Config.JWT.RefreshTTL
	}
	if tokenTTL <= 0 {
		tokenTTL = time.Hour
	}

	key := cache.UserTokenVersionPrefix + userID
	if err := s.Cache.Set(ctx, key, version, tokenTTL).Err(); err != nil {
		return "", fmt.Errorf("store user token version: %w", err)
	}

	expiresAt := jwtv5.NewNumericDate(time.Now().Add(s.Config.JWT.AccessTTL))
	token, err := jwt.GenerateToken(userID, username, version, s.Config.JWT.Issuer, s.Config.JWT.Secret, expiresAt)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func newTokenVersion() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate token version: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
