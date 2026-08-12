package service

import (
	"encoding/json"
	"testing"
)

func TestSanitizeRemoteInboundJSON(t *testing.T) {
	raw := []byte(`{"id":10,"node_id":2,"type":"vless","tag":"v1","listen":"0.0.0.0"}`)
	sanitized := SanitizeRemoteInboundJSON(raw)

	var parsed map[string]interface{}
	if err := json.Unmarshal(sanitized, &parsed); err != nil {
		t.Fatal("unmarshal:", err)
	}

	if _, exists := parsed["id"]; exists {
		t.Error("id field should be removed")
	}
	if _, exists := parsed["node_id"]; exists {
		t.Error("node_id field should be removed")
	}
	if parsed["tag"] != "v1" {
		t.Errorf("tag = %v, want v1", parsed["tag"])
	}
}
