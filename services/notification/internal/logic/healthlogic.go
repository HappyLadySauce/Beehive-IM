package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthLogic {
	return &HealthLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Health returns service liveness for operational checks.
// Health 返回服务存活状态，供运维探测使用。
func (l *HealthLogic) Health(in *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{Status: "ok"}, nil
}
