package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestFormatForUserAgent(t *testing.T) {
	for agent, want := range map[string]string{
		"Happ/5.2.0/ios":      "happ",
		"happ/3.26.3/Android": "happ",
		"clash-verge/v2.2.3":  "clash",
		"ClashX Pro/1.118.0 (com.west2online.ClashXPro; build:1.118.0; macOS 15.1.0) Alamofire/5.9.1": "clash",
		"ClashMetaForAndroid/2.11.5.Meta":                           "clash",
		"ClashforWindows/0.20.39":                                   "clash",
		"FlClash/v0.8.80 clash-verge Platform/android":              "clash",
		"mihomo/1.19.1":                                             "clash",
		"Stash/2.7.1 Clash/1.11.0":                                  "clash",
		"SFA/1.11.0 (Android; sing-box 1.11.0)":                     "json",
		"SFI/1.11.0 (iOS 18.1; sing-box 1.11.0)":                    "json",
		"SFM/1.11.0 (macOS 15.1; sing-box 1.11.0)":                  "json",
		"sing-box 1.14.4":                                           "json",
		"HiddifyNext/2.5.7 (android) like ClashMeta v2ray sing-box": "",
		"v2rayNG/1.9.30":                                            "",
		"v2rayN/7.4.2":                                              "",
		"Shadowrocket/2.2.53 CFNetwork/1568.200.51 Darwin/24.1.0":   "",
		"Streisand/1.6.38":                                          "",
		"HappyClient/1.0":                                           "",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64)":                 "",
		"": "",
	} {
		if got := formatForUserAgent(agent); got != want {
			t.Errorf("formatForUserAgent(%q) = %q, want %q", agent, got, want)
		}
	}
}

// A client that asks for no format gets the one its app reads, and an explicit
// format still wins over the User-Agent.
func TestSubscriptionFormatByUserAgent(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "ua.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	vless := "vless://00000000-0000-4000-8000-000000000001@192.0.2.2:443?type=tcp&encryption=none#VLESS"
	client := model.Client{
		Name:     "ua-test",
		Enable:   true,
		Config:   json.RawMessage(`{}`),
		Inbounds: json.RawMessage(`[]`),
		Links:    externalSubTestLinks(t, []Link{{Type: "external", Uri: vless}}),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	NewSubHandler(router.Group("/sub"))
	get := func(t *testing.T, agent, query string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/sub/"+client.Name+query, nil)
		req.Header.Set("User-Agent", agent)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
		}
		return response.Body.String()
	}
	isClash := func(body string) bool { return strings.Contains(body, "proxies:") }
	isJSON := func(body string) bool {
		var config map[string]any
		return json.Unmarshal([]byte(body), &config) == nil && config["outbounds"] != nil
	}
	isLinks := func(body string) bool {
		decoded, err := base64.StdEncoding.DecodeString(body)
		return err == nil && string(decoded) == vless
	}

	for _, test := range []struct {
		name, agent, query string
		check              func(string) bool
	}{
		{"clash", "clash-verge/v2.2.3", "", isClash},
		{"mihomo", "mihomo/1.19.1", "", isClash},
		{"sing-box", "SFA/1.11.0 (Android; sing-box 1.11.0)", "", isJSON},
		{"hiddify", "HiddifyNext/2.5.7 (android) like ClashMeta v2ray sing-box", "", isLinks},
		{"v2rayNG", "v2rayNG/1.9.30", "", isLinks},
		{"explicit json beats clash agent", "clash-verge/v2.2.3", "?format=json", isJSON},
		{"explicit clash beats sing-box agent", "SFA/1.11.0", "?format=clash", isClash},
	} {
		t.Run(test.name, func(t *testing.T) {
			if body := get(t, test.agent, test.query); !test.check(body) {
				t.Errorf("unexpected body for %q%s:\n%.300s", test.agent, test.query, body)
			}
		})
	}
}
