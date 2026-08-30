package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// openTestDB gives each test its own on-disk database, since OpenDB stores the
// handle in the package-level db used by the migration.
func openTestDB(t *testing.T) {
	t.Helper()
	if err := OpenDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Setting{}, &model.Tls{}); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T) map[string]any {
	t.Helper()
	var setting model.Setting
	if err := db.Where("key = ?", "config").First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(setting.Value), &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func section(t *testing.T, root map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := root[key].(map[string]any)
	if !ok {
		t.Fatalf("missing %q section in %v", key, root)
	}
	return value
}

func TestMigrateSingBox114Config(t *testing.T) {
	openTestDB(t)
	legacy := `{
		"log": {"level": "info"},
		"dns": {"servers": [], "rules": [], "independent_cache": true, "cache_capacity": 4096},
		"experimental": {"cache_file": {"enabled": true, "store_rdrc": true}},
		"route": {
			"rules": [{"action": "sniff"}],
			"rule_set": [
				{"type": "remote", "tag": "geoip-cn", "url": "https://example.com/a.srs", "download_detour": "direct"},
				{"type": "remote", "tag": "geosite-ads", "url": "https://example.com/b.srs"},
				{"type": "remote", "tag": "geosite-ir", "url": "https://example.com/c.srs", "download_detour": "proxy"}
			]
		}
	}`
	if err := db.Create(&model.Setting{Key: "config", Value: legacy}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateSingBox114(); err != nil {
		t.Fatal(err)
	}
	root := readConfig(t)

	dns := section(t, root, "dns")
	if _, ok := dns["independent_cache"]; ok {
		t.Error("independent_cache should have been dropped")
	}
	if dns["cache_capacity"] != float64(4096) {
		t.Errorf("unrelated dns options must be preserved, got %v", dns["cache_capacity"])
	}
	if root["log"] == nil {
		t.Error("unrelated top-level sections must be preserved")
	}

	cacheFile := section(t, section(t, root, "experimental"), "cache_file")
	if _, ok := cacheFile["store_rdrc"]; ok {
		t.Error("store_rdrc should have been renamed")
	}
	if cacheFile["store_dns"] != true {
		t.Errorf("store_rdrc should become store_dns, got %v", cacheFile["store_dns"])
	}

	ruleSets, ok := section(t, root, "route")["rule_set"].([]any)
	if !ok || len(ruleSets) != 3 {
		t.Fatalf("expected 3 rule sets, got %v", ruleSets)
	}
	// A direct detour is what sing-box does anyway, and it cannot be expressed
	// as an http_client detour, so it just goes away.
	direct, _ := ruleSets[0].(map[string]any)
	if _, ok = direct["download_detour"]; ok {
		t.Error("download_detour should have been replaced")
	}
	if _, ok = direct["http_client"]; ok {
		t.Errorf("a direct detour must not become an http_client, got %v", direct)
	}
	if untouched, _ := ruleSets[1].(map[string]any); untouched["http_client"] != nil {
		t.Error("rule sets without download_detour must be left alone")
	}
	proxied, _ := ruleSets[2].(map[string]any)
	if _, ok = proxied["download_detour"]; ok {
		t.Error("download_detour should have been replaced")
	}
	httpClient, ok := proxied["http_client"].(map[string]any)
	if !ok {
		t.Fatalf("expected http_client, got %v", proxied)
	}
	if httpClient["detour"] != "proxy" {
		t.Errorf("unexpected http_client: %v", httpClient)
	}
	// disable_empty_direct_check has no JSON field; emitting it makes the whole
	// config unparseable.
	if _, ok = httpClient["disable_empty_direct_check"]; ok {
		t.Errorf("disable_empty_direct_check is not a real option, got %v", httpClient)
	}
}

func TestMigrateSingBox114Tls(t *testing.T) {
	openTestDB(t)
	server := `{"enabled": true, "server_name": "example.com", "acme": {"domain": ["example.com"], "email": "a@example.com", "dns01_challenge": {"provider": "cloudflare", "api_token": "tok"}}}`
	if err := db.Create(&model.Tls{Name: "cert", Server: json.RawMessage(server)}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateSingBox114(); err != nil {
		t.Fatal(err)
	}

	var stored model.Tls
	if err := db.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(stored.Server, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["acme"]; ok {
		t.Error("inline acme should have been removed")
	}
	if decoded["server_name"] != "example.com" {
		t.Error("unrelated tls options must be preserved")
	}
	provider, ok := decoded["certificate_provider"].(map[string]any)
	if !ok {
		t.Fatalf("expected certificate_provider, got %v", decoded)
	}
	if provider["type"] != "acme" {
		t.Errorf("expected type acme, got %v", provider["type"])
	}
	if provider["email"] != "a@example.com" {
		t.Errorf("acme fields must carry over, got %v", provider)
	}
	if _, ok = provider["dns01_challenge"].(map[string]any); !ok {
		t.Errorf("nested acme options must carry over, got %v", provider)
	}
}

// TestMigrateSingBox114RunsOnce guards that a config edited back to a legacy
// shape after the migration is not silently rewritten again.
func TestMigrateSingBox114RunsOnce(t *testing.T) {
	openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"dns": {"independent_cache": true}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSingBox114(); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Setting{}).Where("key = ?", "config").
		Update("value", `{"dns": {"independent_cache": true}}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSingBox114(); err != nil {
		t.Fatal(err)
	}
	if _, ok := section(t, readConfig(t), "dns")["independent_cache"]; !ok {
		t.Error("migration should not run a second time")
	}
}

// TestMigrateSingBox114Idempotent guards the common case: a config that has
// nothing to migrate must come through byte-identical.
func TestMigrateSingBox114Idempotent(t *testing.T) {
	openTestDB(t)
	clean := `{"log":{"level":"info"},"dns":{"servers":[],"rules":[]},"experimental":{}}`
	if err := db.Create(&model.Setting{Key: "config", Value: clean}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSingBox114(); err != nil {
		t.Fatal(err)
	}
	var setting model.Setting
	if err := db.Where("key = ?", "config").First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if setting.Value != clean {
		t.Errorf("config with nothing to migrate was rewritten:\n got %s\nwant %s", setting.Value, clean)
	}
}
