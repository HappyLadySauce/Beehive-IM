package logic

import (
	"context"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	refreshToken := strings.TrimSpace(in.GetRefreshToken())
	if refreshToken == "" {
		return nil, authStatusError(repository.ErrInvalidRefreshToken)
	}
	if err := l.svcCtx.Auth.RevokeRefreshToken(l.ctx, repository.HashRefreshToken(refreshToken)); err != nil {
		l.Errorf("logout failed: error=%v", err)
		return nil, authStatusError(err)
	}

	return &pb.LogoutResponse{}, nil
}
