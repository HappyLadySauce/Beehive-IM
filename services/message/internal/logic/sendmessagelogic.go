package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendMessage validates permissions and persists one client message.
// SendMessage 校验权限并持久化一条客户端消息。
func (l *SendMessageLogic) SendMessage(in *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	contentType, contentJSON, err := normalizeContent(in.GetContentType(), in.GetContentJson())
	if err != nil {
		return &pb.SendMessageResponse{
			Accepted:  false,
			ErrorCode: repository.CodeForError(err),
			Message:   err.Error(),
		}, nil
	}

	existing, ok, err := l.svcCtx.Messages.FindByIdempotency(l.ctx, in.GetSenderId(), in.GetDeviceId(), in.GetClientMsgId())
	if err != nil {
		if isBusinessError(err) {
			return &pb.SendMessageResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("query idempotent message failed: sender_id=%s device_id=%s client_msg_id=%s error=%v", in.GetSenderId(), in.GetDeviceId(), in.GetClientMsgId(), err)
		return nil, err
	}
	if ok {
		return messageResponse(existing, true), nil
	}

	permission, err := l.svcCtx.Conversation.CheckSendPermission(l.ctx, &conversationservice.CheckSendPermissionRequest{
		ConversationId: in.GetConversationId(),
		UserId:         in.GetSenderId(),
	})
	if err != nil {
		l.Errorf("check send permission rpc failed: conversation_id=%s sender_id=%s error=%v", in.GetConversationId(), in.GetSenderId(), err)
		return nil, err
	}
	if !permission.GetAllowed() {
		return permissionDeniedFromConversation(permission), nil
	}

	seq, err := l.svcCtx.Conversation.AllocateMessageSeq(l.ctx, &conversationservice.AllocateMessageSeqRequest{
		ConversationId: in.GetConversationId(),
		UserId:         in.GetSenderId(),
	})
	if err != nil {
		l.Errorf("allocate message seq rpc failed: conversation_id=%s sender_id=%s error=%v", in.GetConversationId(), in.GetSenderId(), err)
		return nil, err
	}
	if !seq.GetAccepted() {
		return sequenceRejectedFromConversation(seq), nil
	}

	save := repository.SaveMessageInput{
		ConversationID: in.GetConversationId(),
		Seq:            seq.GetSeq(),
		SenderID:       in.GetSenderId(),
		DeviceID:       in.GetDeviceId(),
		ClientMsgID:    in.GetClientMsgId(),
		ClientSeq:      in.GetClientSeq(),
		ContentType:    contentType,
		ContentJSON:    contentJSON,
		RoutingKey:     fmt.Sprintf("%s%s", messageCreatedPrefix, in.GetConversationId()),
	}
	save.OutboxPayload, err = buildOutboxPayload(save)
	if err != nil {
		l.Errorf("build message outbox payload failed: conversation_id=%s sender_id=%s error=%v", in.GetConversationId(), in.GetSenderId(), err)
		return nil, err
	}
	if !json.Valid(save.OutboxPayload) {
		return nil, errors.New("message outbox payload is invalid json")
	}

	msg, duplicate, err := l.svcCtx.Messages.SaveMessageWithOutbox(l.ctx, save)
	if err != nil {
		if isBusinessError(err) {
			return &pb.SendMessageResponse{
				Accepted:  false,
				ErrorCode: repository.CodeForError(err),
				Message:   err.Error(),
			}, nil
		}
		l.Errorf("save message failed: conversation_id=%s sender_id=%s client_msg_id=%s error=%v", in.GetConversationId(), in.GetSenderId(), in.GetClientMsgId(), err)
		return nil, err
	}
	return messageResponse(msg, duplicate), nil
}
