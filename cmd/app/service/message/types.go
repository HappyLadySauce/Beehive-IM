package message

import "time"

const (
	EventRoutingKeyMessageCreated = "message.created"

	maxTextContentBytes = 4096
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
)

// SenderIdentity is the authenticated sender context from JWT claims.
// SenderIdentity 表示来自 JWT claims 的已认证发送者上下文。
type SenderIdentity struct {
	UserID    string
	Username  string
	SessionID string
	DeviceID  string
	Platform  string
}

// SendMessageRequest is the application-level request for a text message.
// SendMessageRequest 表示文本消息发送的应用层请求。
type SendMessageRequest struct {
	ClientMessageID string
	ConversationID  string
	Content         string
}

// StoredConversation is a read model for a visible conversation.
// StoredConversation 表示可见会话的读取模型。
type StoredConversation struct {
	ID        string
	Type      string
	Title     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StoredMessage is a read model for persisted messages.
// StoredMessage 表示已持久化消息的读取模型。
type StoredMessage struct {
	MessageID       string
	ClientMessageID string
	ConversationID  string
	FromUserID      string
	Content         string
	Sequence        uint64
	SentAt          time.Time
}

// MessageCreatedEvent is published after a message is durably stored.
// MessageCreatedEvent 在消息可靠落库后发布。
type MessageCreatedEvent struct {
	MessageID        string   `json:"message_id"`
	ClientMessageID  string   `json:"client_message_id"`
	ConversationID   string   `json:"conversation_id"`
	FromUserID       string   `json:"from_user_id"`
	RecipientUserIDs []string `json:"recipient_user_ids"`
	Content          string   `json:"content"`
	Sequence         uint64   `json:"sequence"`
	SentAt           int64    `json:"sent_at"`
}

// CreateMessageCommand contains normalized data needed to persist a message.
// CreateMessageCommand 包含持久化消息所需的规范化数据。
type CreateMessageCommand struct {
	MessageID        string
	ClientMessageID  string
	ConversationID   string
	SenderUserID     string
	RecipientUserIDs []string
	Content          string
	SentAt           time.Time
}
