package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LinkGitHubLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLinkGitHubLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LinkGitHubLogic {
	return &LinkGitHubLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Step 1 for logged-in user binding GitHub
func (l *LinkGitHubLogic) LinkGitHub(in *pb.LinkGitHubRequest) (*pb.RedirectToGithubAuthorizeUrlResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.RedirectToGithubAuthorizeUrlResponse{}, nil
}
