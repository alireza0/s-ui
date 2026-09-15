package migration

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

func openSnellTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func clientConfig(t *testing.T, db *gorm.DB, id uint) map[string]map[string]any {
	t.Helper()
	var stored model.Client
	if err := db.Where("id = ?", id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var config map[string]map[string]any
	if err := json.Unmarshal(stored.Config, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func createTestClient(t *testing.T, db *gorm.DB, name, config string) *model.Client {
	t.Helper()
	c := &model.Client{
		Name:     name,
		Enable:   true,
		Config:   json.RawMessage(config),
		Inbounds: json.RawMessage(`[]`),
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatal(err)
	}
	return c
}

// Snell arrived as a protocol before the client system knew about it, so every
// client created until now has credentials for the other protocols and none
// for Snell -- which leaves it unable to use a Snell inbound at all. #1262
func TestAddSnellClientConfig(t *testing.T) {
	db := openSnellTestDB(t)

	alice := createTestClient(t, db, "alice", `{"vless":{"name":"alice","uuid":"b831381d-6324-4d53-ad4f-8cda48b30811"}}`)
	bob := createTestClient(t, db, "bob", `{"trojan":{"name":"bob","password":"hunter2"}}`)

	if err := addSnellClientConfig(db); err != nil {
		t.Fatal(err)
	}

	var keys []string
	for _, c := range []*model.Client{alice, bob} {
		config := clientConfig(t, db, c.Id)
		snell, ok := config["snell"]
		if !ok {
			t.Fatalf("client %q got no snell credentials", c.Name)
		}
		if snell["name"] != c.Name {
			t.Errorf("snell name = %v, want %q", snell["name"], c.Name)
		}
		key, _ := snell["userkey"].(string)
		if len(key) != snellUserKeyLength {
			t.Errorf("userkey = %q, want %d characters", key, snellUserKeyLength)
		}
		keys = append(keys, key)

		// Nothing else may be disturbed.
		if len(config) != 2 {
			t.Errorf("client %q config has %d protocols, want 2", c.Name, len(config))
		}
	}

	if keys[0] == keys[1] {
		t.Error("both clients got the same Snell key")
	}
}

// A client that already has Snell credentials keeps them: the migration must
// not hand a working client a new key and cut it off.
func TestAddSnellClientConfigKeepsExisting(t *testing.T) {
	db := openSnellTestDB(t)

	c := createTestClient(t, db, "alice", `{"snell":{"name":"alice","userkey":"keepthisexactkey"}}`)

	if err := addSnellClientConfig(db); err != nil {
		t.Fatal(err)
	}

	if got := clientConfig(t, db, c.Id)["snell"]["userkey"]; got != "keepthisexactkey" {
		t.Errorf("userkey = %v, want it untouched", got)
	}
}

// A replay must not replace the keys handed out by the first pass, which
// subscribers are already using.
func TestAddSnellClientConfigReplayed(t *testing.T) {
	db := openSnellTestDB(t)

	c := createTestClient(t, db, "alice", `{}`)

	if err := addSnellClientConfig(db); err != nil {
		t.Fatal(err)
	}
	first := clientConfig(t, db, c.Id)["snell"]["userkey"]

	if err := addSnellClientConfig(db); err != nil {
		t.Fatal(err)
	}
	if second := clientConfig(t, db, c.Id)["snell"]["userkey"]; second != first {
		t.Errorf("userkey changed on the second run: %v -> %v", first, second)
	}
}

// A config no one can parse is skipped, not fatal: the other clients still get
// their keys.
func TestAddSnellClientConfigSkipsUnparsableConfig(t *testing.T) {
	db := openSnellTestDB(t)

	broken := createTestClient(t, db, "broken", `not json`)
	ok := createTestClient(t, db, "ok", `{}`)

	if err := addSnellClientConfig(db); err != nil {
		t.Fatal(err)
	}

	if _, exists := clientConfig(t, db, ok.Id)["snell"]; !exists {
		t.Error("a readable client was skipped because another one was broken")
	}
	var stored model.Client
	if err := db.Where("id = ?", broken.Id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if string(stored.Config) != "not json" {
		t.Errorf("the unreadable config was rewritten: %s", stored.Config)
	}
}

// 1.6.0 through 1.6.2 ran their migrations from InitDB on every start and
// recorded a flag per migration in the settings table instead of gating on the
// version row that table already holds. The flags decide nothing now, so they
// go -- and nothing else in the table may go with them.
func TestDropMigrationFlags(t *testing.T) {
	db := openSnellTestDB(t)

	kept := map[string]string{"version": "1.6.2", "config": `{"log":{"level":"info"}}`}
	for key, value := range kept {
		if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{
		"migratedSingBox114", "migratedCertProviders", "migratedHTTPClientFix",
		"migratedHysteriaQuic", "migratedRemovedOptions", "migratedEndpointTls",
		"migratedSnellClient",
	} {
		if err := db.Create(&model.Setting{Key: key, Value: "true"}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := to1_6_3(db); err != nil {
		t.Fatal(err)
	}

	var flags int64
	if err := db.Model(&model.Setting{}).Where("key LIKE ?", "migrated%").Count(&flags).Error; err != nil {
		t.Fatal(err)
	}
	if flags != 0 {
		t.Errorf("%d per-migration flag(s) survived", flags)
	}
	for key, want := range kept {
		value, exists, err := readSetting(db, key)
		if err != nil {
			t.Fatal(err)
		}
		if !exists || value != want {
			t.Errorf("setting %q = %q (exists %v), want %q", key, value, exists, want)
		}
	}
}

// A database old enough to have no clients table must not stop the migration:
// there is nothing to give a Snell key to.
func TestAddSnellClientConfigWithoutClientsTable(t *testing.T) {
	db := openTestDB(t)
	if err := to1_6_3(db); err != nil {
		t.Fatal(err)
	}
}
