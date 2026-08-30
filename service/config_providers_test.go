package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// TestGetConfigEmitsCertificateProviders covers the whole path the TLS page
// relies on: providers saved into the base config have to come out in the
// config handed to sing-box, and a TLS config's tag reference has to survive
// alongside them.
func TestGetConfigEmitsCertificateProviders(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()

	base := `{
		"log": {"level": "info"},
		"certificate_providers": [
			{"type": "acme", "tag": "letsencrypt", "domain": ["example.com"], "email": "a@example.com"}
		],
		"route": {"rules": []}
	}`
	// The defaults are only materialised on demand, so the row may not exist yet.
	if err := db.Where("key = ?", "config").Delete(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Setting{Key: "config", Value: base}).Error; err != nil {
		t.Fatal(err)
	}

	tlsConfig := model.Tls{
		Name:   "site",
		Server: json.RawMessage(`{"enabled": true, "certificate_provider": "letsencrypt"}`),
		Client: json.RawMessage(`{}`),
	}
	if err := db.Create(&tlsConfig).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{
		Type:    "vless",
		Tag:     "vless-in",
		TlsId:   tlsConfig.Id,
		Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":42447}`),
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}

	raw, err := (&ConfigService{}).GetConfig("")
	if err != nil {
		t.Fatal(err)
	}

	var built struct {
		CertificateProviders []struct {
			Type   string   `json:"type"`
			Tag    string   `json:"tag"`
			Domain []string `json:"domain"`
		} `json:"certificate_providers"`
		Inbounds []struct {
			Tag string `json:"tag"`
			TLS struct {
				CertificateProvider string `json:"certificate_provider"`
			} `json:"tls"`
		} `json:"inbounds"`
	}
	if err = json.Unmarshal(*raw, &built); err != nil {
		t.Fatal(err)
	}

	if len(built.CertificateProviders) != 1 {
		t.Fatalf("the provider must reach the generated config, got %s", *raw)
	}
	provider := built.CertificateProviders[0]
	if provider.Tag != "letsencrypt" || provider.Type != "acme" || len(provider.Domain) != 1 {
		t.Errorf("the provider must be emitted unchanged, got %+v", provider)
	}
	if len(built.Inbounds) != 1 {
		t.Fatalf("expected the inbound, got %s", *raw)
	}
	if built.Inbounds[0].TLS.CertificateProvider != "letsencrypt" {
		t.Errorf("the inbound TLS must reference the provider, got %s", *raw)
	}
}
