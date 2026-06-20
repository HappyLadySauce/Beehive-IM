package wsproxy

import (
	"context"
	"sync"
)

// PushTarget identifies a local WebSocket delivery target.
// PushTarget 标识本地 WebSocket 投递目标。
type PushTarget struct {
	ConnID    string
	SessionID string
}

type controlMessage struct {
	Kind      string
	GatewayID string
	Reason    string
}

const controlMigrateGateway = "migrate_gateway"

// Hub indexes local WebSocket write queues.
// Hub 索引本机 WebSocket 写队列。
type Hub struct {
	mu             sync.RWMutex
	byConn         map[string]chan<- []byte
	bySession      map[string]chan<- []byte
	controlByConn  map[string]chan<- controlMessage
	gatewayByConn  map[string]string
	connsByGateway map[string]map[string]struct{}
}

func NewHub() *Hub {
	return &Hub{
		byConn:         make(map[string]chan<- []byte),
		bySession:      make(map[string]chan<- []byte),
		controlByConn:  make(map[string]chan<- controlMessage),
		gatewayByConn:  make(map[string]string),
		connsByGateway: make(map[string]map[string]struct{}),
	}
}

func (h *Hub) Register(connID, sessionID string, outbound chan<- []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if connID != "" {
		h.byConn[connID] = outbound
	}
	if sessionID != "" {
		h.bySession[sessionID] = outbound
	}
}

func (h *Hub) Unregister(connID, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if connID != "" {
		delete(h.byConn, connID)
	}
	if sessionID != "" {
		delete(h.bySession, sessionID)
	}
}

func (h *Hub) RegisterConnection(connID, sessionID, gatewayID string, outbound chan<- []byte, control chan<- controlMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if connID != "" {
		h.byConn[connID] = outbound
		h.controlByConn[connID] = control
	}
	if sessionID != "" {
		h.bySession[sessionID] = outbound
	}
	h.setGatewayLocked(connID, gatewayID)
}

func (h *Hub) UnregisterConnection(connID, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if gatewayID := h.gatewayByConn[connID]; gatewayID != "" {
		h.removeGatewayConnLocked(gatewayID, connID)
	}
	if connID != "" {
		delete(h.byConn, connID)
		delete(h.controlByConn, connID)
		delete(h.gatewayByConn, connID)
	}
	if sessionID != "" {
		delete(h.bySession, sessionID)
	}
}

func (h *Hub) UpdateGateway(connID, gatewayID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if oldGatewayID := h.gatewayByConn[connID]; oldGatewayID != "" && oldGatewayID != gatewayID {
		h.removeGatewayConnLocked(oldGatewayID, connID)
	}
	h.setGatewayLocked(connID, gatewayID)
}

func (h *Hub) MigrateGateway(gatewayID, reason string) int {
	h.mu.RLock()
	connIDs := make([]string, 0, len(h.connsByGateway[gatewayID]))
	for connID := range h.connsByGateway[gatewayID] {
		connIDs = append(connIDs, connID)
	}
	controls := make([]chan<- controlMessage, 0, len(connIDs))
	for _, connID := range connIDs {
		if ch := h.controlByConn[connID]; ch != nil {
			controls = append(controls, ch)
		}
	}
	h.mu.RUnlock()

	sent := 0
	for _, ch := range controls {
		select {
		case ch <- controlMessage{Kind: controlMigrateGateway, GatewayID: gatewayID, Reason: reason}:
			sent++
		default:
		}
	}
	return sent
}

func (h *Hub) Deliver(ctx context.Context, target PushTarget, payload []byte) bool {
	h.mu.RLock()
	ch := h.byConn[target.ConnID]
	if ch == nil {
		ch = h.bySession[target.SessionID]
	}
	h.mu.RUnlock()
	if ch == nil {
		return false
	}

	select {
	case ch <- payload:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func (h *Hub) setGatewayLocked(connID, gatewayID string) {
	if connID == "" || gatewayID == "" {
		return
	}
	h.gatewayByConn[connID] = gatewayID
	if h.connsByGateway[gatewayID] == nil {
		h.connsByGateway[gatewayID] = make(map[string]struct{})
	}
	h.connsByGateway[gatewayID][connID] = struct{}{}
}

func (h *Hub) removeGatewayConnLocked(gatewayID, connID string) {
	conns := h.connsByGateway[gatewayID]
	if conns == nil {
		return
	}
	delete(conns, connID)
	if len(conns) == 0 {
		delete(h.connsByGateway, gatewayID)
	}
}
