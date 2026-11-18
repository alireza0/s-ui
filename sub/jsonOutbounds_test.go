package sub

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gin-gonic/gin"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	M "github.com/sagernet/sing/common/metadata"
)

const testCustomOutboundTemplate = `{
	"inbounds": [],
	"custom_outbounds": [
		{"type":"direct","tag":"🎯 Direct","connect_timeout":"10s"},
		{"type":"block","tag":"⛔ Block"}
	],
	"outbound_groups": [
		{"type":"selector","tag":"Ads","outbounds":["🎯 Direct","⛔ Block","$nodes"],"default":"⛔ Block"}
	],
	"final": "Ads"
}`

func TestJSONCustomOutboundsHTTPAndSelect(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "outbounds.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Create(&model.Setting{Key: "subJsonExt", Value: testCustomOutboundTemplate}).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Name: "alice", Enable: true, Config: json.RawMessage(`{}`),
		Inbounds: json.RawMessage(`[]`),
		Links:    json.RawMessage(`[{"type":"external","uri":"socks5://127.0.0.1:1080#alice-node"}]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	NewSubHandler(engine.Group("/sub"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sub/alice?format=json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", response.Code, response.Body.String())
	}

	ctx, cancel := context.WithTimeout(include.Context(context.Background()), 10*time.Second)
	defer cancel()
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, response.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	var timeout time.Duration
	for _, outbound := range options.Outbounds {
		if outbound.Tag == "🎯 Direct" {
			direct, ok := outbound.Options.(*option.DirectOutboundOptions)
			if !ok {
				t.Fatalf("unexpected direct options: %T", outbound.Options)
			}
			timeout = time.Duration(direct.ConnectTimeout)
		}
	}
	if timeout != 10*time.Second {
		t.Fatalf("expected a 10s direct connect timeout, got %s", timeout)
	}
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	outbound, _ := instance.Outbound().Outbound("Ads")
	selector, ok := outbound.(*group.Selector)
	if !ok {
		t.Fatalf("expected a selector, got %T", outbound)
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	destination := M.ParseSocksaddr(listener.Addr().String())
	if conn, err := selector.DialContext(ctx, "tcp", destination); !errors.Is(err, syscall.EPERM) {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("expected the default block selection to reject the dial, got %v", err)
	}
	if !selector.SelectOutbound("🎯 Direct") {
		t.Fatal("could not select the custom direct outbound")
	}
	conn, err := selector.DialContext(ctx, "tcp", destination)
	if err != nil {
		t.Fatalf("the custom direct outbound could not connect: %v", err)
	}
	conn.Close()
	if !selector.SelectOutbound("⛔ Block") {
		t.Fatal("could not select the custom block outbound")
	}
	if conn, err := selector.DialContext(ctx, "tcp", destination); !errors.Is(err, syscall.EPERM) {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("expected the block selection to reject the dial, got %v", err)
	}
}

func TestJSONCustomOutboundsInvalid(t *testing.T) {
	nodes := []map[string]interface{}{{"type": "direct", "tag": "test-node"}}
	for _, tc := range []struct {
		name, template, wantError string
	}{
		{"built-in collision", `{"custom_outbounds":[{"type":"block","tag":"direct"}]}`, "duplicate"},
		{"node collision", `{"custom_outbounds":[{"type":"block","tag":"test-node"}]}`, "duplicate"},
		{"group collision", `{"custom_outbounds":[{"type":"block","tag":"group"}],"outbound_groups":[{"type":"selector","tag":"group","outbounds":["$nodes"]}]}`, "duplicate"},
		{"reserved placeholder", `{"custom_outbounds":[{"type":"direct","tag":"$nodes"}]}`, "invalid"},
		{"proxy type", `{"custom_outbounds":[{"type":"socks","tag":"unsupported"}]}`, "unsupported"},
		{"unexpected property", `{"custom_outbounds":[{"type":"direct","tag":"extra","detour":"proxy"}]}`, "unknown field"},
		{"negative timeout", `{"custom_outbounds":[{"type":"direct","tag":"direct-timeout","connect_timeout":"-1s"}]}`, "non-negative"},
		{"block timeout", `{"custom_outbounds":[{"type":"block","tag":"blocked","connect_timeout":"10s"}]}`, "does not accept"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&JsonService{}).buildJSONConfig(tc.template, nodes, []string{"test-node"})
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("want an error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}
