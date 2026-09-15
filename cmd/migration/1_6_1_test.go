package migration

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// legacyEndpoint mirrors the endpoints table as it was while endpoints pointed
// at a shared TLS config, so the migration has a column to read.
type legacyEndpoint struct {
	Id      uint `gorm:"primaryKey;autoIncrement"`
	Type    string
	Tag     string
	TlsId   uint
	Options json.RawMessage
	Ext     json.RawMessage
}

func (legacyEndpoint) TableName() string { return "endpoints" }

func openEndpointTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&legacyEndpoint{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func endpointOptions(t *testing.T, db *gorm.DB, id uint) map[string]any {
	t.Helper()
	var stored legacyEndpoint
	if err := db.Where("id = ?", id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TlsId != 0 {
		t.Errorf("tls_id = %d, want it cleared", stored.TlsId)
	}
	var options map[string]any
	if err := json.Unmarshal(stored.Options, &options); err != nil {
		t.Fatal(err)
	}
	return options
}

func TestMigrateEndpointTlsOpenVPNClient(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name: "vpn",
		Client: json.RawMessage(`{
			"enabled": true,
			"server_name": "vpn.example.com",
			"certificate_path": "/etc/ssl/ca.crt",
			"client_certificate_path": "/etc/ssl/client.crt",
			"client_key_path": "/etc/ssl/client.key",
			"min_version": "1.2"
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		TlsId:   1,
		Options: json.RawMessage(`{"server":"vpn.example.com","server_port":1194,"mode":"tls"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if options["server"] != "vpn.example.com" {
		t.Errorf("server was lost: %v", options)
	}
	tls, ok := options["tls"].(map[string]any)
	if !ok {
		t.Fatalf("no inline tls object: %v", options)
	}
	if tls["certificate_path"] != "/etc/ssl/ca.crt" {
		t.Errorf("certificate_path = %v", tls["certificate_path"])
	}
	if tls["client_key_path"] != "/etc/ssl/client.key" {
		t.Errorf("client_key_path = %v", tls["client_key_path"])
	}
	// The panel calls it min_version; OpenVPN calls it version_min.
	if tls["version_min"] != "1.2" {
		t.Errorf("version_min = %v", tls["version_min"])
	}
	// `enabled` has no counterpart and sing-box rejects unknown keys.
	if _, exists := tls["enabled"]; exists {
		t.Errorf("enabled was carried over: %v", tls)
	}
}

func TestMigrateEndpointTlsOpenVPNServerClientAuth(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name: "vpn",
		Server: json.RawMessage(`{
			"enabled": true,
			"certificate_path": "/etc/ssl/server.crt",
			"key_path": "/etc/ssl/server.key",
			"client_certificate_path": ["/etc/ssl/clients-ca.crt", "/etc/ssl/other-ca.crt"],
			"client_authentication": "require-and-verify"
		}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-server",
		Tag:     "ovpn-srv",
		TlsId:   1,
		Options: json.RawMessage(`{"listen":"::","listen_port":1194,"mode":"tls"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	tls, ok := endpointOptions(t, db, 1)["tls"].(map[string]any)
	if !ok {
		t.Fatal("no inline tls object")
	}
	// OpenVPN takes a single client CA where a sing-box inbound takes a list.
	if tls["client_certificate_path"] != "/etc/ssl/clients-ca.crt" {
		t.Errorf("client_certificate_path = %v", tls["client_certificate_path"])
	}
	if tls["verify_client_certificate"] != "require" {
		t.Errorf("verify_client_certificate = %v", tls["verify_client_certificate"])
	}
}

func TestMigrateEndpointTlsOpenConnectRenamesTrustAnchor(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name:   "oc",
		Client: json.RawMessage(`{"enabled":true,"certificate_path":"/etc/ssl/ca.crt","insecure":true}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "openconnect",
		Tag:     "oc",
		TlsId:   1,
		Options: json.RawMessage(`{"server":"vpn.example.com"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	tls, ok := endpointOptions(t, db, 1)["tls"].(map[string]any)
	if !ok {
		t.Fatal("no inline tls object")
	}
	if tls["certificate_authority_path"] != "/etc/ssl/ca.crt" {
		t.Errorf("certificate_authority_path = %v", tls["certificate_authority_path"])
	}
	if tls["insecure"] != true {
		t.Errorf("insecure = %v", tls["insecure"])
	}
}

func TestMigrateEndpointTlsKeepsExistingInlineTls(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name:   "vpn",
		Client: json.RawMessage(`{"enabled":true,"certificate_path":"/etc/ssl/template-ca.crt"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		TlsId:   1,
		Options: json.RawMessage(`{"server":"vpn.example.com","tls":{"certificate_path":"/etc/ssl/own-ca.crt"}}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	tls := endpointOptions(t, db, 1)["tls"].(map[string]any)
	if tls["certificate_path"] != "/etc/ssl/own-ca.crt" {
		t.Errorf("the endpoint's own tls block was overwritten: %v", tls)
	}
}

func TestMigrateEndpointTlsClearsDanglingReference(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		TlsId:   7,
		Options: json.RawMessage(`{"server":"vpn.example.com"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if _, exists := options["tls"]; exists {
		t.Errorf("a missing TLS config produced a tls block: %v", options)
	}
}

func TestMigrateEndpointTlsLeavesOtherTypesAlone(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name:   "vpn",
		Client: json.RawMessage(`{"enabled":true,"certificate_path":"/etc/ssl/ca.crt"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "wireguard",
		Tag:     "wg",
		TlsId:   1,
		Options: json.RawMessage(`{"listen_port":51820}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if _, exists := options["tls"]; exists {
		t.Errorf("wireguard was given a tls block: %v", options)
	}
}

// A database created after the tls_id column was dropped has no column to read,
// which must not stop the panel from starting.
func TestMigrateEndpointTlsWithoutColumn(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&model.Endpoint{}); err != nil {
		t.Fatal(err)
	}
	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateEndpointTlsDropsCipherInTlsMode(t *testing.T) {
	db := openEndpointTestDB(t)
	// The panel used to offer `cipher` in both modes, and sing-box rejects it
	// in TLS mode, so the endpoint never started.
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		Options: json.RawMessage(`{"server":"vpn.example.com","mode":"tls","cipher":"AES-256-GCM","auth":"SHA256"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if _, exists := options["cipher"]; exists {
		t.Errorf("cipher survived in tls mode: %v", options)
	}
	if options["auth"] != "SHA256" {
		t.Errorf("auth was dropped with it: %v", options)
	}
}

func TestMigrateEndpointTlsKeepsCipherInStaticKeyMode(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		Options: json.RawMessage(`{"server":"vpn.example.com","mode":"static_key","cipher":"AES-256-CBC","static_key_path":"/etc/openvpn/static.key"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if options["cipher"] != "AES-256-CBC" {
		t.Errorf("cipher = %v, want it kept in static_key mode", options["cipher"])
	}
	if options["static_key_path"] != "/etc/openvpn/static.key" {
		t.Errorf("static_key_path = %v", options["static_key_path"])
	}
}

func TestMigrateEndpointTlsDropsPeerAddressOnTlsServer(t *testing.T) {
	db := openEndpointTestDB(t)
	// In TLS mode a server pushes addresses to whichever clients connect, so
	// sing-box refuses the single peer a static_key tunnel would name.
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-server",
		Tag:     "ovpn-srv",
		Options: json.RawMessage(`{"listen":"::","listen_port":1194,"mode":"tls","peer_address":"10.8.0.2","address":["10.8.0.1/24"]}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	options := endpointOptions(t, db, 1)
	if _, exists := options["peer_address"]; exists {
		t.Errorf("peer_address survived on a tls-mode server: %v", options)
	}
}

func TestMigrateEndpointTlsLeavesUntouchedRowsAlone(t *testing.T) {
	db := openEndpointTestDB(t)
	original := `{"server":"vpn.example.com","mode":"tls","auth":"SHA256"}`
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		Options: json.RawMessage(original),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	var stored legacyEndpoint
	if err := db.Where("id = ?", 1).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if string(stored.Options) != original {
		t.Errorf("a row with nothing to fix was rewritten:\n%s", stored.Options)
	}
}

func TestMigrateEndpointTlsSettlesClientCertificatePolicy(t *testing.T) {
	db := openEndpointTestDB(t)
	// sing-box reads a missing verify_client_certificate as "require", so this
	// server demands certificates it has no CA to check, and refuses to start.
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-server",
		Tag:     "ovpn-srv",
		Options: json.RawMessage(`{"listen":"::","listen_port":1194,"mode":"tls","address":["10.8.0.1/24"],"tls":{"certificate_path":"/etc/ssl/s.crt","key_path":"/etc/ssl/s.key"}}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	tls, ok := endpointOptions(t, db, 1)["tls"].(map[string]any)
	if !ok {
		t.Fatal("tls block went missing")
	}
	if tls["verify_client_certificate"] != "none" {
		t.Errorf("verify_client_certificate = %v, want none", tls["verify_client_certificate"])
	}
	if tls["certificate_path"] != "/etc/ssl/s.crt" {
		t.Errorf("the certificate was lost: %v", tls)
	}
}

func TestMigrateEndpointTlsKeepsStatedClientCertificatePolicy(t *testing.T) {
	db := openEndpointTestDB(t)
	for _, endpoint := range []legacyEndpoint{
		// Asked for client certificates in so many words.
		{Type: "openvpn-server", Tag: "stated", Options: json.RawMessage(
			`{"mode":"tls","tls":{"certificate_path":"/etc/ssl/s.crt","key_path":"/etc/ssl/s.key","verify_client_certificate":"require","client_certificate_path":"/etc/ssl/ca.crt"}}`)},
		// Has a CA, so the "require" it gets by default is what it wants.
		{Type: "openvpn-server", Tag: "hasca", Options: json.RawMessage(
			`{"mode":"tls","tls":{"certificate_path":"/etc/ssl/s.crt","key_path":"/etc/ssl/s.key","client_certificate_path":"/etc/ssl/ca.crt"}}`)},
		// Pins the peer instead of naming a CA, which satisfies sing-box too.
		{Type: "openvpn-server", Tag: "pinned", Options: json.RawMessage(
			`{"mode":"tls","tls":{"certificate_path":"/etc/ssl/s.crt","key_path":"/etc/ssl/s.key","peer_fingerprint":["ab"]}}`)},
		// static_key mode never reaches the TLS credential checks.
		{Type: "openvpn-server", Tag: "static", Options: json.RawMessage(
			`{"mode":"static_key","static_key_path":"/etc/openvpn/static.key","tls":{"certificate_path":"/etc/ssl/s.crt"}}`)},
	} {
		if err := db.Create(&endpoint).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}

	for id := uint(1); id <= 4; id++ {
		tls, ok := endpointOptions(t, db, id)["tls"].(map[string]any)
		if !ok {
			t.Fatalf("endpoint %d lost its tls block", id)
		}
		want := any(nil)
		if id == 1 {
			want = "require"
		}
		if tls["verify_client_certificate"] != want {
			t.Errorf("endpoint %d: verify_client_certificate = %v, want %v", id, tls["verify_client_certificate"], want)
		}
	}
}

// Replaying the step against a database that has already been through it must
// not fail, and must not give an endpoint a second, different TLS block.
func TestMigrateEndpointTlsReplayed(t *testing.T) {
	db := openEndpointTestDB(t)
	if err := db.Create(&model.Tls{
		Name:   "vpn",
		Client: json.RawMessage(`{"enabled": true, "server_name": "vpn.example.com"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyEndpoint{
		Type:    "openvpn-client",
		Tag:     "ovpn",
		TlsId:   1,
		Options: json.RawMessage(`{"server":"vpn.example.com","mode":"tls"}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}
	var first legacyEndpoint
	if err := db.Where("id = ?", 1).First(&first).Error; err != nil {
		t.Fatal(err)
	}

	if err := to1_6_1(db); err != nil {
		t.Fatal(err)
	}
	var second legacyEndpoint
	if err := db.Where("id = ?", 1).First(&second).Error; err != nil {
		t.Fatal(err)
	}
	if string(second.Options) != string(first.Options) {
		t.Errorf("replaying the migration changed the endpoint:\n got %s\nwant %s", second.Options, first.Options)
	}
}
