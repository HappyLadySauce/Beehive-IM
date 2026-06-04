package message

import (
	"errors"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrInvalidSendRequest means the send payload failed validation.
	// ErrInvalidSendRequest 表示发消息请求校验失败。
	ErrInvalidSendRequest = errors.New("invalid send request")
	// ErrConversationNotFound means the conversation does not exist.
	// ErrConversationNotFound 表示会话不存在。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrNotConversationMember means the sender is not in the conversation.
	// ErrNotConversationMember 表示发送者不是会话成员。
	ErrNotConversationMember = errors.New("not a conversation member")
	// ErrConversationMismatch means client_message_id belongs to another conversation.
	// ErrConversationMismatch 表示幂等键属于其他会话。
	ErrConversationMismatch = errors.New("conversation mismatch for client message id")
)


// isPostgresUniqueViolation reports whether err is a PostgreSQL unique constraint violation (SQLSTATE 23505).
// isPostgresUniqueViolation 判断 err 是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
func isPostgresUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ProtocolErrorPayload is returned as TypeMessageError payload.
// ProtocolErrorPayload 作为 TypeMessageError 的载荷返回。
type ProtocolErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// protocolErrorFromSendErr converts an error to a ProtocolErrorPayload.
// protocolErrorFromSendErr 将错误转换为 ProtocolErrorPayload。
func ProtocolErrorFromSendErr(err error) ProtocolErrorPayload {
	switch {
	case errors.Is(err, ErrInvalidSendRequest):
		return ProtocolErrorPayload{Code: "invalid_request", Message: err.Error()}
	case errors.Is(err, ErrConversationNotFound):
		return ProtocolErrorPayload{Code: "conversation_not_found", Message: err.Error()}
	case errors.Is(err, ErrNotConversationMember):
		return ProtocolErrorPayload{Code: "not_conversation_member", Message: err.Error()}
	case errors.Is(err, ErrConversationMismatch):
		return ProtocolErrorPayload{Code: "conversation_mismatch", Message: err.Error()}
	default:
		return ProtocolErrorPayload{Code: "send_failed", Message: err.Error()}
	}
}

// MarshalProtocolError marshals an error into a ProtocolErrorPayload.
// MarshalProtocolError 将错误编码为 ProtocolErrorPayload。
func MarshalProtocolError(err error) ([]byte, error) {
	return json.Marshal(ProtocolErrorFromSendErr(err))
}
