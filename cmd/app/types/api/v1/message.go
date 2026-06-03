package v1

import "time"

// ConversationResponse is returned for a conversation visible to the caller.
// ConversationResponse 表示调用方可见的会话信息。
type ConversationResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     *string   `json:"title,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListConversationsResponse wraps conversation list results.
// ListConversationsResponse 包装会话列表结果。
type ListConversationsResponse struct {
	Items []ConversationResponse `json:"items"`
}

// MessageResponseItem is returned for historical message queries.
// MessageResponseItem 表示历史消息查询返回项。
type MessageResponseItem struct {
	MessageID       string    `json:"message_id"`
	ClientMessageID string    `json:"client_message_id"`
	ConversationID  string    `json:"conversation_id"`
	FromUserID      string    `json:"from_user_id"`
	Content         string    `json:"content"`
	Sequence        uint64    `json:"sequence"`
	SentAt          time.Time `json:"sent_at"`
}

// ListMessagesResponse wraps paginated message history.
// ListMessagesResponse 包装分页历史消息结果。
type ListMessagesResponse struct {
	Items []MessageResponseItem `json:"items"`
}
