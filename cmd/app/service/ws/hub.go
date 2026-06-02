package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// OfflinePublisher publishes messages that cannot be delivered to an online recipient.
// OfflinePublisher 发布无法在线投递的消息，RabbitMQ 接入时实现该接口。
type OfflinePublisher interface {
	PublishOffline(ctx context.Context, message Envelope) error
}

type Hub struct {
	mu               sync.RWMutex
	clients          map[*Client]struct{}
	clientsByUserID  map[string]map[*Client]struct{}
	offlinePublisher OfflinePublisher
}

func NewHub(offlinePublisher OfflinePublisher) *Hub {
	return &Hub{
		clients:          make(map[*Client]struct{}),
		clientsByUserID:  make(map[string]map[*Client]struct{}),
		offlinePublisher: offlinePublisher,
	}
}

func (h *Hub) Register(client *Client) {
	if h == nil || client == nil {
		return
	}
	client.hub = h

	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}
	if h.clientsByUserID[client.Identity.UserID] == nil {
		h.clientsByUserID[client.Identity.UserID] = make(map[*Client]struct{})
	}
	h.clientsByUserID[client.Identity.UserID][client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	if h == nil || client == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)

	userClients := h.clientsByUserID[client.Identity.UserID]
	delete(userClients, client)
	if len(userClients) == 0 {
		delete(h.clientsByUserID, client.Identity.UserID)
	}
	if client.Conn != nil {
		_ = client.Conn.Close()
	}
}

func (h *Hub) HandleEnvelope(ctx context.Context, sender ClientIdentity, envelope Envelope) error {
	if h == nil {
		return fmt.Errorf("ws hub is not initialized")
	}
	switch envelope.Type {
	case TypeMessageSend:
		return h.handleMessageSend(ctx, sender, envelope)
	default:
		return fmt.Errorf("unsupported message type: %s", envelope.Type)
	}
}

func (h *Hub) handleMessageSend(ctx context.Context, sender ClientIdentity, envelope Envelope) error {
	var payload MessageSendPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode message.send payload: %w", err)
	}
	payload.ToUserID = strings.TrimSpace(payload.ToUserID)
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.ToUserID == "" {
		return fmt.Errorf("to_user_id is required")
	}
	if payload.Content == "" {
		return fmt.Errorf("content is required")
	}

	receivePayload := MessageReceivePayload{
		MessageID:      envelope.ID,
		ConversationID: payload.ConversationID,
		FromUserID:     sender.UserID,
		ToUserID:       payload.ToUserID,
		Content:        payload.Content,
		SentAt:         envelope.Timestamp,
	}
	message, err := newEnvelope(envelope.ID, TypeMessageReceive, receivePayload)
	if err != nil {
		return fmt.Errorf("encode receive message: %w", err)
	}

	if h.deliverToOnlineUser(payload.ToUserID, message) {
		return nil
	}
	if h.offlinePublisher == nil {
		return fmt.Errorf("offline publisher is not configured")
	}
	if err := h.offlinePublisher.PublishOffline(ctx, message); err != nil {
		return fmt.Errorf("publish offline message: %w", err)
	}
	return nil
}

func (h *Hub) deliverToOnlineUser(userID string, message Envelope) bool {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clientsByUserID[userID]))
	for client := range h.clientsByUserID[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	delivered := false
	for _, client := range clients {
		select {
		case client.Send <- message:
			delivered = true
		default:
			h.Unregister(client)
		}
	}
	return delivered
}
