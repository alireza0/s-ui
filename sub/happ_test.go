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

func TestHappSubscriptionWithProxyLinks(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "happ.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	httpLink := "http://user:pass@192.0.2.1:3128#HTTP"
	vless := "vless://00000000-0000-4000-8000-000000000001@192.0.2.2:443?type=xhttp&path=%2Fupload&encryption=none#XHTTP"
	socks := "socks://user:pass@192.0.2.3:1080#SOCKS"
	socksAddress := "user:p%40ss@192.0.2.4:1080#SOCKS%205"
	providerBody := "https://user:pass@192.0.2.5:443#HTTPS\r\n\r\n  socks5://" + socksAddress + "  \r\n"
	provider := externalSubTestServer(t, base64.StdEncoding.EncodeToString([]byte(providerBody)))
	client := model.Client{
		Name:     "happ-test",
		Enable:   true,
		Config:   json.RawMessage(`{}`),
		Inbounds: json.RawMessage(`[]`),
		Links: externalSubTestLinks(t, []Link{
			{Type: "external", Uri: httpLink},
			{Type: "external", Uri: vless},
			{Type: "external", Uri: socks},
			{Type: "sub", Uri: provider.URL},
		}),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	NewSubHandler(router.Group("/sub"))
	happ := strings.Join([]string{vless, socks, "socks://" + socksAddress}, "\n")
	raw := strings.Join([]string{httpLink, vless, socks, providerBody}, "\n")
	for _, test := range []struct {
		name, agent, query, want string
	}{
		{"Happ", "Happ/5.2.0/ios", "", happ},
		{"Android", "happ/3.26.3/Android", "", happ},
		{"explicit", "CustomAgent", "?format=happ", happ},
		{"generic", "HappyClient/1.0", "", raw},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/sub/"+client.Name+test.query, nil)
			req.Header.Set("User-Agent", test.agent)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
			}
			if test.query == "" && response.Header().Get("Vary") != "User-Agent" {
				t.Error("default subscription responses must vary by User-Agent")
			}
			body, err := base64.StdEncoding.DecodeString(response.Body.String())
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != test.want {
				t.Errorf("got %q, want %q", body, test.want)
			}
		})
	}
}
