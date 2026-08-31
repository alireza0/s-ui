package database

import (
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func ruleSets(t *testing.T, root map[string]any) []map[string]any {
	t.Helper()
	raw, ok := section(t, root, "route")["rule_set"].([]any)
	if !ok {
		t.Fatalf("missing rule_set in %v", root)
	}
	converted := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		ruleSet, isObject := entry.(map[string]any)
		if !isObject {
			t.Fatalf("expected rule-set objects, got %v", entry)
		}
		converted = append(converted, ruleSet)
	}
	return converted
}

func TestRepairRuleSetHTTPClients(t *testing.T) {
	openTestDB(t)
	broken := `{
		"route": {
			"rule_set": [
				{"type": "remote", "tag": "a", "url": "https://e.com/a.srs",
				 "http_client": {"detour": "direct", "disable_empty_direct_check": true}},
				{"type": "remote", "tag": "b", "url": "https://e.com/b.srs",
				 "http_client": {"detour": "proxy", "disable_empty_direct_check": true}},
				{"type": "remote", "tag": "c", "url": "https://e.com/c.srs",
				 "http_client": {"detour": "proxy"}},
				{"type": "remote", "tag": "d", "url": "https://e.com/d.srs"}
			]
		}
	}`
	if err := db.Create(&model.Setting{Key: "config", Value: broken}).Error; err != nil {
		t.Fatal(err)
	}

	if err := repairRuleSetHTTPClients(); err != nil {
		t.Fatal(err)
	}
	sets := ruleSets(t, readConfig(t))
	if len(sets) != 4 {
		t.Fatalf("expected 4 rule sets, got %v", sets)
	}

	// Nothing worth keeping was left, so the whole block goes.
	if _, ok := sets[0]["http_client"]; ok {
		t.Errorf("a direct detour with no other option must drop http_client, got %v", sets[0])
	}
	// A real detour survives; only the unparseable flag is stripped.
	fixed, ok := sets[1]["http_client"].(map[string]any)
	if !ok {
		t.Fatalf("expected http_client to survive, got %v", sets[1])
	}
	if fixed["detour"] != "proxy" {
		t.Errorf("the detour must be kept, got %v", fixed)
	}
	if _, ok = fixed["disable_empty_direct_check"]; ok {
		t.Errorf("the unparseable flag must be gone, got %v", fixed)
	}
	// Already-valid entries are untouched.
	untouched, _ := sets[2]["http_client"].(map[string]any)
	if untouched["detour"] != "proxy" || len(untouched) != 1 {
		t.Errorf("a valid http_client must be left alone, got %v", untouched)
	}
	if _, ok = sets[3]["http_client"]; ok {
		t.Errorf("a rule-set without http_client must not grow one, got %v", sets[3])
	}
}

func TestRepairRuleSetHTTPClientsRunsOnce(t *testing.T) {
	openTestDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"route": {"rule_set": []}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairRuleSetHTTPClients(); err != nil {
		t.Fatal(err)
	}

	edited := `{"route": {"rule_set": [
		{"type": "remote", "tag": "a", "url": "https://e.com/a.srs",
		 "http_client": {"detour": "direct", "disable_empty_direct_check": true}}
	]}}`
	if err := db.Model(model.Setting{}).Where("key = ?", "config").
		Update("value", edited).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairRuleSetHTTPClients(); err != nil {
		t.Fatal(err)
	}

	sets := ruleSets(t, readConfig(t))
	if _, ok := sets[0]["http_client"]; !ok {
		t.Error("the repair should not run a second time")
	}
}
