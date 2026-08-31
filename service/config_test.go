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

func routeOf(t *testing.T, config SingBoxConfig) map[string]any {
	t.Helper()
	var route map[string]any
	if len(config.Route) == 0 {
		return route
	}
	if err := json.Unmarshal(config.Route, &route); err != nil {
		t.Fatal(err)
	}
	return route
}

func decodeConfig(t *testing.T, raw string) SingBoxConfig {
	t.Helper()
	var config SingBoxConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultHTTPClient(&config); err != nil {
		t.Fatal(err)
	}
	return config
}

// A remote rule-set with no client of its own downloads over the implicit
// default, which sing-box 1.14 deprecates.
func TestEnsureDefaultHTTPClient(t *testing.T) {
	config := decodeConfig(t, `{
		"route": {"rule_set": [
			{"type": "remote", "tag": "a", "url": "https://e.com/a.srs"}
		]}
	}`)

	if len(config.HTTPClients) != 1 {
		t.Fatalf("expected one declared http client, got %v", config.HTTPClients)
	}
	var client map[string]any
	if err := json.Unmarshal(config.HTTPClients[0], &client); err != nil {
		t.Fatal(err)
	}
	// A detour to a plain direct outbound is rejected by sing-box, and the
	// default outbound is what the implicit client used anyway.
	if client["tag"] != defaultHTTPClientTag || len(client) != 1 {
		t.Errorf("unexpected client: %v", client)
	}
	if routeOf(t, config)["default_http_client"] != defaultHTTPClientTag {
		t.Errorf("the route must point at it, got %v", routeOf(t, config))
	}
}

func TestEnsureDefaultHTTPClientLeavesConfigsAlone(t *testing.T) {
	for name, raw := range map[string]string{
		"rule-set names its own client": `{"route": {"rule_set": [
			{"type": "remote", "tag": "a", "url": "https://e.com/a.srs", "http_client": {"detour": "proxy"}}
		]}}`,
		"local rule-set only": `{"route": {"rule_set": [
			{"type": "local", "tag": "a", "path": "/a.srs"}
		]}}`,
		"no rule-set": `{"route": {"rules": []}}`,
		"no route":    `{"log": {"level": "info"}}`,
		"default already set": `{"route": {"default_http_client": "mine", "rule_set": [
			{"type": "remote", "tag": "a", "url": "https://e.com/a.srs"}
		]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			config := decodeConfig(t, raw)
			for _, client := range config.HTTPClients {
				var decoded map[string]any
				if err := json.Unmarshal(client, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded["tag"] == defaultHTTPClientTag {
					t.Errorf("no client should have been declared, got %v", decoded)
				}
			}
			if got := routeOf(t, config)["default_http_client"]; got != nil && got != "mine" {
				t.Errorf("unexpected default_http_client: %v", got)
			}
		})
	}
}

// Declaring clients without naming a default still leaves bare rule-sets
// falling back, so one is added beside them rather than skipped.
func TestEnsureDefaultHTTPClientAppends(t *testing.T) {
	config := decodeConfig(t, `{
		"http_clients": [{"tag": "over-proxy", "detour": "proxy"}],
		"route": {"rule_set": [
			{"type": "remote", "tag": "a", "url": "https://e.com/a.srs"},
			{"type": "remote", "tag": "b", "url": "https://e.com/b.srs", "http_client": "over-proxy"}
		]}
	}`)

	if len(config.HTTPClients) != 2 {
		t.Fatalf("the operator's client must be kept alongside the added one, got %v", config.HTTPClients)
	}
	if routeOf(t, config)["default_http_client"] != defaultHTTPClientTag {
		t.Errorf("expected the added client as default, got %v", routeOf(t, config)["default_http_client"])
	}
}

// The added client must not take a tag the operator already used.
func TestEnsureDefaultHTTPClientAvoidsTagCollision(t *testing.T) {
	config := decodeConfig(t, `{
		"http_clients": [{"tag": "default", "detour": "proxy"}],
		"route": {"rule_set": [{"type": "remote", "tag": "a", "url": "https://e.com/a.srs"}]}
	}`)

	got := routeOf(t, config)["default_http_client"]
	if got == defaultHTTPClientTag {
		t.Fatalf("the added client must not reuse the operator's tag, got %v", got)
	}
	var added map[string]any
	if err := json.Unmarshal(config.HTTPClients[len(config.HTTPClients)-1], &added); err != nil {
		t.Fatal(err)
	}
	if added["tag"] != got {
		t.Errorf("the route must point at the added client, got %v and %v", got, added)
	}
}
