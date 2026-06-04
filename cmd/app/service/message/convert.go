package message

import (
	"strconv"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
)

// FormatUserID converts a numeric user id to its canonical string form.
// FormatUserID 将数值用户 ID 转为标准字符串形式。
func FormatUserID(userID uint64) string {
	return strconv.FormatUint(userID, 10)
}

// FormatConversationID converts a numeric conversation id to its canonical string form.
// FormatConversationID 将数值会话 ID 转为标准字符串形式。
func FormatConversationID(conversationID uint64) string {
	return strconv.FormatUint(conversationID, 10)
}

// ResultFromModel builds an ACK payload from a persisted message row.
// ResultFromModel 根据已持久化的消息记录构造 ACK 载荷。
func ResultFromModel(m model.Message) *MessageSendResult {
	return &MessageSendResult{
		MessageID:       m.MessageID,
		ConversationID:  FormatConversationID(m.ConversationID),
		ClientMessageID: m.ClientMessageID,
		Sequence:        m.Sequence,
		SentAt:          m.SentAt,
	}
}

// DeliverPayloadFromModel builds a per-recipient MQ payload from a persisted message.
// DeliverPayloadFromModel 根据已持久化的消息记录构造按人投递的 MQ 载荷。
func DeliverPayloadFromModel(m model.Message, fromUserID, recipientUserID uint64) MessageDeliverPayload {
	return MessageDeliverPayload{
		MessageID:       m.MessageID,
		ConversationID:  FormatConversationID(m.ConversationID),
		FromUserID:      FormatUserID(fromUserID),
		RecipientUserID: FormatUserID(recipientUserID),
		ClientMessageID: m.ClientMessageID,
		Content:         m.Content,
		Sequence:        m.Sequence,
		SentAt:          m.SentAt,
	}
}
