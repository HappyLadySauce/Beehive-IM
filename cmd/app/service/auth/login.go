package auth

import (
	"strings"
	"context"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"

	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/passwd"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

func (s *AuthService) Login(ctx context.Context, req v1.LoginRequest) (v1.AuthResponse, error) {
	if ctx == nil {
		return v1.AuthResponse{}, fmt.Errorf("context is nil")
	}
	if s == nil || s.DB == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil {
		return v1.AuthResponse{}, fmt.Errorf("auth service is not fully initialized")
	}

	var user model.User
	var err error
	if AccountIsEmail(req.Account) {
		err = s.DB.WithContext(ctx).
		Where("email = ?", req.Account).
		First(&user).Error
	} else {
		err = s.DB.WithContext(ctx).
		Where("username = ?", req.Account).
		First(&user).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return v1.AuthResponse{}, ErrInvalidCredentials
		}
		return v1.AuthResponse{}, fmt.Errorf("find user: %w", err)
	}

	if !passwd.CheckPassword(req.Password, user.PasswordHash) {
		return v1.AuthResponse{}, ErrInvalidCredentials
	}

	return s.CreateSessionToken(ctx, strconv.FormatUint(uint64(user.ID), 10), user.Username, req.DeviceID, req.Platform)
}

func AccountIsEmail(account string) bool {
	if account == "" {
		return false
	}
	if strings.Contains(account, "@") {
		return true
	}
	return false
}

