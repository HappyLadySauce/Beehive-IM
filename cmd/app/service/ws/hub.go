package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
)

// MessageSender persists inbound websocket messages and publishes broker events.
// MessageSender 持久化 WebSocket 入站消息并发布消息代理事件。
type MessageSender interface {
	SendMessage(ctx context.Context, sender msgsvc.SenderIdentity, req msgsvc.SendMessageRequest) (msgsvc.StoredMessage, error)
}

type Hub struct {
	mu              sync.RWMutex
	clients         map[*Client]struct{}
	clientsByUserID map[string]map[*Client]struct{}
	messageSender   MessageSender
}

func NewHub(messageSender MessageSender) *Hub {
	return &Hub{
		clients:         make(map[*Client]struct{}),
		clientsByUserID: make(map[string]map[*Client]struct{}),
		messageSender:   messageSender,
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
	if h.messageSender == nil {
		return fmt.Errorf("message sender is not configured")
	}
	var payload MessageSendPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode message.send payload: %w", err)
	}
	_, err := h.messageSender.SendMessage(ctx, msgsvc.SenderIdentity{
		UserID:    sender.UserID,
		Username:  sender.Username,
		SessionID: sender.SessionID,
		DeviceID:  sender.DeviceID,
		Platform:  sender.Platform,
	}, msgsvc.SendMessageRequest{
		ClientMessageID: payload.ClientMessageID,
		ConversationID:  payload.ConversationID,
		Content:         payload.Content,
	})
	return err
}

// DeliverToOnlineUser enqueues a broker-dispatched message to local websocket clients.
// DeliverToOnlineUser 将消息代理调度的消息写入本机 WebSocket 客户端队列。
func (h *Hub) DeliverToOnlineUser(userID string, message Envelope) bool {
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
