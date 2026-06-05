package presence

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestPresenceRegisterRefreshAndUnregister(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	service, err := NewService(client, "instance-a", 30*time.Second)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	session := Session{
		UserID:    "42",
		SessionID: "session-1",
		DeviceID:  "device-1",
		Platform:  "web",
	}
	if err := service.Register(context.Background(), session); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	instances, err := service.InstancesForUser(context.Background(), "42")
	if err != nil {
		t.Fatalf("InstancesForUser() error = %v", err)
	}
	if len(instances) != 1 || instances[0] != "instance-a" {
		t.Fatalf("instances = %#v, want instance-a", instances)
	}
	if !redisServer.Exists(SessionKey("session-1")) {
		t.Fatal("session presence key missing")
	}

	if err := service.Refresh(context.Background(), session); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if err := service.Unregister(context.Background(), session); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	instances, err = service.InstancesForUser(context.Background(), "42")
	if err != nil {
		t.Fatalf("InstancesForUser() after unregister error = %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("instances after unregister = %#v, want empty", instances)
	}
}

func TestPresenceKeepsInstanceWhileAnotherSessionExists(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	service, err := NewService(client, "instance-a", 30*time.Second)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	first := Session{UserID: "42", SessionID: "session-1"}
	second := Session{UserID: "42", SessionID: "session-2"}
	if err := service.Register(context.Background(), first); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := service.Register(context.Background(), second); err != nil {
		t.Fatalf("Register(second) error = %v", err)
	}
	if err := service.Unregister(context.Background(), first); err != nil {
		t.Fatalf("Unregister(first) error = %v", err)
	}

	instances, err := service.InstancesForUser(context.Background(), "42")
	if err != nil {
		t.Fatalf("InstancesForUser() error = %v", err)
	}
	if len(instances) != 1 || instances[0] != "instance-a" {
		t.Fatalf("instances = %#v, want instance-a", instances)
	}
}
