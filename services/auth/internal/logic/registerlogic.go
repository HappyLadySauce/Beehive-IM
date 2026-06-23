package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// --- Local account / 本地账号登录 ---
func (l *RegisterLogic) Register(in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	username, email, phone, password, err := validateRegisterInput(in)
	if err != nil {
		return nil, err
	}

	passwordHash, err := bcryptHash(password)
	if err != nil {
		l.Errorf("password hash failed: username=%s error=%v", username, err)
		return nil, authStatusError(err)
	}
	user, err := l.svcCtx.Auth.CreateLocalUser(l.ctx, username, email, phone, passwordHash)
	if err != nil {
		if err == repository.ErrAccountExists {
			l.Infof("register rejected: username=%s reason=account_exists", username)
		} else {
			l.Errorf("register failed: username=%s error=%v", username, err)
		}
		return nil, authStatusError(err)
	}
	login, err := issueLoginResponse(l.ctx, l.svcCtx, user)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterResponse{
		AccessToken:  login.GetAccessToken(),
		RefreshToken: login.GetRefreshToken(),
		ExpiresIn:    login.GetExpiresIn(),
		TokenType:    login.GetTokenType(),
		User:         login.GetUser(),
	}, nil
}
