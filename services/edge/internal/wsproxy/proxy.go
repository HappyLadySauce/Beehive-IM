package wsproxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/presence"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
	"github.com/gorilla/websocket"
)

const (
	defaultWriteBufferSize = 64
	defaultReadLimitBytes  = 65536
	attachTimeout          = 2 * time.Second
	rebindTimeout          = 3 * time.Second
	defaultRecoveryWindow  = 5 * time.Second
	defaultIsolation       = 10 * time.Second
	writeTimeout           = 5 * time.Second
	pongWait               = 60 * time.Second
	pingPeriod             = 54 * time.Second
)

var errOutboundBufferFull = errors.New("websocket outbound buffer is full")

type Proxy struct {
	edgeID          string
	writeBufferSize int
	readLimitBytes  int64
	tickets         *ticket.Store
	gatewayRouter   GatewayRouter
	presence        presence.Client
	hub             *Hub
	recovery        RecoveryConfig
	upgrader        websocket.Upgrader
}

type Config struct {
	EdgeID          string
	WriteBufferSize int
	ReadLimitBytes  int64
	Tickets         *ticket.Store
	Gateway         gatewayservice.GatewayService
	GatewayRouter   GatewayRouter
	Presence        presence.Client
	Recovery        RecoveryConfig
}

type GatewayRouter interface {
	Pick(ctx context.Context, excludedGatewayIDs ...string) (gatewayservice.GatewayService, string, error)
	MarkFailed(gatewayID string)
}

type clientEnvelope struct {
	Type string          `json:"type"`
	Seq  int64           `json:"seq"`
	Data json.RawMessage `json:"payload"`
}

type upstreamEndpoint struct {
	gateway   gatewayservice.GatewayService
	gatewayID string
}

type gatewaySession struct {
	ticket           ticket.Ticket
	connID           string
	endpoint         upstreamEndpoint
	lastClientSeq    int64
	lastDeliveredSeq int64
}

// RecoveryConfig controls Edge -> Gateway rebind attempts.
// RecoveryConfig 控制 Edge 到 Gateway 的恢复尝试。
type RecoveryConfig struct {
	MaxAttempts int
	Window      time.Duration
	Backoffs    []time.Duration
	Isolation   time.Duration
}

type gatewayRecvResult struct {
	frame *pb.GatewayFrame
	err   error
}

func NewProxy(c Config) *Proxy {
	if c.EdgeID == "" {
		c.EdgeID = "edge-dev"
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.ReadLimitBytes <= 0 {
		c.ReadLimitBytes = defaultReadLimitBytes
	}
	if c.Tickets == nil {
		c.Tickets = ticket.NewStore(30 * time.Second)
	}
	if c.Presence == nil {
		c.Presence = presence.NewNoopClient()
	}
	if c.GatewayRouter == nil && c.Gateway != nil {
		c.GatewayRouter = staticGatewayRouter{gateway: c.Gateway}
	}
	recovery := normalizeRecoveryConfig(c.Recovery)

	return &Proxy{
		edgeID:          c.EdgeID,
		writeBufferSize: c.WriteBufferSize,
		readLimitBytes:  c.ReadLimitBytes,
		tickets:         c.Tickets,
		gatewayRouter:   c.GatewayRouter,
		presence:        c.Presence,
		hub:             NewHub(),
		recovery:        recovery,
		upgrader: websocket.Upgrader{
			Subprotocols: []string{"beehive.im.v1"},
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// ServeHTTP handles the Edge WebSocket upgrade path: validate ticket, bind upstream
// Gateway session, then run a three-goroutine bidirectional proxy until disconnect.
// ServeHTTP 处理 Edge WebSocket 升级入口：校验 ticket、绑定上游 Gateway 会话，
// 再启动三协程双向代理，直至连接断开。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Phase 1: one-time ticket consumption (binds session/user/device, checks Origin).
	// 阶段 1：一次性消费 ticket（绑定 session/user/device，并校验 Origin）。
	t, err := p.tickets.Consume(r.URL.Query().Get("ticket"), r.Header.Get("Origin"))
	if err != nil {
		http.Error(w, "Invalid websocket ticket", http.StatusUnauthorized)
		return
	}

	// Phase 2: allocate a unique connection id for this WebSocket leg.
	// 阶段 2：为本 WebSocket 连接分配唯一 conn_id。
	connID := "conn-" + randomToken(12)
	if p.gatewayRouter == nil {
		http.Error(w, "Gateway client is unavailable", http.StatusServiceUnavailable)
		return
	}

	// Phase 3: pick a Gateway instance and attach the session before HTTP upgrade.
	// All upstream setup must finish while the response is still plain HTTP.
	// 阶段 3：在 HTTP 升级前选择 Gateway 并完成 Attach；上游准备必须在仍为 HTTP 响应时完成。
	endpoint, err := p.pickGateway(r.Context())
	if err != nil {
		http.Error(w, "Gateway selection failed", http.StatusServiceUnavailable)
		return
	}
	attach, err := p.attach(r.Context(), endpoint.gateway, t, connID)
	if err != nil {
		http.Error(w, "Gateway attach failed", http.StatusServiceUnavailable)
		return
	}
	if !attach.GetAccepted() {
		http.Error(w, attach.GetErrorCode(), http.StatusServiceUnavailable)
		return
	}
	endpoint.gatewayID = gatewayID(attach.GetGatewayId(), endpoint.gatewayID)

	// Phase 4: register connection metadata in presence for routing and TTL refresh.
	// 阶段 4：在 presence 中登记连接元数据，供路由与 TTL 续期使用。
	if err := p.presence.UpsertConnection(r.Context(), presence.ConnectionMeta{
		SessionID: t.SessionID,
		ConnID:    connID,
		EdgeID:    p.edgeID,
		UserID:    t.UserID,
		DeviceID:  t.DeviceID,
		GatewayID: endpoint.gatewayID,
	}); err != nil {
		p.closeGatewaySession(endpoint.gateway, t, connID, "presence_upsert_failed")
		http.Error(w, "Presence upsert failed", http.StatusServiceUnavailable)
		return
	}

	// Phase 5: upgrade to WebSocket; roll back Gateway session and presence on failure.
	// 阶段 5：升级为 WebSocket；失败时回滚 Gateway 会话与 presence。
	ws, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.closeGatewaySession(endpoint.gateway, t, connID, "websocket_upgrade_failed")
		p.removePresence(t, connID)
		return
	}
	defer ws.Close()
	defer p.removePresence(t, connID)

	// Phase 6: configure keepalive and send the first server frame to the client.
	// 阶段 6：配置保活，并向客户端发送首帧 session.connected。
	ws.SetReadLimit(p.readLimitBytes)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	if err := writeJSON(ws, map[string]any{
		"type": "session.connected",
		"payload": map[string]any{
			"session_id": t.SessionID,
			"conn_id":    connID,
			"edge_id":    p.edgeID,
			"gateway_id": endpoint.gatewayID,
		},
	}); err != nil {
		return
	}

	// Phase 7: start the proxy pipeline.
	//   readWebSocket  -> inbound  -> runGateway -> outbound -> writeWebSocket
	// runGateway also handles gRPC Stream, rebind, and hub-driven gateway migration.
	// 阶段 7：启动代理流水线。
	//   readWebSocket -> inbound -> runGateway -> outbound -> writeWebSocket
	// runGateway 还负责 gRPC Stream、重绑与 hub 触发的 Gateway 迁移。
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	outbound := make(chan []byte, p.writeBufferSize)
	inbound := make(chan *pb.GatewayFrame, p.writeBufferSize)
	control := make(chan controlMessage, 4)
	errCh := make(chan error, 3)
	p.hub.RegisterConnection(connID, t.SessionID, endpoint.gatewayID, outbound, control)
	defer p.hub.UnregisterConnection(connID, t.SessionID)

	go p.readWebSocket(ctx, ws, inbound, t, connID, errCh)
	go p.runGateway(ctx, gatewaySession{
		ticket:   t,
		connID:   connID,
		endpoint: endpoint,
	}, inbound, outbound, control, errCh)
	go p.writeWebSocket(ctx, ws, outbound, errCh)

	// Phase 8: block until any leg fails; cancel siblings and close abnormally if needed.
	// 阶段 8：阻塞直至任一协程出错；取消其余协程，必要时发送异常关闭帧。
	err = <-errCh
	cancel()
	if err != nil && !isExpectedClose(err) {
		_ = writeClose(ws, "WebSocket proxy closed")
	}
}

func (p *Proxy) Deliver(ctx context.Context, target PushTarget, payload []byte) bool {
	return p.hub.Deliver(ctx, target, payload)
}

func (p *Proxy) MigrateGateway(gatewayID, reason string) int {
	return p.hub.MigrateGateway(gatewayID, reason)
}

func (p *Proxy) pickGateway(ctx context.Context, excludedGatewayIDs ...string) (upstreamEndpoint, error) {
	gateway, gatewayID, err := p.gatewayRouter.Pick(ctx, excludedGatewayIDs...)
	if err != nil {
		return upstreamEndpoint{}, err
	}
	return upstreamEndpoint{gateway: gateway, gatewayID: gatewayID}, nil
}

func (p *Proxy) attach(ctx context.Context, gateway gatewayservice.GatewayService, t ticket.Ticket, connID string) (*gatewayservice.AttachResponse, error) {
	attachCtx, cancel := context.WithTimeout(ctx, attachTimeout)
	defer cancel()

	return gateway.Attach(attachCtx, &gatewayservice.AttachRequest{
		SessionId: t.SessionID,
		ConnId:    connID,
		EdgeId:    p.edgeID,
		UserId:    t.UserID,
		DeviceId:  t.DeviceID,
	})
}

func (p *Proxy) resume(ctx context.Context, endpoint upstreamEndpoint, state gatewaySession) (*gatewayservice.ResumeResponse, error) {
	resumeCtx, cancel := context.WithTimeout(ctx, rebindTimeout)
	defer cancel()

	return endpoint.gateway.Resume(resumeCtx, &gatewayservice.ResumeRequest{
		SessionId:        state.ticket.SessionID,
		ConnId:           state.connID,
		EdgeId:           p.edgeID,
		UserId:           state.ticket.UserID,
		DeviceId:         state.ticket.DeviceID,
		LastClientSeq:    state.lastClientSeq,
		LastDeliveredSeq: state.lastDeliveredSeq,
	})
}

func (p *Proxy) removePresence(t ticket.Ticket, connID string) {
	ctx, cancel := context.WithTimeout(context.Background(), attachTimeout)
	defer cancel()

	_ = p.presence.RemoveConnection(ctx, presence.ConnectionMeta{
		SessionID: t.SessionID,
		ConnID:    connID,
		EdgeID:    p.edgeID,
		UserID:    t.UserID,
		DeviceID:  t.DeviceID,
	})
}

func (p *Proxy) closeGatewaySession(gateway gatewayservice.GatewayService, t ticket.Ticket, connID string, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), attachTimeout)
	defer cancel()

	if gateway != nil {
		_, _ = gateway.CloseSession(ctx, &gatewayservice.CloseSessionRequest{
			SessionId: t.SessionID,
			ConnId:    connID,
			EdgeId:    p.edgeID,
			Reason:    reason,
		})
	}
}

func (p *Proxy) readWebSocket(ctx context.Context, ws *websocket.Conn, inbound chan<- *pb.GatewayFrame, t ticket.Ticket, connID string, errCh chan<- error) {
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}

		var env clientEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			errCh <- fmt.Errorf("decode websocket frame: %w", err)
			return
		}
		if env.Type == "" {
			errCh <- errors.New("websocket frame type is required")
			return
		}

		frame := &pb.GatewayFrame{
			SessionId:   t.SessionID,
			ConnId:      connID,
			FrameType:   env.Type,
			PayloadJson: string(data),
			ClientSeq:   env.Seq,
		}

		if err := p.presence.RefreshConnection(ctx, presence.ConnectionMeta{
			SessionID: t.SessionID,
			ConnID:    connID,
			EdgeID:    p.edgeID,
			UserID:    t.UserID,
			DeviceID:  t.DeviceID,
		}); err != nil {
			errCh <- err
			return
		}

		select {
		case inbound <- frame:
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
			errCh <- errors.New("gateway inbound buffer is full")
			return
		}

		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}
	}
}

func (p *Proxy) runGateway(ctx context.Context, state gatewaySession, inbound <-chan *pb.GatewayFrame, outbound chan<- []byte, control <-chan controlMessage, errCh chan<- error) {
	stream, streamCancel, err := p.openGatewayStream(ctx, state.endpoint.gateway)
	if err != nil {
		p.closeGatewaySession(state.endpoint.gateway, state.ticket, state.connID, "gateway_stream_initial_failed")
		stream, streamCancel, err = p.rebindGateway(ctx, &state, outbound, state.endpoint.gatewayID, "gateway_stream_initial_failed")
		if err != nil {
			errCh <- err
			return
		}
	}
	recvCh := startGatewayReceiver(stream)
	defer func() {
		if streamCancel != nil {
			streamCancel()
		}
		if stream != nil {
			_ = stream.CloseSend()
		}
		p.closeGatewaySession(state.endpoint.gateway, state.ticket, state.connID, "edge_websocket_closed")
	}()

	for {
		select {
		case frame := <-inbound:
			if frame == nil {
				continue
			}
			if err := stream.Send(frame); err != nil {
				streamCancel()
				_ = stream.CloseSend()
				p.closeGatewaySession(state.endpoint.gateway, state.ticket, state.connID, "gateway_stream_send_failed")
				stream, streamCancel, err = p.rebindGateway(ctx, &state, outbound, state.endpoint.gatewayID, "gateway_stream_send_failed")
				if err != nil {
					errCh <- err
					return
				}
				recvCh = startGatewayReceiver(stream)
				if err := stream.Send(frame); err != nil {
					errCh <- fmt.Errorf("gateway stream send after rebind: %w", err)
					return
				}
			}
			if frame.GetClientSeq() > state.lastClientSeq {
				state.lastClientSeq = frame.GetClientSeq()
			}
		case recv := <-recvCh:
			if recv.err != nil {
				if ctx.Err() != nil {
					return
				}
				streamCancel()
				_ = stream.CloseSend()
				p.closeGatewaySession(state.endpoint.gateway, state.ticket, state.connID, "gateway_stream_receive_failed")
				stream, streamCancel, err = p.rebindGateway(ctx, &state, outbound, state.endpoint.gatewayID, "gateway_stream_receive_failed")
				if err != nil {
					errCh <- err
					return
				}
				recvCh = startGatewayReceiver(stream)
				continue
			}
			if recv.frame.GetServerSeq() > state.lastDeliveredSeq {
				state.lastDeliveredSeq = recv.frame.GetServerSeq()
			}
			if err := sendOutbound(ctx, outbound, gatewayFramePayload(recv.frame)); err != nil {
				errCh <- err
				return
			}
			recvCh = startGatewayReceiver(stream)
		case cmd := <-control:
			if cmd.Kind != controlMigrateGateway || cmd.GatewayID == "" || cmd.GatewayID != state.endpoint.gatewayID {
				continue
			}
			streamCancel()
			_ = stream.CloseSend()
			p.closeGatewaySession(state.endpoint.gateway, state.ticket, state.connID, "gateway_migration_requested")
			stream, streamCancel, err = p.rebindGateway(ctx, &state, outbound, cmd.GatewayID, cmd.Reason)
			if err != nil {
				errCh <- err
				return
			}
			recvCh = startGatewayReceiver(stream)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Proxy) openGatewayStream(ctx context.Context, gateway gatewayservice.GatewayService) (pb.GatewayService_StreamClient, context.CancelFunc, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := gateway.Stream(streamCtx)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return stream, cancel, nil
}

func startGatewayReceiver(stream pb.GatewayService_StreamClient) <-chan gatewayRecvResult {
	ch := make(chan gatewayRecvResult, 1)
	go func() {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			ch <- gatewayRecvResult{err: err}
			return
		}
		ch <- gatewayRecvResult{frame: frame, err: err}
	}()
	return ch
}

func (p *Proxy) rebindGateway(ctx context.Context, state *gatewaySession, outbound chan<- []byte, failedGatewayID string, reason string) (pb.GatewayService_StreamClient, context.CancelFunc, error) {
	rebindCtx, cancel := context.WithTimeout(ctx, p.recovery.Window)
	defer cancel()

	excluded := newGatewayIDSet(failedGatewayID)
	p.gatewayRouter.MarkFailed(failedGatewayID)
	var lastErr error

	for attempt := 0; attempt < p.recovery.MaxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(rebindCtx, p.recovery.backoff(attempt-1)); err != nil {
				return nil, nil, err
			}
		}

		endpoint, err := p.pickGateway(rebindCtx, excluded.Values()...)
		if err != nil {
			lastErr = fmt.Errorf("gateway rebind pick failed: %w", err)
			continue
		}

		stream, streamCancel, err := p.tryRebindEndpoint(rebindCtx, ctx, state, endpoint, outbound)
		if err == nil {
			return stream, streamCancel, nil
		}

		gatewayID := endpoint.gatewayID
		excluded.Add(gatewayID)
		p.gatewayRouter.MarkFailed(gatewayID)
		p.closeGatewaySession(endpoint.gateway, state.ticket, state.connID, reason)
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("no gateway rebind attempt executed")
	}
	return nil, nil, fmt.Errorf("gateway rebind exhausted: %w", lastErr)
}

func (p *Proxy) tryRebindEndpoint(operationCtx context.Context, streamCtx context.Context, state *gatewaySession, endpoint upstreamEndpoint, outbound chan<- []byte) (pb.GatewayService_StreamClient, context.CancelFunc, error) {
	resume, err := p.resume(operationCtx, endpoint, *state)
	if err != nil {
		return nil, nil, fmt.Errorf("gateway resume failed: %w", err)
	}
	if !resume.GetAccepted() {
		return nil, nil, fmt.Errorf("gateway resume rejected: %s %s", resume.GetErrorCode(), resume.GetMessage())
	}

	endpoint.gatewayID = gatewayID(resume.GetGatewayId(), endpoint.gatewayID)
	if resume.GetLastDeliveredSeq() > state.lastDeliveredSeq {
		state.lastDeliveredSeq = resume.GetLastDeliveredSeq()
	}

	stream, streamCancel, err := p.openGatewayStream(streamCtx, endpoint.gateway)
	if err != nil {
		return nil, nil, fmt.Errorf("gateway stream reopen failed: %w", err)
	}
	if err := p.presence.RebindGateway(operationCtx, presence.ConnectionMeta{
		SessionID: state.ticket.SessionID,
		ConnID:    state.connID,
		EdgeID:    p.edgeID,
		UserID:    state.ticket.UserID,
		DeviceID:  state.ticket.DeviceID,
		GatewayID: endpoint.gatewayID,
	}); err != nil {
		streamCancel()
		_ = stream.CloseSend()
		return nil, nil, err
	}

	oldEndpoint := state.endpoint
	state.endpoint = endpoint
	p.hub.UpdateGateway(state.connID, endpoint.gatewayID)
	if err := sendOutbound(operationCtx, outbound, p.sessionResumedPayload(*state)); err != nil {
		streamCancel()
		_ = stream.CloseSend()
		state.endpoint = oldEndpoint
		p.hub.UpdateGateway(state.connID, oldEndpoint.gatewayID)
		return nil, nil, err
	}
	return stream, streamCancel, nil
}

func (p *Proxy) writeWebSocket(ctx context.Context, ws *websocket.Conn, outbound <-chan []byte, errCh chan<- error) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case data := <-outbound:
			ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
				errCh <- err
				return
			}
		case <-ticker.C:
			ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				errCh <- err
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func sendOutbound(ctx context.Context, outbound chan<- []byte, data []byte) error {
	select {
	case outbound <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errOutboundBufferFull
	}
}

func gatewayFramePayload(frame *pb.GatewayFrame) []byte {
	data := []byte(frame.GetPayloadJson())
	if len(data) == 0 {
		data, _ = json.Marshal(map[string]any{
			"type":       frame.GetFrameType(),
			"server_seq": frame.GetServerSeq(),
		})
	}
	return data
}

func (p *Proxy) sessionResumedPayload(state gatewaySession) []byte {
	data, _ := json.Marshal(map[string]any{
		"type": "session.resumed",
		"payload": map[string]any{
			"session_id":         state.ticket.SessionID,
			"conn_id":            state.connID,
			"edge_id":            p.edgeID,
			"gateway_id":         state.endpoint.gatewayID,
			"last_client_seq":    state.lastClientSeq,
			"last_delivered_seq": state.lastDeliveredSeq,
		},
	})
	return data
}

func writeJSON(ws *websocket.Conn, payload any) error {
	ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return ws.WriteJSON(payload)
}

func writeClose(ws *websocket.Conn, reason string) error {
	msg := websocket.FormatCloseMessage(websocket.CloseTryAgainLater, reason)
	ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return ws.WriteMessage(websocket.CloseMessage, msg)
}

func isExpectedClose(err error) bool {
	return errors.Is(err, context.Canceled) ||
		websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived)
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

type staticGatewayRouter struct {
	gateway gatewayservice.GatewayService
}

func (r staticGatewayRouter) Pick(ctx context.Context, excludedGatewayIDs ...string) (gatewayservice.GatewayService, string, error) {
	if r.gateway == nil {
		return nil, "", errors.New("gateway client is unavailable")
	}
	return r.gateway, "", nil
}

func (r staticGatewayRouter) MarkFailed(gatewayID string) {
}

func gatewayID(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func normalizeRecoveryConfig(c RecoveryConfig) RecoveryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.Window <= 0 {
		c.Window = defaultRecoveryWindow
	}
	if c.Isolation <= 0 {
		c.Isolation = defaultIsolation
	}
	if len(c.Backoffs) == 0 {
		c.Backoffs = []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	}
	return c
}

func (c RecoveryConfig) backoff(index int) time.Duration {
	if len(c.Backoffs) == 0 {
		return 0
	}
	if index < len(c.Backoffs) {
		return c.Backoffs[index]
	}
	return c.Backoffs[len(c.Backoffs)-1]
}

func sleepBackoff(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type gatewayIDSet map[string]struct{}

func newGatewayIDSet(ids ...string) gatewayIDSet {
	set := gatewayIDSet{}
	for _, id := range ids {
		set.Add(id)
	}
	return set
}

func (s gatewayIDSet) Add(id string) {
	if id != "" {
		s[id] = struct{}{}
	}
}

func (s gatewayIDSet) Values() []string {
	values := make([]string, 0, len(s))
	for id := range s {
		values = append(values, id)
	}
	return values
}
