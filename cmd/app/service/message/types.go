package message

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// maxClientMessageIDLen matches messages.client_message_id VARCHAR(128).
	// maxClientMessageIDLen 与 messages.client_message_id VARCHAR(128) 一致。
	maxClientMessageIDLen = 128
	// maxMessageContentLen aligns with the WebSocket frame size cap in the WS client.
	// maxMessageContentLen 与 WS 客户端帧大小上限一致。
	maxMessageContentLen = 64 << 10
)

// MessageSendPayload is the WebSocket send request body.
// MessageSendPayload 是 WebSocket 发消息请求体。
type MessageSendPayload struct {
	ConversationID  string `json:"conversation_id"`
	ClientMessageID string `json:"client_message_id"`
	MessageContent  string `json:"message_content"`
}

// SendParams is the validated internal representation after parsing the wire payload.
// SendParams 是解析并校验后的内部发消息参数。
type SendParams struct {
	ConversationID  uint64 `json:"conversation_id"`
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content"`
}

// Parse validates and converts the wire payload into SendParams.
// Parse 校验并将对外载荷转换为内部 SendParams。
func (p MessageSendPayload) Parse() (SendParams, error) {
	convID, err := strconv.ParseUint(strings.TrimSpace(p.ConversationID), 10, 64)
	if err != nil {
		return SendParams{}, fmt.Errorf("%w: invalid conversation_id", ErrInvalidSendRequest)
	}
	clientMessageID := strings.TrimSpace(p.ClientMessageID)
	if clientMessageID == "" {
		return SendParams{}, fmt.Errorf("%w: client_message_id is required", ErrInvalidSendRequest)
	}
	if len(clientMessageID) > maxClientMessageIDLen {
		return SendParams{}, fmt.Errorf("%w: client_message_id exceeds %d characters", ErrInvalidSendRequest, maxClientMessageIDLen)
	}
	content := strings.TrimSpace(p.MessageContent)
	if content == "" {
		return SendParams{}, fmt.Errorf("%w: message_content is required", ErrInvalidSendRequest)
	}
	if len(content) > maxMessageContentLen {
		return SendParams{}, fmt.Errorf("%w: message_content exceeds %d characters", ErrInvalidSendRequest, maxMessageContentLen)
	}
	return SendParams{
		ConversationID:  convID,
		ClientMessageID: clientMessageID,
		Content:         content,
	}, nil
}

// MessageSendResult is returned to the sender as TypeMessageAck payload.
// MessageSendResult 作为 TypeMessageAck 载荷返回给发送方。
type MessageSendResult struct {
	MessageID       string    `json:"message_id"`
	ConversationID  string    `json:"conversation_id"`
	ClientMessageID string    `json:"client_message_id"`
	Sequence        uint64    `json:"sequence"`
	SentAt          time.Time `json:"sent_at"`
}

// MessageDeliverPayload is published to RabbitMQ per recipient.
// MessageDeliverPayload 按接收人发布到 RabbitMQ。
type MessageDeliverPayload struct {
	MessageID       string    `json:"message_id"`
	ConversationID  string    `json:"conversation_id"`
	FromUserID      string    `json:"from_user_id"`
	RecipientUserID string    `json:"recipient_user_id"`
	ClientMessageID string    `json:"client_message_id"`
	Content         string    `json:"content"`
	Sequence        uint64    `json:"sequence"`
	SentAt          time.Time `json:"sent_at"`
}
