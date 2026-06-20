package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GithubCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGithubCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GithubCallbackLogic {
	return &GithubCallbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Step2: client sends code from callback; server exchanges token and issues app session
func (l *GithubCallbackLogic) GithubCallback(in *pb.GithubCallbackRequest) (*pb.LoginResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.LoginResponse{}, nil
}
