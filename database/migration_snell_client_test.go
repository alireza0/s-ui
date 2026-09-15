package database

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func openSnellTestDB(t *testing.T) {
	t.Helper()
	openTestDB(t)
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatal(err)
	}
}

func clientConfig(t *testing.T, id uint) map[string]map[string]any {
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

func createTestClient(t *testing.T, name, config string) *model.Client {
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
	openSnellTestDB(t)

	alice := createTestClient(t, "alice", `{"vless":{"name":"alice","uuid":"b831381d-6324-4d53-ad4f-8cda48b30811"}}`)
	bob := createTestClient(t, "bob", `{"trojan":{"name":"bob","password":"hunter2"}}`)

	if err := addSnellClientConfig(); err != nil {
		t.Fatal(err)
	}

	var keys []string
	for _, c := range []*model.Client{alice, bob} {
		config := clientConfig(t, c.Id)
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
	openSnellTestDB(t)

	c := createTestClient(t, "alice", `{"snell":{"name":"alice","userkey":"keepthisexactkey"}}`)

	if err := addSnellClientConfig(); err != nil {
		t.Fatal(err)
	}

	if got := clientConfig(t, c.Id)["snell"]["userkey"]; got != "keepthisexactkey" {
		t.Errorf("userkey = %v, want it untouched", got)
	}
}

// It runs once. A second start must not replace the keys handed out by the
// first, which subscribers are already using.
func TestAddSnellClientConfigRunsOnce(t *testing.T) {
	openSnellTestDB(t)

	c := createTestClient(t, "alice", `{}`)

	if err := addSnellClientConfig(); err != nil {
		t.Fatal(err)
	}
	first := clientConfig(t, c.Id)["snell"]["userkey"]

	if err := addSnellClientConfig(); err != nil {
		t.Fatal(err)
	}
	if second := clientConfig(t, c.Id)["snell"]["userkey"]; second != first {
		t.Errorf("userkey changed on the second run: %v -> %v", first, second)
	}
}

// A config no one can parse is skipped, not fatal: the other clients still get
// their keys.
func TestAddSnellClientConfigSkipsUnparsableConfig(t *testing.T) {
	openSnellTestDB(t)

	broken := createTestClient(t, "broken", `not json`)
	ok := createTestClient(t, "ok", `{}`)

	if err := addSnellClientConfig(); err != nil {
		t.Fatal(err)
	}

	if _, exists := clientConfig(t, ok.Id)["snell"]; !exists {
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
