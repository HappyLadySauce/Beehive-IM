package logic

import (
	"context"
	"strings"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/pkg/authjwt"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Refresh access token using refresh_token
func (l *RefreshTokenLogic) RefreshToken(in *pb.RefreshTokenRequest) (*pb.LoginResponse, error) {
	oldToken := strings.TrimSpace(in.GetRefreshToken())
	if oldToken == "" {
		return nil, authStatusError(repository.ErrInvalidRefreshToken)
	}
	newToken, err := authjwt.RandomToken(32)
	if err != nil {
		l.Errorf("generate refresh token failed: error=%v", err)
		return nil, authStatusError(err)
	}
	user, err := l.svcCtx.Auth.RotateRefreshToken(
		l.ctx,
		repository.HashRefreshToken(oldToken),
		repository.HashRefreshToken(newToken),
		time.Now().UTC().Add(refreshTTL(l.svcCtx)),
	)
	if err != nil {
		l.Infof("refresh token rejected: error=%v", err)
		return nil, authStatusError(err)
	}

	accessToken, expiresIn, err := l.svcCtx.JWT.Sign(userIDString(user), user.Username)
	if err != nil {
		l.Errorf("sign access token failed: user_id=%s error=%v", userIDString(user), err)
		return nil, authStatusError(err)
	}
	return loginResponse(accessToken, newToken, expiresIn, user), nil
}
