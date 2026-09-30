package migration

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// 1.6.0 cleared the legacy ECH options from the client side only. The server
// side, which the panel copied into every generated outbound, and the stored
// out_json kept them. #1275
func TestClearLegacyECHOptionsServerAndOutJson(t *testing.T) {
	db := openHysteriaTestDB(t)
	if err := db.Create(&model.Tls{
		Name:   "site",
		Server: json.RawMessage(`{"enabled": true, "ech": {"enabled": true, "key_path": "/k.pem", "pq_signature_schemes_enabled": true, "dynamic_record_sizing_disabled": false}}`),
		Client: json.RawMessage(`{}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Inbound{
		Type: "vless", Tag: "vless-in", TlsId: 1,
		Options: json.RawMessage(`{"listen_port": 443}`),
		OutJson: json.RawMessage(`{"server": "example.com", "tls": {"enabled": true, "ech": {"enabled": true, "config": ["c"], "pq_signature_schemes_enabled": true, "dynamic_record_sizing_disabled": null}}}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_4(db); err != nil {
		t.Fatal(err)
	}
	// Re-runnable: a second pass finds nothing and changes nothing.
	if err := to1_6_4(db); err != nil {
		t.Fatal(err)
	}

	var stored model.Tls
	if err := db.Where("id = ?", 1).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var server map[string]any
	if err := json.Unmarshal(stored.Server, &server); err != nil {
		t.Fatal(err)
	}
	ech, _ := server["ech"].(map[string]any)
	for _, removed := range removedECHOptions {
		if _, present := ech[removed]; present {
			t.Errorf("server ech: %q must go, got %v", removed, ech)
		}
	}
	if ech["enabled"] != true || ech["key_path"] != "/k.pem" {
		t.Errorf("server ech: the settings that still apply must survive, got %v", ech)
	}

	_, outJson := inboundOptions(t, db, 1)
	tls, _ := outJson["tls"].(map[string]any)
	outEch, _ := tls["ech"].(map[string]any)
	for _, removed := range removedECHOptions {
		if _, present := outEch[removed]; present {
			t.Errorf("out_json ech: %q must go, got %v", removed, outEch)
		}
	}
	if outEch["enabled"] != true || outJson["server"] != "example.com" {
		t.Errorf("out_json: the rest must survive, got %v", outJson)
	}
}
