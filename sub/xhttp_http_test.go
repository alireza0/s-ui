package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func TestXHTTPSubscriptionHTTPPipeline(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "http-smoke.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	uris, directLinks := fourXHTTPLinks(t)
	plain := strings.Join(uris, "\n")
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.URL.Path == "/base64" {
			_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(plain))))
			return
		}
		_, _ = w.Write([]byte(plain))
	}))
	t.Cleanup(provider.Close)
	linksForURL := func(url string) json.RawMessage {
		data, err := json.Marshal([]Link{{Type: "sub", Uri: url}})
		if err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(data)
	}
	clients := []model.Client{
		{Name: "http-direct", Links: directLinks},
		{Name: "http-plaintext", Links: linksForURL(provider.URL + "/plain")},
		{Name: "http-base64", Links: linksForURL(provider.URL + "/base64")},
	}
	for index := range clients {
		clients[index].Enable = true
		clients[index].Remark = "HTTP smoke test"
		clients[index].Config = json.RawMessage(`{}`)
		clients[index].Inbounds = json.RawMessage(`[]`)
		if err := db.Create(&clients[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	engine := gin.New()
	NewSubHandler(engine.Group("/sub"))
	request := func(t *testing.T, name, query, userAgent string) []byte {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/sub/"+name+query, nil)
		req.Header.Set("User-Agent", userAgent)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("%s%s: HTTP %d: %s", name, query, response.Code, response.Body.String())
		}
		for _, header := range []string{"Subscription-Userinfo", "Profile-Update-Interval", "Profile-Title", "Content-Disposition"} {
			if response.Header().Get(header) == "" {
				t.Errorf("%s%s: missing %s", name, query, header)
			}
		}
		return response.Body.Bytes()
	}
	encoded, err := (&SubService{}).GetSubEncode()
	if err != nil {
		t.Fatal(err)
	}

	var directProxies []map[string]interface{}
	var directJSON map[string]interface{}
	for _, client := range clients {
		t.Run(client.Name, func(t *testing.T) {
			var clash struct {
				Proxies []map[string]interface{} `yaml:"proxies"`
			}
			if err := yaml.Unmarshal(request(t, client.Name, "?format=clash", "Clash.Meta"), &clash); err != nil {
				t.Fatal(err)
			}
			if len(clash.Proxies) != 4 {
				t.Fatalf("got %d proxies, want four", len(clash.Proxies))
			}
			for index, proxy := range clash.Proxies {
				host := fmt.Sprintf("test-%d.example", index+1)
				if proxy["network"] != "xhttp" || proxy["tls"] != true || proxy["servername"] != host || proxy["client-fingerprint"] != "firefox" || !reflect.DeepEqual(proxy["alpn"], []interface{}{"h2", "http/1.1"}) {
					t.Fatalf("node %d lost XHTTP/TLS/ALPN: %v", index, proxy)
				}
				opts, ok := proxy["xhttp-opts"].(map[string]interface{})
				if !ok || opts["host"] != host || opts["path"] != "/api/v1/upload" || opts["mode"] != "auto" {
					t.Fatalf("node %d lost XHTTP parameters: %v", index, opts)
				}
				if index != 2 {
					reuse, ok := opts["reuse-settings"].(map[string]interface{})
					if opts["x-padding-bytes"] != "100-1000" || !ok || reuse["max-concurrency"] != "16-32" || reuse["c-max-reuse-times"] != "64-128" {
						t.Fatalf("node %d lost Extra/XMUX: %v", index, opts)
					}
				}
			}
			if directProxies == nil {
				directProxies = clash.Proxies
			} else if !reflect.DeepEqual(clash.Proxies, directProxies) {
				t.Fatal("External Subs HTTP Clash response differs from direct External Links")
			}

			for _, userAgent := range []string{"HTTP-smoke", "Happ/1.0"} {
				raw := request(t, client.Name, "", userAgent)
				if encoded {
					decoded, err := base64.StdEncoding.DecodeString(string(raw))
					if err != nil {
						t.Fatal(err)
					}
					raw = decoded
				}
				if string(raw) != plain {
					t.Fatalf("%s raw/Happ response changed source URIs", userAgent)
				}
			}
			var jsonConfig map[string]interface{}
			if err := json.Unmarshal(request(t, client.Name, "?format=json", "sing-box"), &jsonConfig); err != nil {
				t.Fatal(err)
			}
			if directJSON == nil {
				directJSON = jsonConfig
			} else if !reflect.DeepEqual(jsonConfig, directJSON) {
				t.Fatal("External Subs JSON HTTP response differs from direct External Links")
			}
			var nodeCount int
			for _, value := range jsonConfig["outbounds"].([]interface{}) {
				outbound := value.(map[string]interface{})
				if outbound["type"] != "vless" {
					continue
				}
				nodeCount++
				if transport, _ := outbound["transport"].(map[string]interface{}); len(transport) != 0 {
					t.Fatalf("Clash-only XHTTP fields leaked into ordinary JSON: %v", transport)
				}
			}
			if nodeCount != 4 {
				t.Fatalf("ordinary JSON lost external nodes: %d", nodeCount)
			}
		})
	}
}
