package session

import (
	"errors"
	"sync"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
)

const (
	CodeInvalidSession       = "INVALID_SESSION"
	CodeSessionAlreadyExists = "SESSION_ALREADY_EXISTS"
	CodeSessionNotFound      = "SESSION_NOT_FOUND"
	CodeSessionCapacity      = "SESSION_CAPACITY_EXCEEDED"
)

var (
	ErrInvalidSession       = errors.New("invalid session")
	ErrSessionAlreadyExists = errors.New("session already exists")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionCapacity      = errors.New("session capacity exceeded")
)

// Session stores rebuildable upstream state for one Edge connection.
// Session 存储单个 Edge 连接可重建的上游状态。
type Session struct {
	SessionID        string
	ConnID           string
	EdgeID           string
	UserID           string
	DeviceID         string
	LastClientSeq    int64
	LastDeliveredSeq int64
	ServerSeq        int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Manager owns in-memory Gateway sessions for the MVP.
// Manager 管理 MVP 阶段的 Gateway 内存会话。
type Manager struct {
	mu          sync.RWMutex
	gatewayID   string
	maxSessions int
	sessions    map[string]*Session
}

func NewManager(gatewayID string, maxSessions int) *Manager {
	if gatewayID == "" {
		gatewayID = "gateway-dev"
	}
	if maxSessions <= 0 {
		maxSessions = 20000
	}

	return &Manager{
		gatewayID:   gatewayID,
		maxSessions: maxSessions,
		sessions:    make(map[string]*Session),
	}
}

func (m *Manager) GatewayID() string {
	return m.gatewayID
}

func (m *Manager) Attach(req *pb.AttachRequest) (*Session, error) {
	if req.GetSessionId() == "" || req.GetConnId() == "" || req.GetEdgeId() == "" || req.GetUserId() == "" {
		return nil, ErrInvalidSession
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[req.GetSessionId()]; ok {
		return nil, ErrSessionAlreadyExists
	}
	if len(m.sessions) >= m.maxSessions {
		return nil, ErrSessionCapacity
	}

	now := time.Now().UTC()
	sess := &Session{
		SessionID:        req.GetSessionId(),
		ConnID:           req.GetConnId(),
		EdgeID:           req.GetEdgeId(),
		UserID:           req.GetUserId(),
		DeviceID:         req.GetDeviceId(),
		LastClientSeq:    req.GetLastClientSeq(),
		LastDeliveredSeq: req.GetLastDeliveredSeq(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	m.sessions[sess.SessionID] = sess
	return cloneSession(sess), nil
}

func (m *Manager) Resume(req *pb.ResumeRequest) (*Session, error) {
	if req.GetSessionId() == "" || req.GetConnId() == "" || req.GetEdgeId() == "" || req.GetUserId() == "" {
		return nil, ErrInvalidSession
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[req.GetSessionId()]; ok {
		sess.ConnID = req.GetConnId()
		sess.EdgeID = req.GetEdgeId()
		sess.UserID = req.GetUserId()
		sess.DeviceID = req.GetDeviceId()
		sess.LastClientSeq = req.GetLastClientSeq()
		sess.LastDeliveredSeq = req.GetLastDeliveredSeq()
		sess.UpdatedAt = time.Now().UTC()
		return cloneSession(sess), nil
	}
	if len(m.sessions) >= m.maxSessions {
		return nil, ErrSessionCapacity
	}

	now := time.Now().UTC()
	sess := &Session{
		SessionID:        req.GetSessionId(),
		ConnID:           req.GetConnId(),
		EdgeID:           req.GetEdgeId(),
		UserID:           req.GetUserId(),
		DeviceID:         req.GetDeviceId(),
		LastClientSeq:    req.GetLastClientSeq(),
		LastDeliveredSeq: req.GetLastDeliveredSeq(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	m.sessions[sess.SessionID] = sess
	return cloneSession(sess), nil
}

func (m *Manager) Close(req *pb.CloseSessionRequest) bool {
	if req.GetSessionId() == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[req.GetSessionId()]
	if !ok {
		return false
	}
	if req.GetConnId() != "" && sess.ConnID != req.GetConnId() {
		return false
	}
	if req.GetEdgeId() != "" && sess.EdgeID != req.GetEdgeId() {
		return false
	}

	delete(m.sessions, req.GetSessionId())
	return true
}

func (m *Manager) Exists(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.sessions[sessionID]
	return ok
}

func (m *Manager) NextServerSeq(sessionID string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return 0, false
	}

	sess.ServerSeq++
	sess.UpdatedAt = time.Now().UTC()
	return sess.ServerSeq, true
}

func cloneSession(sess *Session) *Session {
	cp := *sess
	return &cp
}

func CodeForError(err error) string {
	switch {
	case errors.Is(err, ErrInvalidSession):
		return CodeInvalidSession
	case errors.Is(err, ErrSessionAlreadyExists):
		return CodeSessionAlreadyExists
	case errors.Is(err, ErrSessionCapacity):
		return CodeSessionCapacity
	case errors.Is(err, ErrSessionNotFound):
		return CodeSessionNotFound
	default:
		return "INTERNAL_ERROR"
	}
}
