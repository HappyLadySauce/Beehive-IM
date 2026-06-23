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

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.AuthTokenResponse, err error) {
	result, err := l.svcCtx.Auth.Login(l.ctx, &authservice.LoginRequest{
		Account:  req.Account,
		Password: req.Password,
	})
	if err != nil {
		l.Infof("auth login rejected: account=%s error=%v", req.Account, err)
		return nil, err
	}

	return edgeAuthTokenResponse(result), nil
}
