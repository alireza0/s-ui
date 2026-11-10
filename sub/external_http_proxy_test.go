package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExternalProxyLinksAndSubscriptions(t *testing.T) {
	openClashTestSettings(t)
	service := &ClashService{}
	uris := []string{
		"http://user:pass@192.0.2.1:3071#USA",
		"http://user%40name:p%40ss%3Aword%2B%23%25@proxy.example:8080#%D0%A1%D0%A8%D0%90%20%7C%20HTTP",
		"http://user:pass@192.0.2.2:3071",
		"http://192.0.2.3:8080#USA",
		"https://user:pass@192.0.2.4:8443?sni=tls.example&insecure=1#TLS",
		"socks5://user%40name:p%40ss%3Aword@proxy.example:1080#SOCKS",
	}
	wantNames := []string{"USA", "США | HTTP", "192.0.2.2:3071", "USA-1", "TLS", "SOCKS"}
	decoded := "\r\n  " + strings.Join(uris, "  \r\n\r\n  ") + "  \r\n"
	for _, format := range []string{"direct", "plaintext", "base64"} {
		t.Run(format, func(t *testing.T) {
			var links []Link
			wantRaw := uris
			if format == "direct" {
				for _, uri := range uris {
					links = append(links, Link{Type: "external", Uri: uri})
				}
			} else {
				body := decoded
				if format == "base64" {
					body = base64.StdEncoding.EncodeToString([]byte(body))
				}
				provider := externalSubTestServer(t, body)
				links = []Link{{Type: "sub", Uri: provider.URL}}
				wantRaw = strings.Split(decoded, "\n")
			}
			raw := externalSubTestLinks(t, links)
			original := append([]byte(nil), raw...)
			ordinary, tags := service.GetExternalOutbounds(&raw)
			clash, clashTags := service.GetExternalOutboundsForClash(&raw)
			if len(ordinary) != len(uris) || !reflect.DeepEqual(tags, wantNames) || !reflect.DeepEqual(tags, clashTags) || !reflect.DeepEqual(ordinary, clash) {
				t.Fatalf("proxy import lost nodes or names: %v / %v", tags, clashTags)
			}
			proxies := externalSubTestProxies(t, service, clash)
			if len(proxies) != len(uris) {
				t.Fatalf("got %d Clash proxies, want %d", len(proxies), len(uris))
			}
			for i, proxy := range proxies {
				outbound := ordinary[i]
				wantType := "http"
				if outbound["type"] == "socks" {
					wantType = "socks5"
				}
				if proxy["type"] != wantType || proxy["name"] != wantNames[i] || proxy["server"] != outbound["server"] || proxy["port"] != outbound["server_port"] || proxy["username"] != outbound["username"] || proxy["password"] != outbound["password"] {
					t.Errorf("proxy changed during Clash conversion: %v", proxy)
				}
				if strings.HasPrefix(uris[i], "https://") {
					if proxy["tls"] != true || proxy["sni"] != "tls.example" || proxy["skip-cert-verify"] != true {
						t.Errorf("HTTPS proxy lost TLS options: %v", proxy)
					}
				} else if proxy["tls"] == true {
					t.Errorf("plain proxy enabled TLS: %v", proxy)
				}
			}
			if got := service.GetLinks(&raw, "all", ""); !reflect.DeepEqual(got, wantRaw) || !reflect.DeepEqual([]byte(raw), original) {
				t.Fatal("proxy import changed raw links or source JSON")
			}
		})
	}
}

func TestExternalHTTPProxyDoesNotFetchSubscription(t *testing.T) {
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("http://user:pass@192.0.2.1:3071#Provider-node"))
	}))
	t.Cleanup(provider.Close)
	service := &LinkService{}
	raw := externalSubTestLinks(t, []Link{{Type: "external", Uri: provider.URL}})
	outbounds, _ := service.GetExternalOutbounds(&raw)
	if len(outbounds) != 1 || requests.Load() != 0 {
		t.Fatal("an HTTP proxy link was fetched as a subscription")
	}
	raw = externalSubTestLinks(t, []Link{{Type: "external", Uri: provider.URL + "/subscription"}})
	outbounds, _ = service.GetExternalOutbounds(&raw)
	if len(outbounds) != 0 || requests.Load() != 0 {
		t.Fatal("a subscription path was accepted as a proxy or fetched")
	}
	raw = externalSubTestLinks(t, []Link{{Type: "sub", Uri: provider.URL + "/subscription"}})
	outbounds, tags := service.GetExternalOutbounds(&raw)
	if len(outbounds) != 1 || len(tags) != 1 || tags[0] != "Provider-node" || requests.Load() != 1 {
		t.Fatal("an external subscription URL was parsed as an HTTP proxy")
	}
}
