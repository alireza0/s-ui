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

// handshake_timeout is entered once on the server side, so out_json has to
// carry it over to the client config the same way it does the TLS versions.
// Client-only keys such as spoof stay untouched.
func TestFillOutJsonTlsHandshakeTimeout(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "vless",
		Tag:     "vless-in",
		Options: json.RawMessage(`{"listen_port": 443}`),
		OutJson: json.RawMessage(`{}`),
		TlsId:   1,
		Tls: &model.Tls{
			Id:     1,
			Name:   "tls",
			Server: json.RawMessage(`{"enabled": true, "handshake_timeout": "20s"}`),
			Client: json.RawMessage(`{"spoof": "allowed.example.com", "spoof_method": "wrong-checksum"}`),
		},
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	tls, ok := out["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls is missing from out_json: %s", inbound.OutJson)
	}
	if tls["handshake_timeout"] != "20s" {
		t.Errorf("handshake_timeout should be copied to the client config, got %v", tls["handshake_timeout"])
	}
	if tls["spoof"] != "allowed.example.com" {
		t.Errorf("spoof should be kept, got %v", tls["spoof"])
	}
	if tls["spoof_method"] != "wrong-checksum" {
		t.Errorf("spoof_method should be kept, got %v", tls["spoof_method"])
	}
}

func TestFillOutJsonSnellV6(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "snell",
		Tag:     "snell-in",
		Options: json.RawMessage(`{"listen_port": 443, "version": 6, "psk": "sharedpsk", "mode": "unshaped"}`),
		// What a freshly created inbound carries before its first save.
		OutJson: json.RawMessage(`null`),
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	if out["version"] != float64(6) {
		t.Errorf("version = %v, want 6", out["version"])
	}
	if out["psk"] != "sharedpsk" {
		t.Errorf("psk = %v, want sharedpsk", out["psk"])
	}
	if out["mode"] != "unshaped" {
		t.Errorf("mode = %v, want unshaped", out["mode"])
	}
	if out["server"] != "example.com" {
		t.Errorf("server = %v, want example.com", out["server"])
	}
}

// sing-box numbers the two ends of one protocol generation differently: the
// client that speaks to a version 5 inbound is a version 4 outbound.
func TestFillOutJsonSnellV5MapsToClientVersion4(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "snell",
		Tag:     "snell-in",
		Options: json.RawMessage(`{"listen_port": 443, "version": 5, "psk": "sharedpsk", "obfs_mode": "http"}`),
		OutJson: json.RawMessage(`null`),
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	if out["version"] != float64(4) {
		t.Errorf("version = %v, want 4", out["version"])
	}
	if out["obfs_mode"] != "http" {
		t.Errorf("obfs_mode = %v, want http", out["obfs_mode"])
	}
	if _, ok := out["mode"]; ok {
		t.Error("mode belongs to version 6 and must not appear on a version 4 client")
	}
}

// Switching an existing inbound from version 5 to 6 must not leave the obfs
// options of the old version behind.
func TestFillOutJsonSnellClearsStaleVersionOptions(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "snell",
		Tag:     "snell-in",
		Options: json.RawMessage(`{"listen_port": 443, "version": 6, "psk": "sharedpsk"}`),
		OutJson: json.RawMessage(`{"version": 4, "obfs_mode": "tls", "obfs_host": "bing.com"}`),
	}

	if err := FillOutJson(inbound, "example.com"); err != nil {
		t.Fatal(err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &out); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"obfs_mode", "obfs_host"} {
		if v, ok := out[key]; ok {
			t.Errorf("%s survived the switch to version 6: %v", key, v)
		}
	}
	if out["version"] != float64(6) {
		t.Errorf("version = %v, want 6", out["version"])
	}
}
