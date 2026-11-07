package sub

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"

	"gopkg.in/yaml.v3"
)

// Mihomo wants the bare ECHConfigList; the PEM armour lines made it invalid.
// #1339
func TestClashECHConfigWithoutArmour(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	var outbound map[string]interface{}
	err := json.Unmarshal([]byte(`{"type": "trojan", "tag": "t", "server": "192.0.2.1", "server_port": 443, "password": "p",
		"tls": {"enabled": true, "server_name": "real.example.com", "ech": {"enabled": true,
			"config": ["-----BEGIN ECH CONFIGS-----", "AEX+DQBB", "cHVibGlj", "-----END ECH CONFIGS-----"]}}}`), &outbound)
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
	opts, _ := config.Proxies[0]["ech-opts"].(map[string]interface{})
	if opts["config"] != "AEX+DQBBcHVibGlj" || opts["enable"] != true {
		t.Fatalf("ech-opts = %v", opts)
	}
}
