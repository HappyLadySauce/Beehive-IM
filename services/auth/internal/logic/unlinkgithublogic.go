package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnlinkGitHubLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnlinkGitHubLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlinkGitHubLogic {
	return &UnlinkGitHubLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Unbind GitHub from current user
func (l *UnlinkGitHubLogic) UnlinkGitHub(in *pb.UnlinkGitHubRequest) (*pb.UnlinkGitHubResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.UnlinkGitHubResponse{}, nil
}
