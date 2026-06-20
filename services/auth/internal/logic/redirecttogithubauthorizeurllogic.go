package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RedirectToGithubAuthorizeUrlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRedirectToGithubAuthorizeUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RedirectToGithubAuthorizeUrlLogic {
	return &RedirectToGithubAuthorizeUrlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// --- Github OAuth2 authorization code flow / Github 授权登录 ---
func (l *RedirectToGithubAuthorizeUrlLogic) RedirectToGithubAuthorizeUrl(in *pb.RedirectToGithubAuthorizeUrlRequest) (*pb.RedirectToGithubAuthorizeUrlResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.RedirectToGithubAuthorizeUrlResponse{}, nil
}
