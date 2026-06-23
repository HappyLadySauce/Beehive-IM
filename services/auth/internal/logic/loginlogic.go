package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *pb.LoginRequest) (*pb.LoginResponse, error) {
	account, password, err := validateLoginInput(in)
	if err != nil {
		return nil, err
	}

	user, err := l.svcCtx.Auth.FindLocalUserByAccount(l.ctx, account)
	if err != nil {
		l.Infof("login rejected: account=%s", account)
		return nil, authStatusError(err)
	}
	if err := bcryptVerify(user.PasswordHash, password); err != nil {
		l.Infof("login rejected: account=%s", account)
		return nil, authStatusError(err)
	}
	return issueLoginResponse(l.ctx, l.svcCtx, user)
}
