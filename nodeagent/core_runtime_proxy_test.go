package nodeagent

import (
	"bytes"
	"encoding/json"
	"maps"
	"net"
	"testing"
)

func TestShadowTLSUnchangedUsersAndChangedUsersUseSafePaths(t *testing.T) {
	port := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		value := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return value
	}
	vlessPort, shadowPort := port(), port()
	for shadowPort == vlessPort {
		shadowPort = port()
	}
	config := func(uuid, password string) []byte {
		body, err := json.Marshal(map[string]any{
			"inbounds": []any{
				map[string]any{"type": "vless", "tag": "vpn", "listen": "127.0.0.1", "listen_port": vlessPort, "users": []any{map[string]string{"name": "client/1", "uuid": uuid}}},
				map[string]any{"type": "shadowtls", "tag": "transport", "listen": "127.0.0.1", "listen_port": shadowPort, "version": 3, "detour": "vpn", "handshake": map[string]any{"server": "127.0.0.1", "server_port": 443}, "users": []any{map[string]string{"name": "client/1", "password": password}}},
			},
			"outbounds": []any{map[string]string{"type": "direct", "tag": "direct"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	runtime := NewCoreRuntime()
	defer runtime.Close()
	first := config("11111111-1111-4111-8111-111111111111", "unit-old-password")
	if err := runtime.ApplySafe(first); err != nil {
		t.Fatal(err)
	}
	initial := runtime.core.GetInstance()
	if err := runtime.ApplySafe(first); err != nil || runtime.core.GetInstance() != initial {
		t.Fatal("unchanged ShadowTLS users restarted or rejected the core", err)
	}
	second := config("22222222-2222-4222-8222-222222222222", "unit-old-password")
	if err := runtime.ApplySafe(second); err != nil || runtime.core.GetInstance() != initial {
		t.Fatal("unchanged transport blocked a supported VLESS user update", err)
	}
	third := config("22222222-2222-4222-8222-222222222222", "unit-new-password")
	if err := runtime.ApplySafe(third); err != nil || !runtime.Running() || runtime.core.GetInstance() == initial {
		t.Fatal("changed transport users did not rebuild the core", err)
	}
	if !bytes.Equal(runtime.config, third) || bytes.Contains(runtime.config, []byte("unit-old-password")) {
		t.Fatal("rebuild retained the previous transport credential")
	}
}

func TestUsernameFingerprintsKeepEveryProxyUser(t *testing.T) {
	first, err := userFingerprints([]byte(`{"inbounds":[{"tag":"auth","users":[{"username":"first","password":"old"},{"username":"second","password":"unchanged"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := userFingerprints([]byte(`{"inbounds":[{"tag":"auth","users":[{"username":"first","password":"new"},{"username":"second","password":"unchanged"}]}]}`))
	if err != nil || len(first["auth"]) != 2 || maps.Equal(first["auth"], second["auth"]) {
		t.Fatal("changing an earlier proxy user's password was lost", err)
	}
}
