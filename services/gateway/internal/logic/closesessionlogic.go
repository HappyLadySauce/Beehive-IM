package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseSessionLogic {
	return &CloseSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CloseSession closes an upstream session owned by Edge.
// CloseSession 关闭 Edge 持有的上游会话。
func (l *CloseSessionLogic) CloseSession(in *pb.CloseSessionRequest) (*pb.CloseSessionResponse, error) {
	closed := l.svcCtx.Sessions.Close(in)
	if !closed {
		l.Infof("gateway close session ignored: session_id=%s conn_id=%s edge_id=%s reason=%s", in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), in.GetReason())
	}

	return &pb.CloseSessionResponse{Closed: closed}, nil
}
