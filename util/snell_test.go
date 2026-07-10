package util

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnellLinkToJson(t *testing.T) {
	outbound, tag, err := GetOutbound("snell://server-key@example.com:23456?version=6&mode=unshaped&userkey=user-key#snell", 0)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "snell" {
		t.Fatalf("tag = %q", tag)
	}
	if got := (*outbound)["type"]; got != "snell" {
		t.Fatalf("type = %v", got)
	}
	if got := (*outbound)["version"]; got != 6 {
		t.Fatalf("version = %v", got)
	}
	if got := (*outbound)["psk"]; got != "server-key" {
		t.Fatalf("psk = %v", got)
	}
	if got := (*outbound)["userkey"]; got != "user-key" {
		t.Fatalf("userkey = %v", got)
	}
}

func TestSnellLinkToJsonV4Obfs(t *testing.T) {
	outbound, _, err := GetOutbound("snell://mypassphrase@10.0.0.1:1234?version=4&obfs_mode=http&obfs_host=example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := (*outbound)["version"]; got != 4 {
		t.Fatalf("version = %v", got)
	}
	if got := (*outbound)["obfs_mode"]; got != "http" {
		t.Fatalf("obfs_mode = %v", got)
	}
	if got := (*outbound)["obfs_host"]; got != "example.com" {
		t.Fatalf("obfs_host = %v", got)
	}
	// no userkey when not provided
	if _, ok := (*outbound)["userkey"]; ok {
		t.Fatal("userkey should not be set")
	}
}

func TestSnellLinkRoundTrip(t *testing.T) {
	clientConfigJSON := []byte(`{"snell":{"userkey":"myuserkey"}}`)
	inboundJSON := `{"type":"snell","listen":"::","listen_port":1080,"psk":"mypassphrase","version":5,"obfs_mode":"http"}`

	var inboundMap map[string]interface{}
	if err := json.Unmarshal([]byte(inboundJSON), &inboundMap); err != nil {
		t.Fatal(err)
	}

	addrs := []map[string]interface{}{
		{"server": "127.0.0.1", "server_port": float64(1080), "remark": "test"},
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfigJSON, &userConfig); err != nil {
		t.Fatal(err)
	}

	links := snellLink(userConfig["snell"], inboundMap, addrs)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	link := links[0]
	if !strings.HasPrefix(link, "snell://") {
		t.Fatalf("link doesn't start with snell://: %s", link)
	}
	// version 5 inbound maps to version 4 outbound in snellLink
	if !strings.Contains(link, "version=4") {
		t.Fatalf("expected version=4 (v5 inbound is v4 link): %s", link)
	}
	if !strings.Contains(link, "obfs_mode=http") {
		t.Fatalf("expected obfs_mode=http: %s", link)
	}
	if !strings.Contains(link, "userkey=myuserkey") {
		t.Fatalf("expected userkey=myuserkey: %s", link)
	}
}
