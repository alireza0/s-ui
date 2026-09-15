package migration

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openLegacyDB builds a database where MigrateDb will look for one, with the
// tables a 1.5.2 install had. The handle is closed before MigrateDb opens its
// own, and reopenDB picks the same file up again afterwards.
func openLegacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("SUI_DB_FOLDER", t.TempDir())

	db := reopenDB(t)
	if err := db.AutoMigrate(&model.Setting{}, &model.Tls{}, &model.Inbound{},
		&model.Outbound{}, &model.Client{}, &legacyEndpoint{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func reopenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(config.GetDBPath()))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err = sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}

// The whole chain, in the transaction MigrateDb runs it in: a 1.5.2 database
// must come out with the 1.6 work done, the stale per-migration flags gone and
// the version row telling the next run to skip all of it.
func TestMigrateDbFrom152(t *testing.T) {
	db := openLegacyDB(t)

	rows := []any{
		&model.Setting{Key: "version", Value: "1.5.2"},
		&model.Setting{Key: "config", Value: `{
			"dns": {"independent_cache": true},
			"route": {"rule_set": [{"tag": "geoip", "download_detour": "proxy"}]}
		}`},
		// Left over from the releases that flagged each migration instead of
		// reading the version row.
		&model.Setting{Key: "migratedSingBox114", Value: "true"},
		&model.Tls{Name: "site", Server: json.RawMessage(`{"enabled":true}`), Client: json.RawMessage(`{}`)},
		&model.Inbound{Type: "hysteria", Tag: "hy", Options: json.RawMessage(`{"recv_window_conn":15728640}`)},
		&model.Client{Name: "someone", Enable: true, Config: json.RawMessage(`{"vless":{"name":"someone"}}`),
			Inbounds: json.RawMessage(`[]`), Links: json.RawMessage(`[]`)},
		&legacyEndpoint{Type: "openvpn-client", Tag: "ovpn", TlsId: 1,
			Options: json.RawMessage(`{"server":"vpn.example.com","mode":"tls","cipher":"AES-256-CBC"}`)},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seeding %T: %v", row, err)
		}
	}
	closeDB(t, db)

	if err := MigrateDb(); err != nil {
		t.Fatal(err)
	}

	db = reopenDB(t)
	defer closeDB(t, db)

	// 1.6.0: the deprecated names are gone from the config and the inbound.
	root := readConfig(t, db)
	if _, ok := section(t, root, "dns")["independent_cache"]; ok {
		t.Error("independent_cache should have been dropped")
	}
	options, _ := inboundOptions(t, db, 1)
	if options["connection_receive_window"] != float64(15728640) {
		t.Errorf("the hysteria window was not renamed: %v", options)
	}

	// 1.6.1: the endpoint carries its own TLS block and lost the option its
	// mode rejects.
	endpoint := endpointOptions(t, db, 1)
	if _, ok := endpoint["cipher"]; ok {
		t.Errorf("cipher does not belong to an openvpn endpoint in tls mode: %v", endpoint)
	}

	// 1.6.3: the client has Snell credentials and the stale flag is gone.
	if _, ok := clientConfig(t, db, 1)["snell"]; !ok {
		t.Error("the client got no snell credentials")
	}
	var flags int64
	if err := db.Model(&model.Setting{}).Where("key LIKE ?", "migrated%").Count(&flags).Error; err != nil {
		t.Fatal(err)
	}
	if flags != 0 {
		t.Errorf("%d per-migration flag(s) survived", flags)
	}

	version, _, err := readSetting(db, "version")
	if err != nil {
		t.Fatal(err)
	}
	if version != config.GetVersion() {
		t.Errorf("version = %q, want %q", version, config.GetVersion())
	}
}
