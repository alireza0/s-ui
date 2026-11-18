package sub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gin-gonic/gin"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

const testGroupTemplate = `{
	"inbounds": [],
	"outbound_groups": [
		{"type":"urltest","tag":"♻️ Auto","outbounds":["$nodes"],"interval":"10m"},
		{"type":"selector","tag":"💬 AI","outbounds":["♻️ Auto","direct","$nodes"],"default":"$nodes"}
	],
	"rules": [{"domain":["example.com"],"action":"route","outbound":"💬 AI"}],
	"final": "💬 AI"
}`

func TestJSONOutboundGroupsStart(t *testing.T) {
	nodes := []map[string]interface{}{
		{"type": "socks", "tag": "🇬🇧 Test node", "server": "127.0.0.1", "server_port": 1080},
	}
	config, err := (&JsonService{}).buildJSONConfig(testGroupTemplate, nodes, []string{"🇬🇧 Test node"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	defer cancel()
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, config); err != nil {
		t.Fatal(err)
	}
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestJSONOutboundGroupsInvalid(t *testing.T) {
	tags := []string{"node"}
	nodes := []map[string]interface{}{{"type": "direct", "tag": "node"}}
	for _, tc := range []struct {
		name, groups, wantError string
	}{
		{"duplicate", `[{"type":"selector","tag":"direct","outbounds":["$nodes"]}]`, "duplicate"},
		{"unknown", `[{"type":"selector","tag":"custom","outbounds":["typo"]}]`, "unknown outbound"},
		{"missing default", `[{"type":"selector","tag":"custom","outbounds":["direct"],"default":"$nodes"}]`, "not a member"},
		{"cycle", `[{"type":"selector","tag":"a","outbounds":["b"]},{"type":"urltest","tag":"b","outbounds":["a"]}]`, "circular"},
		{"proxy type", `[{"type":"socks","tag":"private"}]`, "unsupported"},
		{"unknown field", `[{"type":"selector","tag":"custom","outbounds":["$nodes"],"password":"test-only"}]`, "unknown field"},
		{"null groups", `null`, "must be an array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&JsonService{}).buildJSONConfig(`{"outbound_groups":`+tc.groups+`}`, nodes, tags)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("want error containing %q, got %v", tc.wantError, err)
			}
		})
	}
	if _, err := (&JsonService{}).buildJSONConfig(testGroupTemplate, nil, nil); err == nil {
		t.Fatal("expected a subscription without nodes to fail instead of falling back to direct")
	}
}

func TestJSONOutboundGroupsLegacyTemplate(t *testing.T) {
	nodes := []map[string]interface{}{{"type": "direct", "tag": "node"}}
	tags := []string{"node"}
	service := &JsonService{}
	want, err := service.buildJSONConfig(`{"final":"direct"}`, nodes, tags)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.buildJSONConfig(`{
		"final":"direct",
		"outbound_groups":[],
		"outbounds":[{"type":"socks","tag":"previously-ignored","password":"test-only"}],
		"route":{"final":"previously-ignored"},
		"services":[{"type":"api"}]
	}`, nodes, tags)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("the previously ignored fields changed a legacy subscription")
	}
}

func TestJSONOutboundGroupsHTTP(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "groups.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Save(&model.Setting{Key: "subJsonExt", Value: testGroupTemplate}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alice", "bob", "empty"} {
		links := json.RawMessage(`[]`)
		if name != "empty" {
			links = json.RawMessage(`[{"type":"external","uri":"socks5://127.0.0.1:1080#` + name + `-node"}]`)
		}
		client := model.Client{
			Name: name, Enable: true, Config: json.RawMessage(`{}`),
			Inbounds: json.RawMessage(`[]`), Links: links,
		}
		if err := db.Create(&client).Error; err != nil {
			t.Fatal(err)
		}
	}
	engine := gin.New()
	NewSubHandler(engine.Group("/sub"))
	for _, name := range []string{"alice", "bob", "empty"} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sub/"+name+"?format=json", nil))
			if name == "empty" {
				if response.Code != http.StatusBadRequest {
					t.Fatalf("expected a 400 for the node-less subscription, got %d", response.Code)
				}
				return
			}
			if response.Code != http.StatusOK {
				t.Fatalf("expected HTTP 200, got %d: %s", response.Code, response.Body.String())
			}
			other := "alice"
			if name == other {
				other = "bob"
			}
			if strings.Contains(response.Body.String(), other+"-node") {
				t.Fatal("the subscription contains another subscriber's node")
			}
			var config struct {
				Outbounds []subscriptionGroup
				Route     struct{ Final string }
			}
			if err := json.Unmarshal(response.Body.Bytes(), &config); err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(config.Outbounds, func(group subscriptionGroup) bool {
				return group.Tag == "💬 AI"
			})
			if index == -1 {
				t.Fatal("selector was not generated")
			}
			group := config.Outbounds[index]
			want := []string{"♻️ Auto", "direct", name + "-node"}
			if !slices.Equal(group.Outbounds, want) || group.Default != name+"-node" ||
				config.Route.Final != group.Tag {
				t.Fatalf("unexpected selector or final route: %+v %q", group, config.Route.Final)
			}
		})
	}
}
