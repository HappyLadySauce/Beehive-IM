package logic

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMessagesLogic {
	return &ListMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMessages returns one conversation history page.
// ListMessages 返回单个会话的一页历史消息。
func (l *ListMessagesLogic) ListMessages(in *pb.ListMessagesRequest) (*pb.ListMessagesResponse, error) {
	permission, err := l.svcCtx.Conversation.CheckReadPermission(l.ctx, &conversationservice.CheckReadPermissionRequest{
		ConversationId: in.GetConversationId(),
		UserId:         in.GetUserId(),
	})
	if err != nil {
		l.Errorf("check list messages permission rpc failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	if !permission.GetAllowed() {
		return listRejected(permission.GetErrorCode(), permission.GetMessage()), nil
	}

	messages, latestSeq, err := l.svcCtx.Messages.ListMessages(
		l.ctx,
		in.GetConversationId(),
		in.GetAfterSeq(),
		in.GetBeforeSeq(),
		in.GetDirection(),
		in.GetLimit(),
		permission.GetVisibleFromSeq(),
		permission.GetVisibleToSeq(),
	)
	if err != nil {
		if isBusinessError(err) {
			return listRejected(repository.CodeForError(err), err.Error()), nil
		}
		l.Errorf("list messages failed: conversation_id=%s user_id=%s error=%v", in.GetConversationId(), in.GetUserId(), err)
		return nil, err
	}
	return &pb.ListMessagesResponse{
		Accepted:  true,
		Message:   "listed",
		Messages:  messageItemsPB(messages),
		LatestSeq: latestSeq,
	}, nil
}
