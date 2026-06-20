package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RebindGatewayLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRebindGatewayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RebindGatewayLogic {
	return &RebindGatewayLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RebindGateway updates the current upstream gateway route.
// RebindGateway 更新当前上游 Gateway 路由。
func (l *RebindGatewayLogic) RebindGateway(in *pb.RebindGatewayRequest) (*pb.RebindGatewayResponse, error) {
	if err := l.svcCtx.Store.RebindGateway(l.ctx, in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), in.GetGatewayId(), ttlFromSeconds(in.GetTtlSeconds())); err != nil {
		l.Errorf("presence gateway rebind rejected: session_id=%s conn_id=%s gateway_id=%s code=%s", in.GetSessionId(), in.GetConnId(), in.GetGatewayId(), store.CodeForError(err))
		return &pb.RebindGatewayResponse{
			Rebound:   false,
			ErrorCode: store.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	return &pb.RebindGatewayResponse{Rebound: true, Message: "rebound"}, nil
}
