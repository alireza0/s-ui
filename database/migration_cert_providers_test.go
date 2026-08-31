package database

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

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

func tlsServer(t *testing.T, id uint) map[string]any {
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
	openTestDB(t)
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

	if err := migrateCertificateProviders(); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t))
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
	if got := tlsServer(t, 1)["certificate_provider"]; got != tag {
		t.Errorf("expected the tag %q as the reference, got %v", tag, got)
	}
}

// A tag reference is already the new form and must be left alone, as must a
// provider that was defined in the config by hand.
func TestMigrateCertificateProvidersLeavesReferences(t *testing.T) {
	openTestDB(t)
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

	if err := migrateCertificateProviders(); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t))
	if len(providers) != 1 {
		t.Errorf("nothing should have been added, got %v", providers)
	}
	if got := tlsServer(t, 1)["certificate_provider"]; got != "shared" {
		t.Errorf("the reference must be untouched, got %v", got)
	}
}

// Two TLS configs whose names slugify the same must not end up sharing a tag,
// which would silently point one of them at the other's provider.
func TestMigrateCertificateProvidersDeduplicatesTags(t *testing.T) {
	openTestDB(t)
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

	if err := migrateCertificateProviders(); err != nil {
		t.Fatal(err)
	}

	providers := certificateProviders(t, readConfig(t))
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
		reference, _ := tlsServer(t, id)["certificate_provider"].(string)
		if reference == "my-site" {
			t.Errorf("tls %d must not claim the existing provider's tag", id)
		}
		if !tags[reference] {
			t.Errorf("tls %d references %q, which is not defined", id, reference)
		}
	}
}

// The migration runs once; a provider added inline afterwards is the operator's
// own doing and must not be moved out from under them on the next start.
func TestMigrateCertificateProvidersRunsOnce(t *testing.T) {
	openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"log":{"level":"info"}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateCertificateProviders(); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tls{
		Name:   "later",
		Server: json.RawMessage(`{"enabled": true, "certificate_provider": {"type": "acme", "domain": ["example.com"]}}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateCertificateProviders(); err != nil {
		t.Fatal(err)
	}

	if _, hoisted := readConfig(t)["certificate_providers"]; hoisted {
		t.Error("the migration should not run a second time")
	}
}
