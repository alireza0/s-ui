package sub

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"

	"gopkg.in/yaml.v3"
)

// Mihomo reads the gecko packet sizes from their own keys, not from the obfs
// type. #1328
func TestClashHysteria2GeckoPacketSizes(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	var outbound map[string]interface{}
	err := json.Unmarshal([]byte(`{"type": "hysteria2", "tag": "hy2", "server": "192.0.2.1", "server_port": 443, "password": "p",
		"obfs": {"type": "gecko", "password": "secret", "min_packet_size": 600, "max_packet_size": 1100},
		"tls": {"enabled": true, "server_name": "hy2.example"}}`), &outbound)
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
	p := config.Proxies[0]
	if p["obfs"] != "gecko" || p["obfs-password"] != "secret" {
		t.Errorf("obfs = %v / %v", p["obfs"], p["obfs-password"])
	}
	if p["obfs-min-packet-size"] != 600 || p["obfs-max-packet-size"] != 1100 {
		t.Errorf("packet sizes = %v / %v, want 600 / 1100", p["obfs-min-packet-size"], p["obfs-max-packet-size"])
	}
}
