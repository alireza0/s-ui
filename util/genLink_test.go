package util

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestShadowsocksLinkPlugin(t *testing.T) {
	userConfig := map[string]map[string]interface{}{
		"shadowsocks": {"password": "testpass"},
	}
	inbound := map[string]interface{}{
		"method": "aes-256-gcm",
	}
	addrs := []map[string]interface{}{
		{"server": "1.2.3.4", "server_port": float64(8388), "remark": "test"},
	}

	// Without plugin (no out_json at all)
	links := shadowsocksLink(userConfig, inbound, addrs)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if strings.Contains(links[0], "?plugin=") {
		t.Errorf("link should not contain plugin param: %s", links[0])
	}
	if !strings.HasSuffix(links[0], "#test") {
		t.Errorf("link should end with #test: %s", links[0])
	}

	// With plugin + opts in out_json (as the real panel stores them)
	outJson := map[string]interface{}{
		"plugin":      "v2ray-plugin",
		"plugin_opts": "server;tls;host=example.com",
	}
	outJsonBytes, _ := json.Marshal(outJson)
	inbound["out_json"] = json.RawMessage(outJsonBytes)

	links = shadowsocksLink(userConfig, inbound, addrs)
	if !strings.HasSuffix(links[0], "#test") {
		t.Errorf("link should end with #test: %s", links[0])
	}
	// Verify URL round-trip: semicolons must survive url.ParseQuery
	u, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("generated link is not a valid URL: %s", err)
	}
	q, _ := url.ParseQuery(u.RawQuery)
	pluginVal := q.Get("plugin")
	if pluginVal != "v2ray-plugin;server;tls;host=example.com" {
		t.Errorf("plugin param should survive URL round-trip, got: %q from link: %s", pluginVal, links[0])
	}

	// Full round-trip via GetOutbound (the actual import path)
	outbound, tag, err := GetOutbound(links[0], 0)
	if err != nil {
		t.Fatalf("GetOutbound failed: %s", err)
	}
	if tag != "test" {
		t.Errorf("tag should be 'test', got: %q", tag)
	}
	ob := *outbound
	if ob["plugin"] != "v2ray-plugin" {
		t.Errorf("round-trip plugin name should be 'v2ray-plugin', got: %q", ob["plugin"])
	}
	if ob["plugin_opts"] != "server;tls;host=example.com" {
		t.Errorf("round-trip plugin_opts should be 'server;tls;host=example.com', got: %q", ob["plugin_opts"])
	}

	// Plugin without opts
	outJson = map[string]interface{}{"plugin": "v2ray-plugin"}
	outJsonBytes, _ = json.Marshal(outJson)
	inbound["out_json"] = json.RawMessage(outJsonBytes)

	links = shadowsocksLink(userConfig, inbound, addrs)
	outbound, _, err = GetOutbound(links[0], 0)
	if err != nil {
		t.Fatalf("GetOutbound failed for plugin-only: %s", err)
	}
	ob = *outbound
	if ob["plugin"] != "v2ray-plugin" {
		t.Errorf("plugin-only round-trip name should be 'v2ray-plugin', got: %q", ob["plugin"])
	}
	if opts, ok := ob["plugin_opts"].(string); ok && opts != "" {
		t.Errorf("plugin-only round-trip should have empty opts, got: %q", opts)
	}

	// Empty out_json (no plugin)
	outJson = map[string]interface{}{}
	outJsonBytes, _ = json.Marshal(outJson)
	inbound["out_json"] = json.RawMessage(outJsonBytes)

	links = shadowsocksLink(userConfig, inbound, addrs)
	if strings.Contains(links[0], "?plugin=") {
		t.Errorf("empty out_json should not produce plugin param: %s", links[0])
	}
}
