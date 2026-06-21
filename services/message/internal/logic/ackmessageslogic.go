package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AckMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAckMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AckMessagesLogic {
	return &AckMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AckMessages records delivered/read receipts for message sequences.
// AckMessages 记录消息序列的 delivered/read 回执。
func (l *AckMessagesLogic) AckMessages(in *pb.AckMessagesRequest) (*pb.AckMessagesResponse, error) {
	permission, err := l.svcCtx.Conversation.CheckReadPermission(l.ctx, &conversationservice.CheckReadPermissionRequest{
		ConversationId: in.GetConversationId(),
		UserId:         in.GetUserId(),
	})
	if err != nil {
		l.Errorf("check ack permission rpc failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	if !permission.GetAllowed() {
		return ackRejected(permission.GetErrorCode(), permission.GetMessage()), nil
	}

	updated, err := l.svcCtx.Messages.AckMessages(l.ctx, in.GetConversationId(), in.GetUserId(), in.GetAckType(), in.GetSeqs())
	if err != nil {
		if isBusinessError(err) {
			return ackRejected(repository.CodeForError(err), err.Error()), nil
		}
		l.Errorf("ack messages failed: conversation_id=%s user_id=%s ack_type=%s error=%v", in.GetConversationId(), in.GetUserId(), in.GetAckType(), err)
		return nil, err
	}
	return &pb.AckMessagesResponse{
		Accepted: true,
		Message:  "acknowledged",
		Updated:  updated,
	}, nil
}
