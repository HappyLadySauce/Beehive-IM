package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertConnectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertConnectionLogic {
	return &UpsertConnectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpsertConnection registers or replaces an online connection.
// UpsertConnection 注册或覆盖一个在线连接。
func (l *UpsertConnectionLogic) UpsertConnection(in *pb.UpsertConnectionRequest) (*pb.UpsertConnectionResponse, error) {
	if err := l.svcCtx.Store.Upsert(l.ctx, storeConnection(in.GetConnection()), ttlFromSeconds(in.GetTtlSeconds())); err != nil {
		l.Errorf("presence upsert rejected: session_id=%s conn_id=%s code=%s", in.GetConnection().GetSessionId(), in.GetConnection().GetConnId(), store.CodeForError(err))
		return &pb.UpsertConnectionResponse{
			Accepted:  false,
			ErrorCode: store.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	return &pb.UpsertConnectionResponse{Accepted: true, Message: "upserted"}, nil
}
