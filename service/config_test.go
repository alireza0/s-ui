package service

import (
	"encoding/json"
	"testing"
)

func TestSingBoxConfigPreservesSingBox114TopLevelFields(t *testing.T) {
	input := []byte(`{
		"log": {"disabled": true},
		"certificate": {"store": "system"},
		"certificate_providers": [
			{"tag": "origin", "type": "cloudflare-origin-ca", "domain": ["example.com"]}
		],
		"http_clients": [
			{"tag": "h3", "version": 3}
		],
		"inbounds": [],
		"outbounds": [],
		"services": [],
		"endpoints": []
	}`)

	var config SingBoxConfig
	if err := json.Unmarshal(input, &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if len(config.Certificate) == 0 {
		t.Fatal("certificate field was dropped")
	}
	if len(config.CertificateProviders) != 1 {
		t.Fatalf("certificate_providers were not preserved: %d", len(config.CertificateProviders))
	}
	if len(config.HTTPClients) != 1 {
		t.Fatalf("http_clients were not preserved: %d", len(config.HTTPClients))
	}

	output, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(output, &raw); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	for _, key := range []string{"certificate", "certificate_providers", "http_clients"} {
		if len(raw[key]) == 0 {
			t.Fatalf("%s missing from output config", key)
		}
	}
}
