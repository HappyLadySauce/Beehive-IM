package ws

import (
	"fmt"
	"sync"
	"context"
	"encoding/json"

	"k8s.io/klog/v2"


)


// Hub is the central point for managing WebSocket connections.
// Hub 是管理 WebSocket 连接的中心点。
type Hub struct {
	mu sync.RWMutex

	// clients is a map of all clients.
	// clients 是所有客户端的映射。
	clients map[*Client]struct{}
	// clientByUserID is a map of clients by user ID.
	// clientByUserID 是按用户 ID 映射的客户端, 用于多端索引管理。
	clientByUserID map[string]map[*Client]struct{}
}

// NewHub creates a new Hub.
// NewHub 创建一个新的 Hub。
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
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

	// On reconnect, evict the previous connection for the same UserID + SessionID.
	// 同一 UserID + SessionID 重连时，先踢掉旧连接，避免 Hub 里残留僵尸 Client。
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

	// Add the new client to the Hub.
	// 将新客户端添加到 Hub。
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

	if err := client.Close(); err != nil {
		klog.ErrorS(err, "close client", "userID", client.Identity.UserID, "sessionID", client.Identity.SessionID)
	}
	delete(h.clients, client)

	delete(h.clientByUserID[client.Identity.UserID], client)
	if len(h.clientByUserID[client.Identity.UserID]) == 0 {
		delete(h.clientByUserID, client.Identity.UserID)
	}
	return nil
}

// HandleEnvelope handles an envelope.
// HandleEnvelope 处理一个 envelope。
func (h *Hub) HandleEnvelope(ctx context.Context, envelope Envelope) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}

	switch envelope.Type {
	case TypeMessageSend:
		return h.handleMessageSend(ctx, envelope)
	default:
		return fmt.Errorf("unknown envelope type: %d", envelope.Type)
	}
}

// handleMessageSend handles a message send envelope.
// handleMessageSend 处理一个消息发送 envelope。
func (h *Hub) handleMessageSend(ctx context.Context, envelope Envelope) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}

	var payload MessageSendPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal message send payload: %w", err)
	}

	

	return nil
}