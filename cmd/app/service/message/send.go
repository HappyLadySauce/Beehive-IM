package message

import (
	"fmt"
	"context"
	"encoding/json"
)

const (
	MessageSendTopic = "message.send"
)

func (s *MessageService) SendMessage(ctx context.Context, conversationID string, message MessageSendPayload) error {
	if s == nil || s.MQ == nil {
		return fmt.Errorf("message service or mq is nil")
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message send payload: %w", err)
	}

	return s.MQ.SendMessage(ctx, fmt.Sprintf("message.send.%s", conversationID), payload)
}