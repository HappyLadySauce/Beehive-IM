package etcd

import "testing"

func TestNormalizeUsesEnvEndpoints(t *testing.T) {
	t.Setenv("ETCD_ENDPOINTS", "127.0.0.1:2379,127.0.0.2:2379")
	cfg := Config{DisableDefaultEndpoint: true}.Normalize()
	if len(cfg.Endpoints) != 2 {
		t.Fatalf("Endpoints len = %d, want 2", len(cfg.Endpoints))
	}
}

func TestServiceKey(t *testing.T) {
	got := ServiceKey("/beehive-im", "dev", "gateway", "gateway-1")
	want := "/beehive-im/dev/services/gateway/gateway-1"
	if got != want {
		t.Fatalf("ServiceKey() = %q, want %q", got, want)
	}
}

func TestEncodeDecodeNode(t *testing.T) {
	node := ServiceNode{InstanceID: "gw-1", Service: "gateway", Address: "127.0.0.1:9100", Status: "online"}
	data, err := EncodeNode(node)
	if err != nil {
		t.Fatalf("EncodeNode() error = %v", err)
	}
	decoded, err := DecodeNode([]byte(data))
	if err != nil {
		t.Fatalf("DecodeNode() error = %v", err)
	}
	if decoded.InstanceID != node.InstanceID || decoded.SchemaVersion != 1 {
		t.Fatalf("decoded node = %+v", decoded)
	}
}
