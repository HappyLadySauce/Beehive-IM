package message

import (
)

type MessageSendPayload struct {
	ConversationID string `json:"conversation_id"`
	MessageContent string `json:"message_content"`
	ToUserID string `json:"to_user_id"`
	FromUserID string `json:"from_user_id"`
}


