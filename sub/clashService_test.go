package sub

import (
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"

	"gopkg.in/yaml.v3"
)

// "Enable UDP by default" skipped anytls, which Mihomo also keeps TCP-only
// unless the proxy opts in; and an anytls proxy without insecure set got
// skip-cert-verify: null (#1291).
func TestClashDefaultUdpCoversAnytls(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: "subClashUdp", Value: "true"}).Error; err != nil {
		t.Fatal(err)
	}

	outbounds := []map[string]interface{}{
		{"type": "vless", "tag": "v", "server": "a.com", "server_port": 443, "uuid": "b831381d-6324-4d53-ad4f-8cda48b30811"},
		{"type": "anytls", "tag": "at", "server": "a.com", "server_port": 443, "password": "p",
			"tls": map[string]interface{}{"enabled": true, "server_name": "a.com"}},
	}
	out, err := (&ClashService{}).ConvertToClashMeta(&outbounds, basicClashConfig)
	if err != nil {
		t.Fatal(err)
	}

	var config struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(out), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(config.Proxies))
	}
	for _, proxy := range config.Proxies {
		if proxy["udp"] != true {
			t.Errorf("%v: udp = %v, want true", proxy["name"], proxy["udp"])
		}
	}
	anytls := config.Proxies[1]
	if v, present := anytls["skip-cert-verify"]; present {
		t.Errorf("anytls: skip-cert-verify must be left out when insecure is unset, got %v", v)
	}
	if anytls["sni"] != "a.com" {
		t.Errorf("anytls: sni = %v, want a.com", anytls["sni"])
	}
}
