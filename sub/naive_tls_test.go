package sub

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestGetOutboundsNormalizesStoredNaiveTLS(t *testing.T) {
	for _, protocol := range []string{"naive", "trojan"} {
		t.Run(protocol, func(t *testing.T) {
			out, err := json.Marshal(map[string]any{
				"type": protocol, "tag": "fixture", "server": "proxy.example.com", "server_port": 443,
				"tls": map[string]any{"enabled": true, "server_name": "proxy.example.com", "alpn": []string{"h2"}, "insecure": true},
			})
			if err != nil {
				t.Fatal(err)
			}
			credentials, err := json.Marshal(map[string]any{protocol: map[string]string{"name": "fixture", "password": "fixture-password"}})
			if err != nil {
				t.Fatal(err)
			}
			inbound := &model.Inbound{Type: protocol, Tag: "fixture", OutJson: out, Addrs: json.RawMessage(`[]`)}
			outbounds, _, err := (&JsonService{}).getOutbounds(credentials, []*model.Inbound{inbound}, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(*outbounds) != 1 {
				t.Fatalf("expected one outbound, got %d", len(*outbounds))
			}
			tls, _ := (*outbounds)[0]["tls"].(map[string]any)
			if tls["enabled"] != true || tls["server_name"] != "proxy.example.com" || (*outbounds)[0]["password"] != "fixture-password" {
				t.Fatal("normalization lost supported TLS settings or client credentials")
			}
			for _, key := range []string{"alpn", "insecure"} {
				_, present := tls[key]
				if present != (protocol == "trojan") {
					t.Fatalf("unexpected %s presence for %s", key, protocol)
				}
			}
		})
	}
}
