package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResumeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResumeLogic {
	return &ResumeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Resume rebuilds an upstream session after Edge rebind.
// Resume 在 Edge 切换上游后重建上游会话。
func (l *ResumeLogic) Resume(in *pb.ResumeRequest) (*pb.ResumeResponse, error) {
	if l.svcCtx.IsDraining() {
		l.Infof("gateway resume rejected: session_id=%s conn_id=%s edge_id=%s code=%s", in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), session.CodeGatewayDraining)
		return &pb.ResumeResponse{
			Accepted:  false,
			GatewayId: l.svcCtx.Sessions.GatewayID(),
			ErrorCode: session.CodeGatewayDraining,
			Message:   "gateway is draining",
		}, nil
	}

	sess, err := l.svcCtx.Sessions.Resume(in)
	if err != nil {
		l.Errorf("gateway resume rejected: session_id=%s conn_id=%s edge_id=%s code=%s", in.GetSessionId(), in.GetConnId(), in.GetEdgeId(), session.CodeForError(err))
		return &pb.ResumeResponse{
			Accepted:  false,
			GatewayId: l.svcCtx.Sessions.GatewayID(),
			ErrorCode: session.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	return &pb.ResumeResponse{
		Accepted:         true,
		GatewayId:        l.svcCtx.Sessions.GatewayID(),
		LastDeliveredSeq: sess.LastDeliveredSeq,
		Message:          "resumed",
	}, nil
}
