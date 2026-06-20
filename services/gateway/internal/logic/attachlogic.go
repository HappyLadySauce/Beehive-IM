package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AttachLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAttachLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AttachLogic {
	return &AttachLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Attach creates an upstream session for an Edge connection.
// Attach 为 Edge 连接创建上游会话。
func (l *AttachLogic) Attach(in *pb.AttachRequest) (*pb.AttachResponse, error) {
	if l.svcCtx.IsDraining() {
		l.Infof("gateway attach rejected: session_id=%s conn_id=%s edge_id=%s code=%s", in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), session.CodeGatewayDraining)
		return &pb.AttachResponse{
			Accepted:  false,
			GatewayId: l.svcCtx.Sessions.GatewayID(),
			ErrorCode: session.CodeGatewayDraining,
			Message:   "gateway is draining",
		}, nil
	}

	if _, err := l.svcCtx.Sessions.Attach(in); err != nil {
		l.Errorf("gateway attach rejected: session_id=%s conn_id=%s edge_id=%s code=%s", in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), session.CodeForError(err))
		return &pb.AttachResponse{
			Accepted:  false,
			GatewayId: l.svcCtx.Sessions.GatewayID(),
			ErrorCode: session.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	return &pb.AttachResponse{
		Accepted:  true,
		GatewayId: l.svcCtx.Sessions.GatewayID(),
		Message:   "attached",
		ErrorCode: "",
	}, nil
}
