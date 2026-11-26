package nodes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestDisabledTemplateKeyCannotChangeProtocol(t *testing.T) {
	db, _, _ := fixture(t)
	request := map[string]interface{}{"name": "history", "kind": inboundTemplate, "spec": specExample()}
	body, _ := json.Marshal(request)
	if err := Save("nodetemplates", "new", body, "test"); err != nil {
		t.Fatal(err)
	}
	var template model.NodeTemplate
	db.First(&template)
	request["id"] = template.Id
	request["expected_version"] = 1
	request["spec"] = json.RawMessage(strings.Replace(string(specExample()), `"enable":true`, `"enable":false`, 1))
	body, _ = json.Marshal(request)
	if err := Save("nodetemplates", "edit", body, "test"); err != nil {
		t.Fatal(err)
	}
	request["expected_version"] = 2
	request["spec"] = json.RawMessage(strings.Replace(string(specExample()), `"protocol":"vless"`, `"protocol":"tuic"`, 1))
	body, _ = json.Marshal(request)
	if err := Save("nodetemplates", "edit", body, "test"); err == nil {
		t.Fatal("disabled identity was reused for another protocol")
	}
	request["spec"] = specExample()
	body, _ = json.Marshal(request)
	if err := Save("nodetemplates", "edit", body, "test"); err != nil {
		t.Fatal("same-protocol history restore failed", err)
	}
}
