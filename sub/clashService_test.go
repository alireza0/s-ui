package sub

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHasGroupNamed(t *testing.T) {
	tests := []struct {
		name     string
		pg       interface{}
		lookFor  string
		expected bool
	}{
		{"nil input", nil, "Proxy", false},
		{"non-slice input", "not a slice", "Proxy", false},
		{"empty slice", []interface{}{}, "Proxy", false},
		{"no match", []interface{}{
			map[string]interface{}{"name": "Auto", "type": "url-test"},
			map[string]interface{}{"name": "Direct", "type": "select"},
		}, "Proxy", false},
		{"exact match", []interface{}{
			map[string]interface{}{"name": "Auto", "type": "url-test"},
			map[string]interface{}{"name": "Proxy", "type": "select"},
		}, "Proxy", true},
		{"match first item", []interface{}{
			map[string]interface{}{"name": "Proxy", "type": "select"},
			map[string]interface{}{"name": "Auto", "type": "url-test"},
		}, "Proxy", true},
		{"group without name key", []interface{}{
			map[string]interface{}{"type": "select"},
		}, "Proxy", false},
		{"name not a string", []interface{}{
			map[string]interface{}{"name": 123},
		}, "Proxy", false},
		{"single match in slice with non-map items", []interface{}{
			"not a map",
			map[string]interface{}{"name": "Proxy"},
		}, "Proxy", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasGroupNamed(tt.pg, tt.lookFor)
			if result != tt.expected {
				t.Errorf("hasGroupNamed(%v, %q) = %v, want %v", tt.pg, tt.lookFor, result, tt.expected)
			}
		})
	}
}

// minimalOutbound — bare min fields ConvertToClashMeta needs to process a proxy node.
func minimalOutbound(tag, srv string, port int) map[string]interface{} {
	return map[string]interface{}{
		"tag":         tag,
		"type":        "vless",
		"server":      srv,
		"server_port": port,
		"uuid":        "00000000-0000-0000-0000-000000000000",
	}
}

// parseYAML is a test helper.
func parseYAML(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	return m
}

func TestConvertToClashMeta_ProxyGroupInjection(t *testing.T) {
	svc := &ClashService{}
	outbounds := &[]map[string]interface{}{
		minimalOutbound("node-hk-01", "1.2.3.4", 443),
		minimalOutbound("node-us-01", "5.6.7.8", 8443),
	}

	// ── Scenario 1: no proxy-groups in template → should inject Proxy + Auto ──
	noGroupsCfg := `mixed-port: 7890
dns:
  enable: false
rules:
  - MATCH,Proxy
`
	result, err := svc.ConvertToClashMeta(outbounds, noGroupsCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseYAML(t, result)
	pg, ok := m["proxy-groups"].([]interface{})
	if !ok || len(pg) < 2 {
		t.Fatalf("Scenario 1: expected at least 2 proxy-groups, got %v", pg)
	}
	if !hasGroupNamed(pg, "Proxy") {
		t.Errorf("Scenario 1: expected 'Proxy' group to be injected")
	}
	if !hasGroupNamed(pg, "Auto") {
		t.Errorf("Scenario 1: expected 'Auto' group to be injected")
	}
	t.Log("Scenario 1 PASS: no proxy-groups → Proxy + Auto injected")

	// ── Scenario 2: custom groups without "Proxy" → should inject + prepend ──
	customNoProxyCfg := `mixed-port: 7890
dns:
  enable: false
proxy-groups:
  - name: default
    type: select
    proxies:
      - Proxy
      - DIRECT
  - name: 🇨🇳 国内直连
    type: select
    proxies:
      - DIRECT
      - Proxy
rules:
  - MATCH,default
`
	result, err = svc.ConvertToClashMeta(outbounds, customNoProxyCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = parseYAML(t, result)
	pg, ok = m["proxy-groups"].([]interface{})
	if !ok || len(pg) != 4 {
		t.Fatalf("Scenario 2: expected exactly 4 proxy-groups (Proxy+Auto prepended + 2 custom), got %v", pg)
	}
	// Proxy + Auto must be prepended before custom groups
	first := pg[0].(map[string]interface{})
	second := pg[1].(map[string]interface{})
	third := pg[2].(map[string]interface{})
	if first["name"] != "Proxy" {
		t.Errorf("Scenario 2: first group should be 'Proxy', got %v", first["name"])
	}
	if second["name"] != "Auto" {
		t.Errorf("Scenario 2: second group should be 'Auto', got %v", second["name"])
	}
	if third["name"] != "default" {
		t.Errorf("Scenario 2: third group should be custom 'default', got %v", third["name"])
	}
	// Verify Proxy group contains Auto + all node tags
	proxyProxies, _ := first["proxies"].([]interface{})
	if len(proxyProxies) < 3 {
		t.Errorf("Scenario 2: Proxy group should contain Auto + 2 nodes, got %v", proxyProxies)
	}
	if proxyProxies[0] != "Auto" {
		t.Errorf("Scenario 2: Proxy group first proxy should be 'Auto', got %v", proxyProxies[0])
	}
	t.Log("Scenario 2 PASS: custom groups without Proxy → Proxy+Auto prepended, custom preserved")

	// ── Scenario 3: custom groups WITH "Proxy" → should NOT inject duplicates ──
	customWithProxyCfg := `mixed-port: 7890
dns:
  enable: false
proxy-groups:
  - name: Proxy
    type: fallback
    proxies:
      - DIRECT
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: Streaming
    type: select
    proxies:
      - Proxy
      - DIRECT
rules:
  - MATCH,Proxy
`
	result, err = svc.ConvertToClashMeta(outbounds, customWithProxyCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = parseYAML(t, result)
	pg, ok = m["proxy-groups"].([]interface{})
	if !ok || len(pg) != 2 {
		t.Fatalf("Scenario 3: expected exactly 2 proxy-groups (user-defined only), got %d: %v", len(pg), pg)
	}
	if pg[0].(map[string]interface{})["name"] != "Proxy" {
		t.Errorf("Scenario 3: first group should be user's 'Proxy', got %v", pg[0].(map[string]interface{})["name"])
	}
	if pg[1].(map[string]interface{})["name"] != "Streaming" {
		t.Errorf("Scenario 3: second group should be user's 'Streaming', got %v", pg[1].(map[string]interface{})["name"])
	}
	// The user's "Proxy" group should NOT be replaced by our default
	userProxy, _ := pg[0].(map[string]interface{})
	if userProxy["type"] != "fallback" {
		t.Errorf("Scenario 3: user's Proxy type should remain 'fallback', got %v", userProxy["type"])
	}
	t.Log("Scenario 3 PASS: user defines their own Proxy → no injection, user's group preserved")

	// ── Scenario 4: empty proxy-groups → should inject defaults ──
	emptyGroupsCfg := `mixed-port: 7890
dns:
  enable: false
proxy-groups: []
rules:
  - MATCH,Proxy
`
	result, err = svc.ConvertToClashMeta(outbounds, emptyGroupsCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = parseYAML(t, result)
	pg, ok = m["proxy-groups"].([]interface{})
	if !ok || len(pg) < 2 {
		t.Fatalf("Scenario 4: expected at least 2 proxy-groups, got %v", pg)
	}
	if !hasGroupNamed(pg, "Proxy") || !hasGroupNamed(pg, "Auto") {
		t.Errorf("Scenario 4: expected Proxy + Auto injected for empty proxy-groups")
	}
	t.Log("Scenario 4 PASS: empty proxy-groups → Proxy + Auto injected")
	// ── Scenario 5: custom groups without Proxy reference → no injection ──
	customNoRefCfg := `mixed-port: 7890
dns:
  enable: false
proxy-groups:
  - name: manual
    type: select
    proxies:
      - node-hk-01
      - node-us-01
      - DIRECT
  - name: auto
    type: url-test
    proxies:
      - node-hk-01
      - node-us-01
rules:
  - MATCH,manual

`
	result, err = svc.ConvertToClashMeta(outbounds, customNoRefCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = parseYAML(t, result)
	pg, ok = m["proxy-groups"].([]interface{})
	if !ok {
		t.Fatalf("Scenario 5: expected proxy-groups to exist")
	}
	if hasGroupNamed(pg, "Proxy") || hasGroupNamed(pg, "Auto") {
		t.Errorf("Scenario 5: Proxy/Auto should NOT be injected when no group references Proxy")
	}
	if len(pg) != 2 {
		t.Errorf("Scenario 5: expected exactly 2 user groups, got %d", len(pg))
	}
	t.Log("Scenario 5 PASS: custom groups without Proxy reference → no injection")
	// ── Scenario 6: user already has Auto group, references Proxy → inject only Proxy ──
	userHasAutoCfg := `mixed-port: 7890
dns:
  enable: false
proxy-groups:
  - name: Auto
    type: url-test
    proxies:
      - node-hk-01
      - node-us-01
    url: http://www.gstatic.com/generate_204
    interval: 600
  - name: default
    type: select
    proxies:
      - Proxy
      - DIRECT
rules:
  - MATCH,default
`
	result, err = svc.ConvertToClashMeta(outbounds, userHasAutoCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = parseYAML(t, result)
	pg, ok = m["proxy-groups"].([]interface{})
	if !ok || len(pg) < 3 {
		t.Fatalf("Scenario 6: expected at least 3 proxy-groups (Proxy + 2 user), got %d", len(pg))
	}
	// Should have Proxy injected
	if !hasGroupNamed(pg, "Proxy") {
		t.Errorf("Scenario 6: expected Proxy to be injected")
	}
	// Should NOT have duplicate Auto — only the user original
	countAuto := 0
	for _, item := range pg {
		if g, ok := item.(map[string]interface{}); ok {
			if g["name"] == "Auto" {
				countAuto++
			}
		}
	}
	if countAuto != 1 {
		t.Errorf("Scenario 6: expected exactly 1 Auto group, got %d", countAuto)
	}
	// Verify injected Proxy does NOT reference Auto (since Auto is user-managed)
	first = pg[0].(map[string]interface{})
	proxyProxies, _ = first["proxies"].([]interface{})
	for _, p := range proxyProxies {
		if pname, ok := p.(string); ok && pname == "Auto" {
			t.Errorf("Scenario 6: injected Proxy should not reference Auto (user has their own)")
		}
	}
	t.Log("Scenario 6 PASS: user has Auto + references Proxy → injects only Proxy, not duplicate Auto")

}
