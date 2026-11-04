package sub

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClashWsKeepsMaxEarlyData(t *testing.T) {
	openClashTestSettings(t)
	var outbound map[string]interface{}
	err := json.Unmarshal([]byte(`{"type": "vless", "tag": "ws", "server": "192.0.2.1", "server_port": 443, "uuid": "00000000-0000-4000-8000-000000000001", "tls": {"enabled": true, "server_name": "ws.example"}, "transport": {"type": "ws", "path": "/socket", "max_early_data": 2048, "early_data_header_name": "Sec-WebSocket-Protocol"}}`), &outbound)
	if err != nil {
		t.Fatal(err)
	}
	outbounds := []map[string]interface{}{outbound}
	result, err := (&ClashService{}).ConvertToClashMeta(&outbounds, "mode: rule\n")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Proxies) != 1 {
		t.Fatalf("got %d proxies, want one", len(config.Proxies))
	}
	wsOpts, _ := config.Proxies[0]["ws-opts"].(map[string]interface{})
	if wsOpts["max-early-data"] != 2048 {
		t.Errorf("max-early-data = %v, want 2048", wsOpts["max-early-data"])
	}
	if wsOpts["early-data-header-name"] != "Sec-WebSocket-Protocol" {
		t.Errorf("early-data-header-name = %v", wsOpts["early-data-header-name"])
	}
}
