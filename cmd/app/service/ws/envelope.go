package ws

import (
	"encoding/json"
	"time"
)

// EnvelopeType is the type of the envelope.
// EnvelopeType 是 envelope 的类型。
const (
	TypeMessageError = 0
	TypeMessageAck = 1
	TypeMessageSend = 2
	TypeMessageReceive = 3

)

// Envelope is the envelope of the websocket message.
// Envelope 是 WebSocket 消息的 envelope。
type Envelope struct {
	ID string `json:"id"`
	Type int `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageSendPayload is the payload of the message send envelope.
// MessageSendPayload 是消息发送 envelope 的 payload。
type MessageSendPayload struct {
	ConversationID string `json:"conversation_id"`
	MessageContent string `json:"message_content"`
}




