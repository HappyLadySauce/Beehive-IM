package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DismissConversationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDismissConversationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DismissConversationLogic {
	return &DismissConversationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DismissConversation closes a group conversation by owner.
func (l *DismissConversationLogic) DismissConversation(in *pb.DismissConversationRequest) (*pb.DismissConversationResponse, error) {
	if err := l.svcCtx.Conversations.DismissConversation(l.ctx, in.GetConversationId(), in.GetActorUserId()); err != nil {
		if isBusinessError(err) {
			return &pb.DismissConversationResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("dismiss conversation failed: conversation_id=%s actor_user_id=%s error=%v", in.GetConversationId(), in.GetActorUserId(), err)
		return nil, err
	}

	return &pb.DismissConversationResponse{Accepted: true, Message: "dismissed"}, nil
}
