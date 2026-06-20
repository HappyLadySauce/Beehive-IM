package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshConnectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshConnectionLogic {
	return &RefreshConnectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RefreshConnection refreshes online connection TTL.
// RefreshConnection 续期在线连接 TTL。
func (l *RefreshConnectionLogic) RefreshConnection(in *pb.RefreshConnectionRequest) (*pb.RefreshConnectionResponse, error) {
	if err := l.svcCtx.Store.Refresh(l.ctx, in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), ttlFromSeconds(in.GetTtlSeconds())); err != nil {
		l.Errorf("presence refresh rejected: session_id=%s conn_id=%s code=%s", in.GetSessionId(), in.GetConnId(), store.CodeForError(err))
		return &pb.RefreshConnectionResponse{
			Refreshed: false,
			ErrorCode: store.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	return &pb.RefreshConnectionResponse{Refreshed: true, Message: "refreshed"}, nil
}
