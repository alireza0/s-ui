package util

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGetOutboundProxyLink(t *testing.T) {
	// Proxy links must keep credentials and remain usable without a fragment.
	for _, scheme := range []string{"http", "https", "socks5"} {
		for _, test := range []struct {
			name, uri, tag, host, username, password string
			port, index                              int
		}{
			{name: "named", uri: "http://user:pass@192.0.2.1:3071#USA", tag: "USA", host: "192.0.2.1", port: 3071, username: "user", password: "pass"},
			{name: "unnamed", uri: "http://user:pass@proxy.example:3071", tag: "proxy.example:3071", host: "proxy.example", port: 3071, username: "user", password: "pass"},
			{name: "escaped", uri: "http://user%40name:p%40ss%3Aword%2B%23%25@proxy.example:8080#%D0%A1%D0%A8%D0%90%20%7C%20HTTP%2520", tag: "США | HTTP%20", host: "proxy.example", port: 8080, username: "user@name", password: "p@ss:word+#%"},
			{name: "no-auth", uri: "http://192.0.2.2:80", tag: "192.0.2.2:80", host: "192.0.2.2", port: 80},
			{name: "username-only", uri: "http://user@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user"},
			{name: "empty-password", uri: "http://user:@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user"},
			{name: "password-colon", uri: "http://user:pass:word@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user", password: "pass:word"},
			{name: "indexed", uri: "http://user:pass@proxy.example:3071#USA", tag: "3.USA", host: "proxy.example", port: 3071, username: "user", password: "pass", index: 3},
			{name: "indexed-unnamed", uri: "http://proxy.example:3071", tag: "2.proxy.example:3071", host: "proxy.example", port: 3071, index: 2},
		} {
			t.Run(scheme+"/"+test.name, func(t *testing.T) {
				want := map[string]interface{}{"type": "http", "tag": test.tag, "server": test.host, "server_port": test.port, "username": test.username, "password": test.password}
				if scheme == "https" {
					want["tls"] = map[string]interface{}{"enabled": true}
				} else if scheme == "socks5" {
					want["type"] = "socks"
					want["version"] = "5"
				}
				uri := strings.Replace(test.uri, "http://", scheme+"://", 1)
				for _, parse := range []func(string, int) (*map[string]interface{}, string, error){GetOutbound, GetOutboundForClash} {
					outbound, tag, err := parse(uri, test.index)
					if err != nil || outbound == nil {
						t.Fatalf("parse proxy link: %v", err)
					}
					if tag != test.tag || !reflect.DeepEqual(*outbound, want) {
						t.Errorf("got %v / %q, want %v / %q", *outbound, tag, want, test.tag)
					}
				}
			})
		}
	}
}

func TestGetOutboundRejectsInvalidProxyLink(t *testing.T) {
	for _, scheme := range []string{"http", "https", "socks5"} {
		for _, uri := range []string{
			"http://user:pass@:3071", "http://proxy.example", "http://proxy.example:",
			"http://proxy.example:0", "http://proxy.example:65536", "http://proxy.example:abc",
			"http:///missing-host:8080", "http://user:%zz@proxy.example:8080", "http://proxy.example:8080#%zz",
			"http://proxy.example:8080/", "http://proxy.example:8080/subscription",
			"http://proxy.example:8080/%73ubscription", "http://proxy.example:8080/subscription?token=test",
		} {
			uri = strings.Replace(uri, "http://", scheme+"://", 1)
			t.Run(uri, func(t *testing.T) {
				outbound, tag, err := GetOutbound(uri, 0)
				if err == nil || outbound != nil || tag != "" {
					t.Errorf("invalid proxy link accepted: %v / %q / %v", outbound, tag, err)
				}
			})
		}
	}
}

func TestHTTPSProxyTLSOptions(t *testing.T) {
	outbound, _, err := GetOutbound("https://user:pass@192.0.2.1:8443?sni=tls.example&insecure=1&alpn=h2%2Chttp%2F1.1#TLS", 0)
	if err != nil || outbound == nil {
		t.Fatalf("parse HTTPS proxy: %v", err)
	}
	want := map[string]interface{}{"enabled": true, "server_name": "tls.example", "insecure": true, "alpn": []string{"h2", "http/1.1"}}
	if !reflect.DeepEqual((*outbound)["tls"], want) {
		t.Errorf("TLS options = %v, want %v", (*outbound)["tls"], want)
	}
}

func TestPanelGeneratedProxyLinksRoundTrip(t *testing.T) {
	for _, scheme := range []string{"http", "https", "socks5"} {
		t.Run(scheme, func(t *testing.T) {
			typ := "http"
			if scheme == "socks5" {
				typ = "socks"
			}
			inbound := inboundFor(t, typ, nil)
			addr := map[string]interface{}{"server": "192.0.2.1", "server_port": 8443, "remark": "-Proxy | 100%"}
			if scheme == "https" {
				addr["tls"] = map[string]interface{}{"enabled": true}
			}
			raw, err := json.Marshal([]map[string]interface{}{addr})
			if err != nil {
				t.Fatal(err)
			}
			inbound.Addrs = raw
			fields := map[string]interface{}{"username": "user@name", "password": "p@ss:word+#%"}
			links := LinkGenerator(clientConfigFor(t, typ, fields), inbound, "unused.example", "")
			if len(links) != 1 || !strings.HasPrefix(links[0], scheme+"://") {
				t.Fatalf("unexpected generated links: %v", links)
			}
			outbound, tag, err := GetOutbound(links[0], 0)
			if err != nil || outbound == nil {
				t.Fatalf("import generated link: %v", err)
			}
			if (*outbound)["type"] != typ || tag != inbound.Tag+addr["remark"].(string) || (*outbound)["username"] != fields["username"] || (*outbound)["password"] != fields["password"] {
				t.Errorf("generated proxy changed on import: %v", *outbound)
			}
		})
	}
}
