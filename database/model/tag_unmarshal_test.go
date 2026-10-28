package model

import "testing"

// Regression: Outbound and Endpoint used a bare raw["tag"].(string)
// assertion in UnmarshalJSON while their sibling types used the comma-ok
// form, so a syntactically valid saved object with a missing or
// non-string tag (hand-written API call, import script, third-party
// client) panicked the save handler instead of saving with an empty tag.
func TestOutboundUnmarshalMissingOrBadTag(t *testing.T) {
	for _, data := range []string{
		`{"type":"direct"}`,
		`{"type":"direct","tag":123}`,
		`{"type":"direct","tag":null}`,
		`{"type":"direct","tag":{"x":1}}`,
	} {
		var outbound Outbound
		if err := outbound.UnmarshalJSON([]byte(data)); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if outbound.Tag != "" {
			t.Fatalf("unmarshal %s: tag = %q, want empty", data, outbound.Tag)
		}
	}

	var outbound Outbound
	if err := outbound.UnmarshalJSON([]byte(`{"type":"direct","tag":"out"}`)); err != nil {
		t.Fatalf("unmarshal valid: %v", err)
	}
	if outbound.Tag != "out" {
		t.Fatalf("tag = %q, want %q", outbound.Tag, "out")
	}
}

func TestEndpointUnmarshalMissingOrBadTag(t *testing.T) {
	for _, data := range []string{
		`{"type":"wireguard"}`,
		`{"type":"wireguard","tag":123}`,
		`{"type":"wireguard","tag":null}`,
		`{"type":"wireguard","tag":{"x":1}}`,
	} {
		var endpoint Endpoint
		if err := endpoint.UnmarshalJSON([]byte(data)); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if endpoint.Tag != "" {
			t.Fatalf("unmarshal %s: tag = %q, want empty", data, endpoint.Tag)
		}
	}

	var endpoint Endpoint
	if err := endpoint.UnmarshalJSON([]byte(`{"type":"wireguard","tag":"ep"}`)); err != nil {
		t.Fatalf("unmarshal valid: %v", err)
	}
	if endpoint.Tag != "ep" {
		t.Fatalf("tag = %q, want %q", endpoint.Tag, "ep")
	}
}
