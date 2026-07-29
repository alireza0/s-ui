package sub

import (
	"encoding/json"
	"testing"
)

func TestLinkServiceIncludesNodeLinks(t *testing.T) {
	links := json.RawMessage(`[
		{"type":"local","remark":"local-vless","uri":"vless://abc@master:443#local"},
		{"type":"node","remark":"de-vless","uri":"vless://abc@de.example.com:443#DE"},
		{"type":"external","uri":"vless://abc@ext.example.com:443#EXT"}
	]`)

	ls := &LinkService{}
	result := ls.GetLinks(&links, "all", "")

	// Should include local, node, and external links
	if len(result) != 3 {
		t.Fatalf("expected 3 links, got %d: %v", len(result), result)
	}

	// Verify all three are present
	found := map[string]bool{}
	for _, l := range result {
		if l == "vless://abc@master:443#local" {
			found["local"] = true
		}
		if l == "vless://abc@de.example.com:443#DE" {
			found["node"] = true
		}
		if l == "vless://abc@ext.example.com:443#EXT" {
			found["external"] = true
		}
	}
	if !found["local"] || !found["node"] || !found["external"] {
		t.Errorf("missing link types: %v", found)
	}
}

func TestLinkServiceNodeLinksExcludedFromNonAll(t *testing.T) {
	links := json.RawMessage(`[
		{"type":"local","remark":"local-vless","uri":"vless://abc@master:443#local"},
		{"type":"node","remark":"de-vless","uri":"vless://abc@de.example.com:443#DE"},
		{"type":"external","uri":"vless://abc@ext.example.com:443#EXT"}
	]`)

	ls := &LinkService{}
	result := ls.GetLinks(&links, "external", "")

	// With types != "all", only external links should be included
	if len(result) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(result), result)
	}
	if result[0] != "vless://abc@ext.example.com:443#EXT" {
		t.Errorf("expected external link, got %s", result[0])
	}
}

func TestExternalOutboundsIgnoresNodeLinks(t *testing.T) {
	links := json.RawMessage(`[
		{"type":"node","remark":"de-vless","uri":"vless://abc@de.example.com:443#DE"},
		{"type":"local","remark":"local-vless","uri":"vless://abc@master:443#local"}
	]`)

	ls := &LinkService{}
	outbounds, tags := ls.GetExternalOutbounds(&links)

	// Node and local links should not generate external outbounds
	if len(outbounds) != 0 {
		t.Errorf("expected 0 external outbounds, got %d", len(outbounds))
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}
