package logic

import (
	"context"
	"testing"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/pb"
	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestPresenceLogicUpsertAndGetLiveRoutes(t *testing.T) {
	ctx := context.Background()
	svcCtx, closeSvc := newTestServiceContext(t)
	defer closeSvc()

	upsert := NewUpsertConnectionLogic(ctx, svcCtx)
	resp, err := upsert.UpsertConnection(&pb.UpsertConnectionRequest{
		Connection: &pb.ConnectionMeta{
			SessionId: "session-1",
			ConnId:    "conn-1",
			EdgeId:    "edge-1",
			UserId:    "user-1",
			DeviceId:  "web-1",
			GatewayId: "gateway-1",
		},
		TtlSeconds: 60,
	})
	if err != nil {
		t.Fatalf("UpsertConnection() error = %v", err)
	}
	if !resp.GetAccepted() {
		t.Fatalf("Accepted = false, response = %+v", resp)
	}

	getRoutes := NewGetLiveRoutesLogic(ctx, svcCtx)
	routes, err := getRoutes.GetLiveRoutes(&pb.GetLiveRoutesRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetLiveRoutes() error = %v", err)
	}
	if len(routes.GetRoutes()) != 1 || routes.GetRoutes()[0].GetGatewayId() != "gateway-1" {
		t.Fatalf("routes = %+v", routes.GetRoutes())
	}
}

func newTestServiceContext(t *testing.T) (*svc.ServiceContext, func()) {
	t.Helper()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	return &svc.ServiceContext{
			Redis: client,
			Store: store.New(client, time.Minute),
		}, func() {
			_ = client.Close()
			server.Close()
		}
}
