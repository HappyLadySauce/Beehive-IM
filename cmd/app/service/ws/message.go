package ws

import (
	"encoding/json"
	"time"
)

const (
	TypeMessageSend    = "message.send"
	TypeMessageReceive = "message.receive"
	TypeAck            = "ack"
	TypeError          = "error"
)

// Envelope is the common websocket message shape exchanged with clients.
// Envelope 是客户端与服务端之间统一的 WebSocket 消息格式。
type Envelope struct {
	ID        string          `json:"id,omitempty"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// MessageSendPayload is the client request payload for one-to-one chat.
// MessageSendPayload 是客户端发送一对一聊天消息的请求载荷。
type MessageSendPayload struct {
	ConversationID string `json:"conversation_id,omitempty"`
	ToUserID       string `json:"to_user_id"`
	Content        string `json:"content"`
}

// MessageReceivePayload is delivered to the recipient online or through offline transport.
// MessageReceivePayload 会投递给在线接收方，或交给离线通道补偿。
type MessageReceivePayload struct {
	MessageID      string `json:"message_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	FromUserID     string `json:"from_user_id"`
	ToUserID       string `json:"to_user_id"`
	Content        string `json:"content"`
	SentAt         int64  `json:"sent_at"`
}

type AckPayload struct {
	MessageID string `json:"message_id,omitempty"`
	Type      string `json:"type"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newEnvelope(messageID, typ string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:        messageID,
		Type:      typ,
		Payload:   raw,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}
