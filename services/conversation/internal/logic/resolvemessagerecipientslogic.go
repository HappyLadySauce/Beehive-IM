package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveMessageRecipientsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveMessageRecipientsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveMessageRecipientsLogic {
	return &ResolveMessageRecipientsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveMessageRecipients returns active users that should receive a message event.
// ResolveMessageRecipients 返回应接收消息事件的活跃用户。
func (l *ResolveMessageRecipientsLogic) ResolveMessageRecipients(in *pb.ResolveMessageRecipientsRequest) (*pb.ResolveMessageRecipientsResponse, error) {
	recipients, err := l.svcCtx.Conversations.ResolveMessageRecipients(l.ctx, in.GetConversationId())
	if err != nil {
		if isBusinessError(err) {
			return &pb.ResolveMessageRecipientsResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("resolve message recipients failed: conversation_id=%s sender_id=%s error=%v", in.GetConversationId(), in.GetSenderId(), err)
		return nil, err
	}
	return &pb.ResolveMessageRecipientsResponse{
		Accepted: true,
		Message:  "resolved",
		UserIds:  recipients,
	}, nil
}
