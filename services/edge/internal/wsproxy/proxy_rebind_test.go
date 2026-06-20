package wsproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/presence"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRunGatewayRebindsAfterReceiveFailure(t *testing.T) {
	oldGateway := newFakeGateway("gateway-a")
	newGateway := newFakeGateway("gateway-b")
	router := &fakeGatewayRouter{
		endpoints: []fakeGatewayEndpoint{
			{id: "gateway-b", gateway: newGateway},
		},
	}
	presenceClient := &recordingPresence{}
	proxy := NewProxy(Config{
		EdgeID:        "edge-1",
		GatewayRouter: router,
		Presence:      presenceClient,
		Tickets:       ticket.NewStore(time.Minute),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inbound := make(chan *pb.GatewayFrame, 1)
	outbound := make(chan []byte, 2)
	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		proxy.runGateway(ctx, gatewaySession{
			ticket: ticket.Ticket{
				SessionID: "session-1",
				UserID:    "user-1",
				DeviceID:  "device-1",
			},
			connID: "conn-1",
			endpoint: upstreamEndpoint{
				gateway:   oldGateway,
				gatewayID: "gateway-a",
			},
			lastClientSeq:    7,
			lastDeliveredSeq: 3,
		}, inbound, outbound, errCh)
	}()

	oldStream := oldGateway.waitStream(t)
	oldStream.recv <- gatewayRecvResult{err: io.ErrUnexpectedEOF}
	newGateway.waitStream(t)

	select {
	case data := <-outbound:
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("session.resumed payload decode error = %v", err)
		}
		if payload["type"] != "session.resumed" {
			t.Fatalf("payload type = %v, want session.resumed", payload["type"])
		}
	case err := <-errCh:
		t.Fatalf("runGateway() error = %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session.resumed")
	}

	if newGateway.resumeCount() != 1 {
		t.Fatalf("Resume() calls = %d, want 1", newGateway.resumeCount())
	}
	if presenceClient.gatewayID() != "gateway-b" {
		t.Fatalf("presence rebind gateway = %q, want gateway-b", presenceClient.gatewayID())
	}
	if !router.excluded("gateway-a") {
		t.Fatal("router did not receive failed gateway exclusion")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runGateway() did not stop after context cancellation")
	}
}

type fakeGatewayEndpoint struct {
	id      string
	gateway gatewayservice.GatewayService
}

type fakeGatewayRouter struct {
	mu        sync.Mutex
	endpoints []fakeGatewayEndpoint
	excludes  []string
}

func (r *fakeGatewayRouter) Pick(ctx context.Context, excludedGatewayIDs ...string) (gatewayservice.GatewayService, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.excludes = append(r.excludes, excludedGatewayIDs...)
	if len(r.endpoints) == 0 {
		return nil, "", errors.New("no fake gateway")
	}
	endpoint := r.endpoints[0]
	r.endpoints = r.endpoints[1:]
	return endpoint.gateway, endpoint.id, nil
}

func (r *fakeGatewayRouter) excluded(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, excluded := range r.excludes {
		if excluded == id {
			return true
		}
	}
	return false
}

type fakeGateway struct {
	gatewayservice.GatewayService
	id             string
	streams        chan *scriptedGatewayStream
	mu             sync.Mutex
	resumeRequests []*gatewayservice.ResumeRequest
}

func newFakeGateway(id string) *fakeGateway {
	return &fakeGateway{
		id:      id,
		streams: make(chan *scriptedGatewayStream, 4),
	}
}

func (g *fakeGateway) Resume(ctx context.Context, in *gatewayservice.ResumeRequest, opts ...grpc.CallOption) (*gatewayservice.ResumeResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.resumeRequests = append(g.resumeRequests, in)
	return &gatewayservice.ResumeResponse{
		Accepted:         true,
		GatewayId:        g.id,
		LastDeliveredSeq: in.GetLastDeliveredSeq(),
		Message:          "resumed",
	}, nil
}

func (g *fakeGateway) CloseSession(ctx context.Context, in *gatewayservice.CloseSessionRequest, opts ...grpc.CallOption) (*gatewayservice.CloseSessionResponse, error) {
	return &gatewayservice.CloseSessionResponse{Closed: true}, nil
}

func (g *fakeGateway) Stream(ctx context.Context, opts ...grpc.CallOption) (pb.GatewayService_StreamClient, error) {
	stream := newScriptedGatewayStream(ctx)
	g.streams <- stream
	return stream, nil
}

func (g *fakeGateway) waitStream(t *testing.T) *scriptedGatewayStream {
	t.Helper()

	select {
	case stream := <-g.streams:
		return stream
	case <-time.After(time.Second):
		t.Fatalf("gateway %s stream was not opened", g.id)
		return nil
	}
}

func (g *fakeGateway) resumeCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	return len(g.resumeRequests)
}

type scriptedGatewayStream struct {
	ctx    context.Context
	cancel context.CancelFunc
	recv   chan gatewayRecvResult
}

func newScriptedGatewayStream(parent context.Context) *scriptedGatewayStream {
	ctx, cancel := context.WithCancel(parent)
	return &scriptedGatewayStream{
		ctx:    ctx,
		cancel: cancel,
		recv:   make(chan gatewayRecvResult, 4),
	}
}

func (s *scriptedGatewayStream) Send(frame *pb.GatewayFrame) error {
	return nil
}

func (s *scriptedGatewayStream) Recv() (*pb.GatewayFrame, error) {
	select {
	case result := <-s.recv:
		return result.frame, result.err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *scriptedGatewayStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *scriptedGatewayStream) Trailer() metadata.MD {
	return nil
}

func (s *scriptedGatewayStream) CloseSend() error {
	s.cancel()
	return nil
}

func (s *scriptedGatewayStream) Context() context.Context {
	return s.ctx
}

func (s *scriptedGatewayStream) SendMsg(m any) error {
	return nil
}

func (s *scriptedGatewayStream) RecvMsg(m any) error {
	return nil
}

type recordingPresence struct {
	mu          sync.Mutex
	reboundConn presence.ConnectionMeta
}

func (p *recordingPresence) UpsertConnection(ctx context.Context, conn presence.ConnectionMeta) error {
	return nil
}

func (p *recordingPresence) RefreshConnection(ctx context.Context, conn presence.ConnectionMeta) error {
	return nil
}

func (p *recordingPresence) RebindGateway(ctx context.Context, conn presence.ConnectionMeta) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.reboundConn = conn
	return nil
}

func (p *recordingPresence) RemoveConnection(ctx context.Context, conn presence.ConnectionMeta) error {
	return nil
}

func (p *recordingPresence) gatewayID() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.reboundConn.GatewayID
}
