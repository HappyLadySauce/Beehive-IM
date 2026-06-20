package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveConnectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveConnectionLogic {
	return &RemoveConnectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RemoveConnection removes an online connection if ownership still matches.
// RemoveConnection 在归属仍匹配时移除在线连接。
func (l *RemoveConnectionLogic) RemoveConnection(in *pb.RemoveConnectionRequest) (*pb.RemoveConnectionResponse, error) {
	removed, err := l.svcCtx.Store.Remove(l.ctx, storeConnection(in.GetConnection()))
	if err != nil {
		l.Errorf("presence remove rejected: session_id=%s conn_id=%s code=%s", in.GetConnection().GetSessionId(), in.GetConnection().GetConnId(), store.CodeForError(err))
		return nil, err
	}

	return &pb.RemoveConnectionResponse{Removed: removed}, nil
}
