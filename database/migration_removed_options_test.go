package database

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestMigrateRemovedOptions(t *testing.T) {
	openHysteriaTestDB(t)
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

	if err := migrateRemovedOptions(); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, 1)
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
	openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "tun", Tag: "tun-in",
		Options: json.RawMessage(`{"address":["172.19.0.1/30"],"mtu":9000}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateRemovedOptions(); err != nil {
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
