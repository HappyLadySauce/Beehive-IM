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

// Hub indexes local WebSocket write queues.
// Hub 索引本机 WebSocket 写队列。
type Hub struct {
	mu        sync.RWMutex
	byConn    map[string]chan<- []byte
	bySession map[string]chan<- []byte
}

func NewHub() *Hub {
	return &Hub{
		byConn:    make(map[string]chan<- []byte),
		bySession: make(map[string]chan<- []byte),
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
