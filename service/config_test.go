package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// GetConfig rebuilds the sing-box config by decoding the stored base config
// into SingBoxConfig and marshalling it back, so anything the struct does not
// model is dropped on the way through. certificate_providers is edited by the
// TLS page and has to survive that round trip.
func TestSingBoxConfigKeepsCertificateProviders(t *testing.T) {
	base := `{
		"log": {"level": "info"},
		"certificate_providers": [
			{"type": "acme", "tag": "letsencrypt", "domain": ["example.com"]},
			{"type": "cloudflare-origin-ca", "tag": "cf", "domain": ["example.com"]}
		],
		"route": {"rules": []}
	}`

	var config SingBoxConfig
	if err := json.Unmarshal([]byte(base), &config); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		CertificateProviders []struct {
			Type string `json:"type"`
			Tag  string `json:"tag"`
		} `json:"certificate_providers"`
	}
	if err = json.Unmarshal(rebuilt, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.CertificateProviders) != 2 {
		t.Fatalf("both providers must reach the final config, got %s", rebuilt)
	}
	if decoded.CertificateProviders[0].Tag != "letsencrypt" || decoded.CertificateProviders[1].Type != "cloudflare-origin-ca" {
		t.Errorf("providers must carry over unchanged, got %s", rebuilt)
	}
}

// A config with no providers must not grow an empty key, which sing-box would
// read as an explicit empty list.
func TestSingBoxConfigOmitsEmptyCertificateProviders(t *testing.T) {
	var config SingBoxConfig
	if err := json.Unmarshal([]byte(`{"log": {"level": "info"}}`), &config); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rebuilt), "certificate_providers") {
		t.Errorf("expected no certificate_providers key, got %s", rebuilt)
	}
}
