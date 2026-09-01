package util

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// https://github.com/alireza0/s-ui/issues/1243
// Editing a naive inbound and clearing QUIC Congestion Control must remove
// the stale quic / quic_congestion_control keys from the stored out_json.
func TestFillOutJsonNaiveClearsStaleQuic(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "naive",
		Tag:     "naive-in",
		Options: json.RawMessage(`{"listen_port": 443}`),
		// out_json left over from a previous save with QUIC enabled
		OutJson: json.RawMessage(`{"quic": true, "quic_congestion_control": "bbr"}`),
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	if v, ok := out["quic"]; ok {
		t.Errorf("quic should be removed when quic_congestion_control is cleared, got %v", v)
	}
	if v, ok := out["quic_congestion_control"]; ok {
		t.Errorf("quic_congestion_control should be removed when cleared, got %v", v)
	}
}

func TestFillOutJsonNaiveSetsQuic(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "naive",
		Tag:     "naive-in",
		Options: json.RawMessage(`{"listen_port": 443, "quic_congestion_control": "bbr_standard"}`),
		OutJson: json.RawMessage(`{}`),
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	if out["quic"] != true {
		t.Errorf("quic = %v, want true", out["quic"])
	}
	if out["quic_congestion_control"] != "bbr" {
		t.Errorf("quic_congestion_control = %v, want bbr (mapped from bbr_standard)", out["quic_congestion_control"])
	}
}
