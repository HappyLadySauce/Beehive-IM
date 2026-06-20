package store

import (
	"context"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestStoreUpsertGetRefreshRebindRemove(t *testing.T) {
	ctx := context.Background()
	store, closeStore := newTestStore(t)
	defer closeStore()

	conn := ConnectionMeta{
		SessionID: "session-1",
		ConnID:    "conn-1",
		EdgeID:    "edge-1",
		UserID:    "user-1",
		DeviceID:  "web-1",
		GatewayID: "gateway-1",
	}
	if err := store.Upsert(ctx, conn, time.Minute); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	routes, err := store.GetLiveRoutes(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetLiveRoutes() error = %v", err)
	}
	if len(routes) != 1 || routes[0].GatewayID != "gateway-1" {
		t.Fatalf("routes = %+v", routes)
	}

	if err := store.Refresh(ctx, "session-1", "conn-1", "edge-1", time.Minute); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if err := store.RebindGateway(ctx, "session-1", "conn-1", "edge-1", "gateway-2", time.Minute); err != nil {
		t.Fatalf("RebindGateway() error = %v", err)
	}
	routes, err = store.GetLiveRoutes(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetLiveRoutes() after rebind error = %v", err)
	}
	if routes[0].GatewayID != "gateway-2" {
		t.Fatalf("GatewayID = %q, want gateway-2", routes[0].GatewayID)
	}

	conn.GatewayID = "gateway-2"
	removed, err := store.Remove(ctx, conn)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !removed {
		t.Fatal("Remove() removed = false, want true")
	}
	routes, err = store.GetLiveRoutes(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetLiveRoutes() after remove error = %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("routes len = %d, want 0", len(routes))
	}
}

func TestStoreRejectsStaleRefresh(t *testing.T) {
	ctx := context.Background()
	store, closeStore := newTestStore(t)
	defer closeStore()

	conn := ConnectionMeta{SessionID: "session-1", ConnID: "conn-1", EdgeID: "edge-1", UserID: "user-1", DeviceID: "web-1"}
	if err := store.Upsert(ctx, conn, time.Minute); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	err := store.Refresh(ctx, "session-1", "conn-2", "edge-1", time.Minute)
	if !errors.Is(err, ErrStaleConnection) {
		t.Fatalf("Refresh() error = %v, want %v", err, ErrStaleConnection)
	}
}

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	return New(client, time.Minute), func() {
		_ = client.Close()
		server.Close()
	}
}
