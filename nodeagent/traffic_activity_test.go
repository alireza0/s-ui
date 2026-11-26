package nodeagent

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
)

func TestActivityIsSampledWithoutControllerAndSamplesAreIndependent(t *testing.T) {
	runtime := NewCoreRuntime()
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	body, _ := json.Marshal(map[string]any{
		"inbounds": []any{map[string]any{
			"type": "vless", "tag": "vpn", "listen": "127.0.0.1", "listen_port": port,
			"users": []any{map[string]string{"name": "client/1", "uuid": "11111111-1111-4111-8111-111111111111"}},
		}},
		"outbounds": []any{map[string]string{"type": "direct", "tag": "direct"}},
	})
	if err := runtime.ApplyWithAccess(body, AccessSnapshot{Clients: []Grant{{Name: "client/1", TrafficID: "1:0"}}}); err != nil {
		t.Fatal(err)
	}
	conn, peer := net.Pipe()
	defer peer.Close()
	tracked := runtime.core.GetInstance().SessionTracker().RoutedConnection(context.Background(), conn, adapter.InboundContext{Inbound: "vpn", User: "client/1"}, nil, nil)
	defer tracked.Close()
	done := make(chan struct{})
	go func() { defer close(done); buf := make([]byte, 4); _, _ = peer.Read(buf) }()
	tracked.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := tracked.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	<-done
	runtime.SampleTraffic()
	first := runtime.Traffic()
	if first == nil || first.Activity["client/1"] == 0 {
		t.Fatal("offline traffic did not retain user activity")
	}
	recorded := first.Activity["client/1"]
	if usage := first.Users["1:0"]; usage.Up+usage.Down != 4 {
		t.Fatal("routed bytes did not reach the client's quota counter", usage)
	}
	first.Activity["client/1"] = 0
	second := runtime.Traffic()
	if second.Activity["client/1"] != recorded {
		t.Fatal("status caller could corrupt retained activity")
	}
}
