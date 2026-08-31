package sub

import (
	"encoding/json"
	"testing"
)

func runAddHTTPClients(t *testing.T, template string) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	var othersJson map[string]interface{}
	if err := json.Unmarshal([]byte(template), &othersJson); err != nil {
		t.Fatal(err)
	}
	route := map[string]interface{}{"final": "proxy"}
	if ruleSet, ok := othersJson["rule_set"]; ok {
		route["rule_set"] = ruleSet
	}
	jsonConfig := map[string]interface{}{}
	(&JsonService{}).addHTTPClients(&jsonConfig, route, othersJson)
	return jsonConfig, route
}

// A remote rule-set with no client of its own would use the implicit default,
// which sing-box 1.14 reports as deprecated in the client's log.
func TestAddHTTPClientsDeclaresDefault(t *testing.T) {
	jsonConfig, route := runAddHTTPClients(t, `{
		"rule_set": [
			{"tag": "geosite-ir", "type": "remote", "format": "binary", "url": "https://e.com/a.srs"}
		]
	}`)

	clients, ok := jsonConfig["http_clients"].([]interface{})
	if !ok || len(clients) != 1 {
		t.Fatalf("expected one declared http client, got %v", jsonConfig["http_clients"])
	}
	client, _ := clients[0].(map[string]interface{})
	if client["tag"] != defaultHTTPClientTag {
		t.Errorf("unexpected client: %v", client)
	}
	// A detour would be wrong here: sing-box refuses a detour to a plain direct
	// outbound, and downloading over the default outbound is the intent.
	if _, hasDetour := client["detour"]; hasDetour {
		t.Errorf("the default download client must carry no detour, got %v", client)
	}
	if route["default_http_client"] != defaultHTTPClientTag {
		t.Errorf("the route must point at it, got %v", route["default_http_client"])
	}
}

// Rule-sets that name their own client need no default, and a config with no
// remote rule-set at all must not grow one.
func TestAddHTTPClientsLeavesConfigsAlone(t *testing.T) {
	for name, template := range map[string]string{
		"every rule-set has a client": `{
			"rule_set": [
				{"tag": "a", "type": "remote", "format": "binary", "url": "https://e.com/a.srs",
				 "http_client": {"detour": "proxy"}}
			]
		}`,
		"local rule-set only": `{
			"rule_set": [{"tag": "a", "type": "local", "format": "binary", "path": "/a.srs"}]
		}`,
		"no rule-set": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			jsonConfig, route := runAddHTTPClients(t, template)
			if _, ok := jsonConfig["http_clients"]; ok {
				t.Errorf("no client should have been declared, got %v", jsonConfig["http_clients"])
			}
			if _, ok := route["default_http_client"]; ok {
				t.Errorf("no default should have been set, got %v", route["default_http_client"])
			}
		})
	}
}

// A template that declares its own clients decides for itself.
func TestAddHTTPClientsKeepsTemplateClients(t *testing.T) {
	jsonConfig, route := runAddHTTPClients(t, `{
		"http_clients": [{"tag": "over-proxy", "detour": "proxy"}],
		"default_http_client": "over-proxy",
		"rule_set": [
			{"tag": "a", "type": "remote", "format": "binary", "url": "https://e.com/a.srs"}
		]
	}`)

	clients, ok := jsonConfig["http_clients"].([]interface{})
	if !ok || len(clients) != 1 {
		t.Fatalf("the template's clients must be carried over, got %v", jsonConfig["http_clients"])
	}
	client, _ := clients[0].(map[string]interface{})
	if client["tag"] != "over-proxy" || client["detour"] != "proxy" {
		t.Errorf("the template's client must be unchanged, got %v", client)
	}
	if route["default_http_client"] != "over-proxy" {
		t.Errorf("the template's default must win, got %v", route["default_http_client"])
	}
}

func runAddOthersRoute(t *testing.T, template string) map[string]interface{} {
	t.Helper()
	var othersJson map[string]interface{}
	if err := json.Unmarshal([]byte(template), &othersJson); err != nil {
		t.Fatal(err)
	}
	route := map[string]interface{}{"final": "proxy"}
	if defaultDomainResolver, ok := othersJson["default_domain_resolver"].(string); ok && defaultDomainResolver != "" {
		route["default_domain_resolver"] = defaultDomainResolver
	} else if fallback := fallbackDomainResolver(othersJson); fallback != "" {
		route["default_domain_resolver"] = fallback
	}
	return route
}

// With two or more DNS servers and nothing naming the resolver for dial
// fields, sing-box guesses and reports the guess as deprecated.
func TestFallbackDomainResolver(t *testing.T) {
	for name, expect := range map[string]struct {
		template string
		want     string
	}{
		"final server wins": {`{"dns": {"servers": [
			{"tag": "proxy-dns", "type": "tcp", "server": "8.8.8.8"},
			{"tag": "local-dns", "type": "local"}
		], "final": "local-dns"}}`, "local-dns"},
		"first server without a final": {`{"dns": {"servers": [
			{"tag": "a", "type": "local"}, {"tag": "b", "type": "local"}
		]}}`, "a"},
		"single server needs no choice": {`{"dns": {"servers": [
			{"tag": "a", "type": "local"}
		], "final": "a"}}`, ""},
		"no dns section": {`{}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			var othersJson map[string]interface{}
			if err := json.Unmarshal([]byte(expect.template), &othersJson); err != nil {
				t.Fatal(err)
			}
			if got := fallbackDomainResolver(othersJson); got != expect.want {
				t.Errorf("want %q, got %q", expect.want, got)
			}
		})
	}
}

// An explicit choice in the template must not be second-guessed.
func TestDefaultDomainResolverFromTemplate(t *testing.T) {
	route := runAddOthersRoute(t, `{
		"default_domain_resolver": "direct-dns",
		"dns": {"servers": [
			{"tag": "proxy-dns", "type": "tcp", "server": "8.8.8.8"},
			{"tag": "direct-dns", "type": "local"}
		], "final": "proxy-dns"}
	}`)
	if route["default_domain_resolver"] != "direct-dns" {
		t.Errorf("the template's choice must win, got %v", route["default_domain_resolver"])
	}
}
