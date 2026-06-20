package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CleanupEdgeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCleanupEdgeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CleanupEdgeLogic {
	return &CleanupEdgeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CleanupEdge removes stale routes owned by one edge.
// CleanupEdge 清理某个 Edge 拥有的残留路由。
func (l *CleanupEdgeLogic) CleanupEdge(in *pb.CleanupEdgeRequest) (*pb.CleanupEdgeResponse, error) {
	removed, err := l.svcCtx.Store.CleanupEdge(l.ctx, in.GetEdgeId(), int(in.GetBatchSize()))
	if err != nil {
		l.Errorf("presence edge cleanup failed: edge_id=%s code=%s", in.GetEdgeId(), store.CodeForError(err))
		return nil, err
	}

	return &pb.CleanupEdgeResponse{Removed: int32(removed)}, nil
}
