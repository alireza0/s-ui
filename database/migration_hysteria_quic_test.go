package database

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func openHysteriaTestDB(t *testing.T) {
	t.Helper()
	openTestDB(t)
	if err := db.AutoMigrate(&model.Inbound{}, &model.Outbound{}); err != nil {
		t.Fatal(err)
	}
}

func inboundOptions(t *testing.T, id uint) (map[string]any, map[string]any) {
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
	openHysteriaTestDB(t)
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

	if err := migrateHysteriaQUICFields(); err != nil {
		t.Fatal(err)
	}

	options, outJson := inboundOptions(t, 1)
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
	openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "hysteria", Tag: "hy-in",
		Options: json.RawMessage(`{
			"recv_window_client": 1,
			"stream_receive_window": 67108864
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateHysteriaQUICFields(); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, 1)
	if options["stream_receive_window"] != float64(67108864) {
		t.Errorf("the QUIC field must win, got %v", options)
	}
	if _, ok := options["recv_window_client"]; ok {
		t.Errorf("the deprecated name must still go, got %v", options)
	}
}

// Other protocols carry none of these names and must not be touched.
func TestMigrateHysteriaQUICFieldsIgnoresOtherTypes(t *testing.T) {
	openHysteriaTestDB(t)
	if err := db.Create(&model.Inbound{
		Type: "hysteria2", Tag: "hy2-in",
		Options: json.RawMessage(`{"up_mbps": 100, "recv_window_conn": 1}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateHysteriaQUICFields(); err != nil {
		t.Fatal(err)
	}

	options, _ := inboundOptions(t, 1)
	if options["recv_window_conn"] != float64(1) {
		t.Errorf("a hysteria2 inbound must be left alone, got %v", options)
	}
}
