package sub

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClashWsKeepsAllHeaders(t *testing.T) {
	openClashTestSettings(t)
	var outbound map[string]interface{}
	err := json.Unmarshal([]byte(`{"type": "vless", "tag": "ws", "server": "192.0.2.1", "server_port": 443, "uuid": "00000000-0000-4000-8000-000000000001", "transport": {"type": "ws", "path": "/socket", "headers": {"Host": ["example.com"], "User-Agent": "agent/1.0", "X-Token": ["abc"]}}}`), &outbound)
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
	headers, _ := wsOpts["headers"].(map[string]interface{})
	for name, want := range map[string]string{"Host": "example.com", "User-Agent": "agent/1.0", "X-Token": "abc"} {
		if headers[name] != want {
			t.Errorf("header %s = %v, want %q", name, headers[name], want)
		}
	}
}
