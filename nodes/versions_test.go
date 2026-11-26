package nodes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestTemplateVersionsAreAppendOnlyAndCompareExpectedVersion(t *testing.T) {
	db, _, _ := fixture(t)
	request := map[string]interface{}{"name": "versioned", "kind": inboundTemplate, "spec": specExample()}
	body, _ := json.Marshal(request)
	if err := Save("nodetemplates", "new", body, "test"); err != nil {
		t.Fatal(err)
	}
	var template model.NodeTemplate
	db.First(&template)
	if template.Version != 1 || template.SchemaVersion != 1 {
		t.Fatal("missing initial immutable version")
	}
	var first model.NodeTemplateVersion
	db.First(&first, "template_id = ? AND version = ?", template.Id, 1)
	original := string(first.Payload)
	request["id"] = template.Id
	request["expected_version"] = 1
	request["spec"] = json.RawMessage(strings.Replace(string(specExample()), `"name":"VLESS"`, `"name":"Renamed"`, 1))
	body, _ = json.Marshal(request)
	if err := Save("nodetemplates", "edit", body, "test"); err != nil {
		t.Fatal(err)
	}
	db.First(&template, template.Id)
	if template.Version != 2 {
		t.Fatal("changed template did not create next version")
	}
	db.First(&first, "template_id = ? AND version = ?", template.Id, 1)
	if string(first.Payload) != original {
		t.Fatal("old version was modified")
	}
	if err := Save("nodetemplates", "edit", body, "test"); err == nil || err.Error() != "template_version_conflict" {
		t.Fatal("stale writer overwrote template", err)
	}
	versions, err := TemplateHistory(template.Id)
	if err != nil || len(versions) != 2 {
		t.Fatal("incorrect version history", err)
	}
	public, _ := json.Marshal(versions)
	if strings.Contains(string(public), "options") || strings.Contains(string(public), "payload") {
		t.Fatal("history metadata leaked legacy/private payload")
	}
	restore, _ := json.Marshal(map[string]any{"id": template.Id, "version": 1, "expected_version": 2})
	requireNodeTestSuccess(t, Save("nodetemplates", "restoreversion", restore, "test"))
	view, err := ReadTemplateVersion(template.Id, 0)
	requireNodeTestSuccess(t, err)
	if view.Version != 3 || string(view.Spec) != original {
		t.Fatal("restore did not append the selected version")
	}
	if err := Save("nodetemplates", "edit", json.RawMessage(`{"id":1,"name":"old editor","config":{},"blueprints":[]}`), "test"); err == nil {
		t.Fatal("legacy editor overwrote versioned template")
	}
}
