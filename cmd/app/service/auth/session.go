package auth

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/session"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func (s *AuthService) CreateSessionToken(ctx context.Context, userID, username, deviceID, platform string) (v1.AuthResponse, error) {
	if s == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil {
		return v1.AuthResponse{}, fmt.Errorf("auth service is not fully initialized")
	}

	sessionID, err := session.GenerateSessionID()
	if err != nil {
		return v1.AuthResponse{}, fmt.Errorf("generate session id: %w", err)
	}

	refreshToken, err := jwt.GenerateRefreshToken()
	if err != nil {
		return v1.AuthResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}

	expiresAt := time.Now().Add(s.Config.JWT.AccessTTL)
	token, err := jwt.GenerateToken(
		jwt.TokenClaims{
			SessionID: sessionID,
			UserID:    userID,
			Username:  username,
			DeviceID:  deviceID,
			Platform:  platform,
		},
		s.Config.JWT.Issuer,
		s.Config.JWT.Secret,
		jwtv5.NewNumericDate(expiresAt),
	)
	if err != nil {
		return v1.AuthResponse{}, fmt.Errorf("generate access token: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.Config.Cache.CommandTimeout)
	defer cancel()

	if err := s.storeSession(ctx, sessionID, refreshToken, SessionRecord{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		Platform: platform,
	}); err != nil {
		return v1.AuthResponse{}, err
	}

	return v1.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
		ExpiresAt:    expiresAt,
	}, nil
}
