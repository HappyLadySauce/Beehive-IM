package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LeaveConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLeaveConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaveConversationLogic {
	return &LeaveConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LeaveConversation lets one member leave a group conversation.
func (l *LeaveConversationLogic) LeaveConversation(in *pb.LeaveConversationRequest) (*pb.LeaveConversationResponse, error) {
	if err := l.svcCtx.Conversations.LeaveConversation(l.ctx, in.GetConversationId(), in.GetActorUserId()); err != nil {
		if isBusinessError(err) {
			return &pb.LeaveConversationResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("leave conversation failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}

	return &pb.LeaveConversationResponse{Accepted: true, Message: "left"}, nil
}
