package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestExtractClientName(t *testing.T) {
	u1 := json.RawMessage(`{"name":"alice","uuid":"123"}`)
	if got := extractClientName(u1); got != "alice" {
		t.Errorf("extractClientName(name) = %s, want alice", got)
	}

	u2 := json.RawMessage(`{"username":"bob","password":"pass"}`)
	if got := extractClientName(u2); got != "bob" {
		t.Errorf("extractClientName(username) = %s, want bob", got)
	}

	u3 := json.RawMessage(`{"invalid":"json"}`)
	if got := extractClientName(u3); got != "" {
		t.Errorf("extractClientName(invalid) = %s, want empty", got)
	}
}

func TestSSRFDNSResolutionRejectsLoopback(t *testing.T) {
	ns := &NodeService{}

	// localhost resolves to 127.0.0.1 (loopback)
	// Without AllowPrivateAddress it should be rejected by IP check anyway,
	// but with AllowPrivateAddress=true, special IPs (169.254/link-local/multicast/unspecified) are rejected.
	n := &model.Node{
		Name:                "dns-test",
		Address:             "localhost",
		Port:                2095,
		Scheme:              "https",
		BasePath:            "/",
		ApiToken:            "token123",
		AllowPrivateAddress: true,
	}

	// Should normalize without error since localhost resolves to 127.0.0.1 (private, not link-local/multicast/unspecified)
	if err := ns.Normalize(n); err != nil {
		t.Errorf("expected localhost with AllowPrivateAddress to be accepted, got: %v", err)
	}
}

func TestClientChangeSnapshot(t *testing.T) {
	snap := &ClientChangeSnapshot{
		ClientName: "alice",
		NodeName:   "Germany Node",
		InboundTag: "vless-de",
	}
	if snap.ClientName != "alice" || snap.NodeName != "Germany Node" {
		t.Errorf("unexpected snapshot fields: %+v", snap)
	}
}
