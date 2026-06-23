// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/authservice"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.LogoutRequest) (resp *types.EmptyResponse, err error) {
	if _, err := l.svcCtx.Auth.Logout(l.ctx, &authservice.LogoutRequest{RefreshToken: req.RefreshToken}); err != nil {
		l.Errorf("auth logout rpc failed: error=%v", err)
		return nil, err
	}

	return &types.EmptyResponse{Success: true}, nil
}
