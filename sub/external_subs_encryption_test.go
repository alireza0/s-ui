package sub

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestExternalVLESSClashEncryption(t *testing.T) {
	openClashTestSettings(t)
	service := &ClashService{}
	const encrypted = "mlkem768x25519plus.native.1rtt.A+B/C=="
	nodes := []struct {
		name       string
		transport  string
		encryption string
	}{
		{name: "encrypted-xhttp", transport: "xhttp", encryption: encrypted},
		{name: "encrypted-ws", transport: "ws", encryption: encrypted},
		{name: "none", transport: "xhttp", encryption: "none"},
		{name: "absent", transport: "ws"},
	}
	var uris []string
	var links []Link
	for _, node := range nodes {
		query := url.Values{"type": {node.transport}, "security": {"tls"}, "sni": {"test.example"}, "host": {"test.example"}, "path": {"/upload"}}
		if node.encryption != "" {
			query.Set("encryption", node.encryption)
		}
		uri := "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:443?" + query.Encode() + "#" + node.name
		uris = append(uris, uri)
		links = append(links, Link{Type: "external", Uri: uri})
	}
	decoded := "\r\n  " + strings.Join(uris, "  \r\n\r\n  ") + "  \r\n"
	for _, format := range []string{"direct", "plaintext", "base64"} {
		t.Run(format, func(t *testing.T) {
			raw := externalSubTestLinks(t, links)
			wantRaw := uris
			if format != "direct" {
				body := decoded
				if format == "base64" {
					body = base64.StdEncoding.EncodeToString([]byte(body))
				}
				server := externalSubTestServer(t, body)
				raw = externalSubTestLinks(t, []Link{{Type: "sub", Uri: server.URL}})
				wantRaw = strings.Split(decoded, "\n")
			}
			original := append([]byte(nil), raw...)
			ordinary, ordinaryTags := service.GetExternalOutbounds(&raw)
			if len(ordinary) != len(nodes) {
				t.Fatalf("ordinary export has %d nodes, want %d", len(ordinary), len(nodes))
			}
			for _, outbound := range ordinary {
				if _, present := outbound["encryption"]; present {
					t.Fatal("Clash encryption leaked into ordinary sing-box export")
				}
			}
			outbounds, tags := service.GetExternalOutboundsForClash(&raw)
			proxies := externalSubTestProxies(t, service, outbounds)
			if len(proxies) != len(nodes) || !reflect.DeepEqual(tags, ordinaryTags) {
				t.Fatalf("Clash export lost nodes: %v", tags)
			}
			for i, node := range nodes {
				proxy := proxies[i]
				got, present := proxy["encryption"]
				if present != (node.encryption != "") || present && got != node.encryption {
					t.Errorf("%s encryption = %v (present=%v), want %q", node.name, got, present, node.encryption)
				}
				if proxy["name"] != node.name || proxy["network"] != node.transport || proxy["tls"] != true {
					t.Errorf("%s transport or TLS changed: %v", node.name, proxy)
				}
			}
			if got := service.GetLinks(&raw, "all", ""); !reflect.DeepEqual(got, wantRaw) {
				t.Fatal("Clash export changed Happ/raw source links")
			}
			after, afterTags := service.GetExternalOutbounds(&raw)
			if !reflect.DeepEqual(after, ordinary) || !reflect.DeepEqual(afterTags, ordinaryTags) || !reflect.DeepEqual([]byte(raw), original) {
				t.Fatal("Clash export mutated ordinary export or source JSON")
			}
		})
	}
}
