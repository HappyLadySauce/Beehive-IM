package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/message/pb"
)

const (
	timeFormat           = time.RFC3339
	maxTextBytes         = 4096
	maxContentJSONBytes  = 8192
	messageCreatedPrefix = "message.created."
	codePermissionDenied = "PERMISSION_DENIED"
)

func messageResponse(msg repository.Message, duplicate bool) *pb.SendMessageResponse {
	return &pb.SendMessageResponse{
		Accepted:       true,
		Message:        "persisted",
		MessageId:      msg.MessageID,
		ConversationId: msg.ConversationID,
		Seq:            msg.Seq,
		SenderId:       msg.SenderID,
		ContentType:    msg.ContentType,
		CreatedAt:      msg.CreatedAt.UTC().Format(timeFormat),
		Duplicate:      duplicate,
	}
}

func messageItemPB(msg repository.Message) *pb.MessageItem {
	return &pb.MessageItem{
		MessageId:      msg.MessageID,
		ConversationId: msg.ConversationID,
		Seq:            msg.Seq,
		SenderId:       msg.SenderID,
		DeviceId:       msg.DeviceID,
		ClientMsgId:    msg.ClientMsgID,
		ClientSeq:      msg.ClientSeq,
		ContentType:    msg.ContentType,
		ContentJson:    string(msg.ContentJSON),
		CreatedAt:      msg.CreatedAt.UTC().Format(timeFormat),
	}
}

func messageItemsPB(messages []repository.Message) []*pb.MessageItem {
	out := make([]*pb.MessageItem, 0, len(messages))
	for _, msg := range messages {
		out = append(out, messageItemPB(msg))
	}
	return out
}

func normalizeContent(contentType, contentJSON string) (string, []byte, error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		contentType = repository.ContentTypeText
	}
	if contentType != repository.ContentTypeText {
		return "", nil, fmt.Errorf("%w: unsupported content_type", repository.ErrInvalidArgument)
	}
	contentJSON = strings.TrimSpace(contentJSON)
	if contentJSON == "" || len(contentJSON) > maxContentJSONBytes || !json.Valid([]byte(contentJSON)) {
		return "", nil, fmt.Errorf("%w: invalid content_json", repository.ErrInvalidArgument)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(contentJSON), &payload); err != nil {
		return "", nil, fmt.Errorf("%w: invalid text content", repository.ErrInvalidArgument)
	}
	if strings.TrimSpace(payload.Text) == "" {
		return "", nil, fmt.Errorf("%w: text is required", repository.ErrInvalidArgument)
	}
	if len([]byte(payload.Text)) > maxTextBytes {
		return "", nil, fmt.Errorf("%w: text is too long", repository.ErrInvalidArgument)
	}
	return contentType, []byte(contentJSON), nil
}

func buildOutboxPayload(msg repository.SaveMessageInput) ([]byte, error) {
	return json.Marshal(map[string]any{
		"event_type":      "message.created",
		"conversation_id": msg.ConversationID,
		"seq":             msg.Seq,
		"sender_id":       msg.SenderID,
		"device_id":       msg.DeviceID,
		"client_msg_id":   msg.ClientMsgID,
		"content_type":    msg.ContentType,
		"content":         json.RawMessage(msg.ContentJSON),
		"created_at":      time.Now().UTC().Format(timeFormat),
	})
}

func permissionDeniedFromConversation(resp *conversationservice.CheckSendPermissionResponse) *pb.SendMessageResponse {
	code := resp.GetErrorCode()
	if code == "" {
		code = codePermissionDenied
	}
	return &pb.SendMessageResponse{
		Accepted:  false,
		ErrorCode: code,
		Message:   resp.GetMessage(),
	}
}

func sequenceRejectedFromConversation(resp *conversationservice.AllocateMessageSeqResponse) *pb.SendMessageResponse {
	code := resp.GetErrorCode()
	if code == "" {
		code = codePermissionDenied
	}
	return &pb.SendMessageResponse{
		Accepted:  false,
		ErrorCode: code,
		Message:   resp.GetMessage(),
	}
}

func ackRejected(code, message string) *pb.AckMessagesResponse {
	if code == "" {
		code = repository.CodeInvalidArgument
	}
	return &pb.AckMessagesResponse{
		Accepted:  false,
		ErrorCode: code,
		Message:   message,
	}
}

func listRejected(code, message string) *pb.ListMessagesResponse {
	if code == "" {
		code = repository.CodeInvalidArgument
	}
	return &pb.ListMessagesResponse{
		Accepted:  false,
		ErrorCode: code,
		Message:   message,
	}
}

func syncRejected(code, message string) *pb.SyncMessagesResponse {
	if code == "" {
		code = repository.CodeInvalidArgument
	}
	return &pb.SyncMessagesResponse{
		Accepted:  false,
		ErrorCode: code,
		Message:   message,
	}
}

func isBusinessError(err error) bool {
	return errors.Is(err, repository.ErrInvalidArgument)
}
