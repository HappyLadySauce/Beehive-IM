package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	"k8s.io/klog/v2"
)

// Hub is the central point for managing WebSocket connections.
// Hub 是管理 WebSocket 连接的中心点。
type Hub struct {
	mu sync.RWMutex

	messages *message.MessageService

	clients        map[*Client]struct{}
	clientByUserID map[string]map[*Client]struct{}
}

// NewHub creates a new Hub.
// NewHub 创建一个新的 Hub。
func NewHub(messages *message.MessageService) *Hub {
	return &Hub{
		messages:       messages,
		clients:        make(map[*Client]struct{}),
		clientByUserID: make(map[string]map[*Client]struct{}),
	}
}

// Register registers a client to the hub.
// Register 注册一个客户端到 Hub。
func (h *Hub) Register(client *Client) error {
	if h == nil || client == nil {
		return fmt.Errorf("hub or client is nil")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if clients := h.clientByUserID[client.Identity.UserID]; clients != nil {
		for old := range clients {
			if !old.IsSame(client) {
				continue
			}
			if err := old.Close(); err != nil {
				klog.ErrorS(err, "close old client", "userID", client.Identity.UserID, "sessionID", client.Identity.SessionID)
			}
			delete(h.clients, old)
			delete(clients, old)
		}
	}

	h.clients[client] = struct{}{}
	if h.clientByUserID[client.Identity.UserID] == nil {
		h.clientByUserID[client.Identity.UserID] = make(map[*Client]struct{})
	}
	h.clientByUserID[client.Identity.UserID][client] = struct{}{}

	return nil
}

// Unregister unregisters a client from the hub.
// Unregister 注销一个客户端从 Hub。
func (h *Hub) Unregister(client *Client) error {
	if h == nil || client == nil {
		return fmt.Errorf("hub or client is nil")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return nil
	}

	delete(h.clients, client)
	delete(h.clientByUserID[client.Identity.UserID], client)
	if len(h.clientByUserID[client.Identity.UserID]) == 0 {
		delete(h.clientByUserID, client.Identity.UserID)
	}

	return client.Close()
}

// HandleEnvelope handles an inbound envelope from a connected client.
// HandleEnvelope 处理来自已连接客户端的入站 envelope。
func (h *Hub) HandleEnvelope(ctx context.Context, client *Client, envelope Envelope) error {
	if h == nil || client == nil {
		return fmt.Errorf("hub or client is nil")
	}

	switch envelope.Type {
	case TypeMessageSend:
		return h.handleMessageSend(ctx, client, envelope)
	default:
		return fmt.Errorf("unknown envelope type: %d", envelope.Type)
	}
}

// handleMessageSend persists and fans out a message, then ACKs the sender.
// handleMessageSend 落库并扇出消息，然后向发送方返回 ACK。
func (h *Hub) handleMessageSend(ctx context.Context, client *Client, envelope Envelope) error {
	if h == nil || h.messages == nil {
		return fmt.Errorf("message service is not configured")
	}

	var payload message.MessageSendPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal message send payload: %w", err)
	}

	senderUserID, err := strconv.ParseUint(client.Identity.UserID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid sender user id: %w", err)
	}

	result, err := h.messages.SendMessage(ctx, senderUserID, payload)
	if err != nil {
		errPayload, marshalErr := message.MarshalProtocolError(err)
		if marshalErr != nil {
			return fmt.Errorf("marshal protocol error: %w", marshalErr)
		}
		if sendErr := client.SendEnvelope(Envelope{
			ID:        envelope.ID,
			Type:      TypeMessageError,
			Payload:   errPayload,
			Timestamp: time.Now().UTC(),
		}); sendErr != nil {
			return sendErr
		}
		return nil
	}

	ackPayload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal ack payload: %w", err)
	}

	return client.SendEnvelope(Envelope{
		ID:        envelope.ID,
		Type:      TypeMessageAck,
		Payload:   ackPayload,
		Timestamp: time.Now().UTC(),
	})
}

// DeliverToUser pushes a received message to all online sessions of the user.
// DeliverToUser 将收到的消息推送给该用户的所有在线会话。
func (h *Hub) DeliverToUser(ctx context.Context, payload message.MessageDeliverPayload) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal receive payload: %w", err)
	}

	envelope := Envelope{
		ID:        payload.MessageID,
		Type:      TypeMessageReceive,
		Payload:   body,
		Timestamp: time.Now().UTC(),
	}

	h.mu.RLock()
	clientSet := h.clientByUserID[payload.RecipientUserID]
	targets := make([]*Client, 0, len(clientSet))
	for client := range clientSet {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	for _, client := range targets {
		if err := client.SendEnvelope(envelope); err != nil {
			klog.ErrorS(err, "deliver message to client",
				"recipientUserID", payload.RecipientUserID,
				"messageID", payload.MessageID,
				"sessionID", client.Identity.SessionID,
			)
		}
	}
	return nil
}
