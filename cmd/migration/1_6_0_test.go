package migration

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB gives each test its own on-disk database. A migration runs before
// the panel brings the schema up to date, so the tables are created here
// straight from the models rather than through InitDB.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.Setting{}, &model.Tls{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func rawConfig(t *testing.T, db *gorm.DB) string {
	t.Helper()
	value, exists, err := readSetting(db, "config")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("no config setting")
	}
	return value
}

func readConfig(t *testing.T, db *gorm.DB) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(rawConfig(t, db)), &root); err != nil {
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
	db := openTestDB(t)
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

	if err := migrateSingBox114(db); err != nil {
		t.Fatal(err)
	}
	root := readConfig(t, db)

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
	db := openTestDB(t)
	server := `{"enabled": true, "server_name": "example.com", "acme": {"domain": ["example.com"], "email": "a@example.com", "dns01_challenge": {"provider": "cloudflare", "api_token": "tok"}}}`
	if err := db.Create(&model.Tls{Name: "cert", Server: json.RawMessage(server)}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateSingBox114(db); err != nil {
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

// TestMigrateSingBox114Idempotent guards the common case: a config that has
// nothing to migrate must come through byte-identical.
func TestMigrateSingBox114Idempotent(t *testing.T) {
	db := openTestDB(t)
	clean := `{"log":{"level":"info"},"dns":{"servers":[],"rules":[]},"experimental":{}}`
	if err := db.Create(&model.Setting{Key: "config", Value: clean}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSingBox114(db); err != nil {
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

func certificateProviders(t *testing.T, root map[string]any) []map[string]any {
	t.Helper()
	raw, ok := root["certificate_providers"].([]any)
	if !ok {
		t.Fatalf("missing certificate_providers in %v", root)
	}
	providers := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		provider, isObject := entry.(map[string]any)
		if !isObject {
			t.Fatalf("expected provider objects, got %v", entry)
		}
		providers = append(providers, provider)
	}
	return providers
}

func tlsServer(t *testing.T, db *gorm.DB, id uint) map[string]any {
	t.Helper()
	var stored model.Tls
	if err := db.Where("id = ?", id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var server map[string]any
	if err := json.Unmarshal(stored.Server, &server); err != nil {
		t.Fatal(err)
	}
	return server
}

func TestMigrateCertificateProviders(t *testing.T) {
	db := openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"log":{"level":"info"}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tls{
		Name: "Main Site",
		Server: json.RawMessage(`{
			"enabled": true,
			"certificate_provider": {"type": "acme", "domain": ["example.com"], "email": "a@example.com"}
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateCertificateProviders(db); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t, db))
	if len(providers) != 1 {
		t.Fatalf("expected one hoisted provider, got %v", providers)
	}
	if providers[0]["type"] != "acme" || providers[0]["email"] != "a@example.com" {
		t.Errorf("the provider fields must carry over, got %v", providers[0])
	}
	tag, _ := providers[0]["tag"].(string)
	if tag != "main-site" {
		t.Errorf("the tag should be named after the TLS config, got %q", tag)
	}
	// The TLS config must now point at the shared provider rather than repeat it.
	if got := tlsServer(t, db, 1)["certificate_provider"]; got != tag {
		t.Errorf("expected the tag %q as the reference, got %v", tag, got)
	}
}

// A tag reference is already the new form and must be left alone, as must a
// provider that was defined in the config by hand.
func TestMigrateCertificateProvidersLeavesReferences(t *testing.T) {
	db := openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{
		"certificate_providers": [{"type": "acme", "tag": "shared", "domain": ["example.com"]}]
	}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tls{
		Name:   "site",
		Server: json.RawMessage(`{"enabled": true, "certificate_provider": "shared"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateCertificateProviders(db); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t, db))
	if len(providers) != 1 {
		t.Errorf("nothing should have been added, got %v", providers)
	}
	if got := tlsServer(t, db, 1)["certificate_provider"]; got != "shared" {
		t.Errorf("the reference must be untouched, got %v", got)
	}
}

// Two TLS configs whose names slugify the same must not end up sharing a tag,
// which would silently point one of them at the other's provider.
func TestMigrateCertificateProvidersDeduplicatesTags(t *testing.T) {
	db := openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{
		"certificate_providers": [{"type": "acme", "tag": "my-site", "domain": ["taken.example"]}]
	}`}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"My Site", "my.site"} {
		if err := db.Create(&model.Tls{
			Name:   name,
			Server: json.RawMessage(`{"enabled": true, "certificate_provider": {"type": "acme", "domain": ["example.com"]}}`),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateCertificateProviders(db); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t, db))
	if len(providers) != 3 {
		t.Fatalf("expected the hand-written provider plus two hoisted ones, got %v", providers)
	}
	tags := make(map[string]bool)
	for _, provider := range providers {
		tag, _ := provider["tag"].(string)
		if tag == "" {
			t.Fatalf("every shared provider needs a tag, got %v", provider)
		}
		if tags[tag] {
			t.Fatalf("duplicate tag %q in %v", tag, providers)
		}
		tags[tag] = true
	}
	for id := uint(1); id <= 2; id++ {
		reference, _ := tlsServer(t, db, id)["certificate_provider"].(string)
		if reference == "my-site" {
			t.Errorf("tls %d must not claim the existing provider's tag", id)
		}
		if !tags[reference] {
			t.Errorf("tls %d references %q, which is not defined", id, reference)
		}
	}
}

func ruleSets(t *testing.T, root map[string]any) []map[string]any {
	t.Helper()
	raw, ok := section(t, root, "route")["rule_set"].([]any)
	if !ok {
		t.Fatalf("missing rule_set in %v", root)
	}
	converted := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		ruleSet, isObject := entry.(map[string]any)
		if !isObject {
			t.Fatalf("expected rule-set objects, got %v", entry)
		}
		converted = append(converted, ruleSet)
	}
	return converted
}

func TestRepairRuleSetHTTPClients(t *testing.T) {
	db := openTestDB(t)
	broken := `{
		"route": {
			"rule_set": [
				{"type": "remote", "tag": "a", "url": "https://e.com/a.srs",
				 "http_client": {"detour": "direct", "disable_empty_direct_check": true}},
				{"type": "remote", "tag": "b", "url": "https://e.com/b.srs",
				 "http_client": {"detour": "proxy", "disable_empty_direct_check": true}},
				{"type": "remote", "tag": "c", "url": "https://e.com/c.srs",
				 "http_client": {"detour": "proxy"}},
				{"type": "remote", "tag": "d", "url": "https://e.com/d.srs"}
			]
		}
	}`
	if err := db.Create(&model.Setting{Key: "config", Value: broken}).Error; err != nil {
		t.Fatal(err)
	}

	if err := repairRuleSetHTTPClients(db); err != nil {
		t.Fatal(err)
	}
	sets := ruleSets(t, readConfig(t, db))
	if len(sets) != 4 {
		t.Fatalf("expected 4 rule sets, got %v", sets)
	}

	// Nothing worth keeping was left, so the whole block goes.
	if _, ok := sets[0]["http_client"]; ok {
		t.Errorf("a direct detour with no other option must drop http_client, got %v", sets[0])
	}
	// A real detour survives; only the unparseable flag is stripped.
	fixed, ok := sets[1]["http_client"].(map[string]any)
	if !ok {
		t.Fatalf("expected http_client to survive, got %v", sets[1])
	}
	if fixed["detour"] != "proxy" {
		t.Errorf("the detour must be kept, got %v", fixed)
	}
	if _, ok = fixed["disable_empty_direct_check"]; ok {
		t.Errorf("the unparseable flag must be gone, got %v", fixed)
	}
	// Already-valid entries are untouched.
	untouched, _ := sets[2]["http_client"].(map[string]any)
	if untouched["detour"] != "proxy" || len(untouched) != 1 {
		t.Errorf("a valid http_client must be left alone, got %v", untouched)
	}
	if _, ok = sets[3]["http_client"]; ok {
		t.Errorf("a rule-set without http_client must not grow one, got %v", sets[3])
	}
}

func openHysteriaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&model.Inbound{}, &model.Outbound{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func inboundOptions(t *testing.T, db *gorm.DB, id uint) (map[string]any, map[string]any) {
	t.Helper()
	var stored model.Inbound
	if err := db.Where("id = ?", id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var options, outJson map[string]any
	if err := json.Unmarshal(stored.Options, &options); err != nil {
		t.Fatal(err)
	}
	if len(stored.OutJson) > 0 {
		if err := json.Unmarshal(stored.OutJson, &outJson); err != nil {
			t.Fatal(err)
		}
	}
	return options, outJson
}

// An inbound and an outbound disagree on which window is the stream one, so
// the two directions are renamed differently.
func TestMigrateHysteriaQUICFields(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "hysteria", Tag: "hy-in",
		Options: json.RawMessage(`{
			"listen": "::", "listen_port": 443, "up_mbps": 100,
			"recv_window_conn": 15728640,
			"recv_window_client": 67108864,
			"max_conn_client": 1024,
			"disable_mtu_discovery": true
		}`),
		OutJson: json.RawMessage(`{"type": "hysteria", "recv_window": 67108864}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Outbound{
		Type: "hysteria", Tag: "hy-out",
		Options: json.RawMessage(`{
			"server": "example.com", "server_port": 443,
			"recv_window_conn": 15728640,
			"recv_window": 67108864,
			"disable_mtu_discovery": true
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateHysteriaQUICFields(db); err != nil {
		t.Fatal(err)
	}

	options, outJson := inboundOptions(t, db, 1)
	if options["connection_receive_window"] != float64(15728640) {
		t.Errorf("recv_window_conn should become connection_receive_window, got %v", options)
	}
	if options["stream_receive_window"] != float64(67108864) {
		t.Errorf("recv_window_client should become stream_receive_window, got %v", options)
	}
	if options["max_concurrent_streams"] != float64(1024) {
		t.Errorf("max_conn_client should become max_concurrent_streams, got %v", options)
	}
	if options["disable_path_mtu_discovery"] != true {
		t.Errorf("disable_mtu_discovery should be renamed, got %v", options)
	}
	for _, deprecated := range []string{"recv_window_conn", "recv_window_client", "max_conn_client", "disable_mtu_discovery"} {
		if _, ok := options[deprecated]; ok {
			t.Errorf("%q must not survive, got %v", deprecated, options)
		}
	}
	// The client-side copy is an outbound, where recv_window is the stream one.
	if outJson["stream_receive_window"] != float64(67108864) {
		t.Errorf("the out_json copy must be migrated too, got %v", outJson)
	}

	var storedOut model.Outbound
	if err := db.Where("id = ?", 1).First(&storedOut).Error; err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(storedOut.Options, &out); err != nil {
		t.Fatal(err)
	}
	if out["stream_receive_window"] != float64(67108864) {
		t.Errorf("an outbound's recv_window is the stream window, got %v", out)
	}
	if out["connection_receive_window"] != float64(15728640) {
		t.Errorf("recv_window_conn should become connection_receive_window, got %v", out)
	}
}

// sing-box reads the deprecated name only when the QUIC field is unset, so a
// config carrying both must keep the QUIC value.
func TestMigrateHysteriaQUICFieldsKeepsExisting(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "hysteria", Tag: "hy-in",
		Options: json.RawMessage(`{
			"recv_window_client": 1,
			"stream_receive_window": 67108864
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateHysteriaQUICFields(db); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, db, 1)
	if options["stream_receive_window"] != float64(67108864) {
		t.Errorf("the QUIC field must win, got %v", options)
	}
	if _, ok := options["recv_window_client"]; ok {
		t.Errorf("the deprecated name must still go, got %v", options)
	}
}

// Other protocols carry none of these names and must not be touched.
func TestMigrateHysteriaQUICFieldsIgnoresOtherTypes(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "hysteria2", Tag: "hy2-in",
		Options: json.RawMessage(`{"up_mbps": 100, "recv_window_conn": 1}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateHysteriaQUICFields(db); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, db, 1)
	if options["recv_window_conn"] != float64(1) {
		t.Errorf("a hysteria2 inbound must be left alone, got %v", options)
	}
}

func TestMigrateRemovedOptions(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "tun", Tag: "tun-in",
		Options: json.RawMessage(`{"address": ["172.19.0.1/30"], "mtu": 9000, "endpoint_independent_nat": false}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tls{
		Name:   "site",
		Server: json.RawMessage(`{"enabled": true}`),
		Client: json.RawMessage(`{
			"enabled": true,
			"ech": {"enabled": true, "pq_signature_schemes_enabled": false, "dynamic_record_sizing_disabled": false, "config_path": "/e.pem"}
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateRemovedOptions(db); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, db, 1)
	if _, ok := options["endpoint_independent_nat"]; ok {
		t.Errorf("the removed tun option must go, got %v", options)
	}
	if options["mtu"] != float64(9000) {
		t.Errorf("the rest of the options must survive, got %v", options)
	}

	var stored model.Tls
	if err := db.Where("id = ?", 1).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var client map[string]any
	if err := json.Unmarshal(stored.Client, &client); err != nil {
		t.Fatal(err)
	}
	ech, ok := client["ech"].(map[string]any)
	if !ok {
		t.Fatalf("the ech block must survive, got %v", client)
	}
	for _, removed := range []string{"pq_signature_schemes_enabled", "dynamic_record_sizing_disabled"} {
		if _, present := ech[removed]; present {
			t.Errorf("%q must go, got %v", removed, ech)
		}
	}
	if ech["enabled"] != true || ech["config_path"] != "/e.pem" {
		t.Errorf("the settings that still apply must survive, got %v", ech)
	}
}

// Objects carrying none of them must not be rewritten.
func TestMigrateRemovedOptionsLeavesCleanObjects(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "tun", Tag: "tun-in",
		Options: json.RawMessage(`{"address":["172.19.0.1/30"],"mtu":9000}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateRemovedOptions(db); err != nil {
		t.Fatal(err)
	}

	var stored model.Inbound
	if err := db.Where("id = ?", 1).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if string(stored.Options) != `{"address":["172.19.0.1/30"],"mtu":9000}` {
		t.Errorf("a clean object must be left byte for byte, got %s", stored.Options)
	}
}

// The version row decides whether a step runs, so a database whose version
// lags behind what it has already been through gets the step a second time.
// Replaying it must leave the config exactly as the first pass left it.
func TestMigrateSingBox114Replayed(t *testing.T) {
	db := openTestDB(t)
	legacy := `{"dns": {"independent_cache": true}, "route": {"rule_set": [{"tag": "a", "download_detour": "proxy"}]}}`
	if err := db.Create(&model.Setting{Key: "config", Value: legacy}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSingBox114(db); err != nil {
		t.Fatal(err)
	}
	first := rawConfig(t, db)

	if err := migrateSingBox114(db); err != nil {
		t.Fatal(err)
	}
	if second := rawConfig(t, db); second != first {
		t.Errorf("replaying the migration changed the config:\n got %s\nwant %s", second, first)
	}
}

// Replaying the step must not hoist the provider a second time, which would
// leave two definitions of it and point the TLS config at the newer one.
func TestMigrateCertificateProvidersReplayed(t *testing.T) {
	db := openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"log":{"level":"info"}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tls{
		Name:   "site",
		Server: json.RawMessage(`{"enabled": true, "certificate_provider": {"type": "acme", "domain": ["example.com"]}}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateCertificateProviders(db); err != nil {
		t.Fatal(err)
	}
	first := rawConfig(t, db)

	if err := migrateCertificateProviders(db); err != nil {
		t.Fatal(err)
	}
	if providers := certificateProviders(t, readConfig(t, db)); len(providers) != 1 {
		t.Fatalf("expected the one hoisted provider, got %v", providers)
	}
	if second := rawConfig(t, db); second != first {
		t.Errorf("replaying the migration changed the config:\n got %s\nwant %s", second, first)
	}
}

// A repaired http_client carries nothing left to repair, so a replay is a
// no-op rather than a second rewrite.
func TestRepairRuleSetHTTPClientsReplayed(t *testing.T) {
	db := openTestDB(t)
	broken := `{"route": {"rule_set": [
		{"type": "remote", "tag": "a", "url": "https://e.com/a.srs",
		 "http_client": {"detour": "direct", "disable_empty_direct_check": true}}
	]}}`
	if err := db.Create(&model.Setting{Key: "config", Value: broken}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairRuleSetHTTPClients(db); err != nil {
		t.Fatal(err)
	}
	first := rawConfig(t, db)

	if err := repairRuleSetHTTPClients(db); err != nil {
		t.Fatal(err)
	}
	if second := rawConfig(t, db); second != first {
		t.Errorf("replaying the repair changed the config:\n got %s\nwant %s", second, first)
	}
}
