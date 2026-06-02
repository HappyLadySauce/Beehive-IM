package auth

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	jwtv5 "github.com/golang-jwt/jwt/v5"

)

func (s *AuthService) RefreshSessionToken(ctx context.Context, req v1.RefreshRequest) (v1.AuthResponse, error) {
	if ctx == nil {
		return v1.AuthResponse{}, fmt.Errorf("context is nil")
	}
	if s == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil || s.Config.Cache == nil {
		return v1.AuthResponse{}, fmt.Errorf("auth service is not fully initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, s.Config.Cache.CommandTimeout)
	defer cancel()

	record, err := s.getSession(ctx, req.SessionID)
	if err != nil {
		return v1.AuthResponse{}, err
	}
	if record.RefreshHash != jwt.HashRefreshToken(req.RefreshToken) {
		return v1.AuthResponse{}, fmt.Errorf("invalid refresh token")
	}

	refreshToken, err := jwt.GenerateRefreshToken()
	if err != nil {
		return v1.AuthResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}

	expiresAt := time.Now().Add(s.Config.JWT.AccessTTL)
	token, err := jwt.GenerateToken(
		jwt.TokenClaims{
			SessionID: req.SessionID,
			UserID:    record.UserID,
			Username:  record.Username,
			DeviceID:  record.DeviceID,
			Platform:  record.Platform,
		},
		s.Config.JWT.Issuer,
		s.Config.JWT.Secret,
		jwtv5.NewNumericDate(expiresAt),
	)
	if err != nil {
		return v1.AuthResponse{}, fmt.Errorf("generate access token: %w", err)
	}

	record.RefreshHash = jwt.HashRefreshToken(refreshToken)
	if err := s.saveSession(ctx, req.SessionID, record); err != nil {
		return v1.AuthResponse{}, err
	}

	return v1.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		SessionID:    req.SessionID,
		ExpiresAt:    expiresAt,
	}, nil
}
