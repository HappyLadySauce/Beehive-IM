package wsproxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	writeTimeout           = 5 * time.Second
)

type Proxy struct {
	edgeID          string
	writeBufferSize int
	readLimitBytes  int64
	tickets         *ticket.Store
	gatewayRouter   GatewayRouter
	presence        presence.Client
	hub             *Hub
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
}

type GatewayRouter interface {
	Pick(ctx context.Context) (gatewayservice.GatewayService, string, error)
}

type clientEnvelope struct {
	Type string          `json:"type"`
	Seq  int64           `json:"seq"`
	Data json.RawMessage `json:"payload"`
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

	return &Proxy{
		edgeID:          c.EdgeID,
		writeBufferSize: c.WriteBufferSize,
		readLimitBytes:  c.ReadLimitBytes,
		tickets:         c.Tickets,
		gatewayRouter:   c.GatewayRouter,
		presence:        c.Presence,
		hub:             NewHub(),
		upgrader: websocket.Upgrader{
			Subprotocols: []string{"beehive.im.v1"},
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t, err := p.tickets.Consume(r.URL.Query().Get("ticket"), r.Header.Get("Origin"))
	if err != nil {
		http.Error(w, "Invalid websocket ticket", http.StatusUnauthorized)
		return
	}

	connID := "conn-" + randomToken(12)
	if p.gatewayRouter == nil {
		http.Error(w, "Gateway client is unavailable", http.StatusServiceUnavailable)
		return
	}

	gateway, selectedGatewayID, err := p.gatewayRouter.Pick(r.Context())
	if err != nil {
		http.Error(w, "Gateway selection failed", http.StatusServiceUnavailable)
		return
	}
	attach, err := p.attach(r.Context(), gateway, t, connID)
	if err != nil {
		http.Error(w, "Gateway attach failed", http.StatusServiceUnavailable)
		return
	}
	if !attach.GetAccepted() {
		http.Error(w, attach.GetErrorCode(), http.StatusServiceUnavailable)
		return
	}

	if err := p.presence.UpsertConnection(r.Context(), presence.ConnectionMeta{
		SessionID: t.SessionID,
		ConnID:    connID,
		EdgeID:    p.edgeID,
		UserID:    t.UserID,
		DeviceID:  t.DeviceID,
		GatewayID: gatewayID(attach.GetGatewayId(), selectedGatewayID),
	}); err != nil {
		p.closeGatewaySession(gateway, t, connID, "presence_upsert_failed")
		http.Error(w, "Presence upsert failed", http.StatusServiceUnavailable)
		return
	}

	ws, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.cleanup(gateway, t, connID)
		return
	}
	defer ws.Close()
	defer p.cleanup(gateway, t, connID)

	ws.SetReadLimit(p.readLimitBytes)
	if err := writeJSON(ws, map[string]any{
		"type": "session.connected",
		"payload": map[string]any{
			"session_id": t.SessionID,
			"conn_id":    connID,
			"edge_id":    p.edgeID,
			"gateway_id": gatewayID(attach.GetGatewayId(), selectedGatewayID),
		},
	}); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream, err := gateway.Stream(ctx)
	if err != nil {
		_ = writeClose(ws, "Gateway stream failed")
		return
	}
	defer stream.CloseSend()

	outbound := make(chan []byte, p.writeBufferSize)
	errCh := make(chan error, 3)
	p.hub.Register(connID, t.SessionID, outbound)
	defer p.hub.Unregister(connID, t.SessionID)

	go p.readWebSocket(ctx, ws, stream, t, connID, errCh)
	go p.readGateway(ctx, stream, outbound, errCh)
	go p.writeWebSocket(ctx, ws, outbound, errCh)

	err = <-errCh
	cancel()
	if err != nil && !isExpectedClose(err) {
		_ = writeClose(ws, "WebSocket proxy closed")
	}
}

func (p *Proxy) Deliver(ctx context.Context, target PushTarget, payload []byte) bool {
	return p.hub.Deliver(ctx, target, payload)
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

func (p *Proxy) cleanup(gateway gatewayservice.GatewayService, t ticket.Ticket, connID string) {
	p.closeGatewaySession(gateway, t, connID, "edge_websocket_closed")
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

func (p *Proxy) readWebSocket(ctx context.Context, ws *websocket.Conn, stream pb.GatewayService_StreamClient, t ticket.Ticket, connID string, errCh chan<- error) {
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

		if err := stream.Send(&pb.GatewayFrame{
			SessionId:   t.SessionID,
			ConnId:      connID,
			FrameType:   env.Type,
			PayloadJson: string(data),
			ClientSeq:   env.Seq,
		}); err != nil {
			errCh <- err
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

func (p *Proxy) readGateway(ctx context.Context, stream pb.GatewayService_StreamClient, outbound chan<- []byte, errCh chan<- error) {
	for {
		frame, err := stream.Recv()
		if err != nil {
			errCh <- err
			return
		}

		data := []byte(frame.GetPayloadJson())
		if len(data) == 0 {
			data, _ = json.Marshal(map[string]any{
				"type":       frame.GetFrameType(),
				"server_seq": frame.GetServerSeq(),
			})
		}

		select {
		case outbound <- data:
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		}
	}
}

func (p *Proxy) writeWebSocket(ctx context.Context, ws *websocket.Conn, outbound <-chan []byte, errCh chan<- error) {
	for {
		select {
		case data := <-outbound:
			ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
				errCh <- err
				return
			}
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		}
	}
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

func (r staticGatewayRouter) Pick(ctx context.Context) (gatewayservice.GatewayService, string, error) {
	if r.gateway == nil {
		return nil, "", errors.New("gateway client is unavailable")
	}
	return r.gateway, "", nil
}

func gatewayID(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
