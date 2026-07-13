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

func TestVlessFlowGate(t *testing.T) {
	userConfigWithFlow := map[string]interface{}{
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"flow": "xtls-rprx-vision",
	}
	userConfigNoFlow := map[string]interface{}{
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	}
	addrs := []map[string]interface{}{
		{
			"server":      "1.2.3.4",
			"server_port": float64(443),
			"remark":      "test",
			"tls": map[string]interface{}{
				"enabled": true,
				"reality": map[string]interface{}{"enabled": true},
			},
		},
	}

	makeInbound := func(realityEnabled bool, allowFlow interface{}) map[string]interface{} {
		reality := map[string]interface{}{"enabled": realityEnabled}
		tls := map[string]interface{}{"reality": reality}
		outJsonMap := map[string]interface{}{"tls": tls}
		if allowFlow != nil {
			outJsonMap["allow_flow"] = allowFlow
		}
		b, _ := json.Marshal(outJsonMap)
		return map[string]interface{}{"out_json": json.RawMessage(b)}
	}

	getFlow := func(link string) string {
		u, _ := url.Parse(link)
		q, _ := url.ParseQuery(u.RawQuery)
		return q.Get("flow")
	}

	// no out_json at all — flow must be absent even with user flow set
	links := vlessLink(userConfigWithFlow, map[string]interface{}{}, addrs)
	if getFlow(links[0]) != "" {
		t.Errorf("no out_json: flow should be absent, got: %s", links[0])
	}

	// allow_flow = false explicitly — flow suppressed
	if getFlow(vlessLink(userConfigWithFlow, makeInbound(true, false), addrs)[0]) != "" {
		t.Errorf("allow_flow=false: flow should be absent")
	}

	// allow_flow = true explicitly — flow present
	if getFlow(vlessLink(userConfigWithFlow, makeInbound(true, true), addrs)[0]) != "xtls-rprx-vision" {
		t.Errorf("allow_flow=true: flow should be 'xtls-rprx-vision'")
	}

	// reality enabled + no allow_flow flag — backwards compat defaults to enabled
	if getFlow(vlessLink(userConfigWithFlow, makeInbound(true, nil), addrs)[0]) != "xtls-rprx-vision" {
		t.Errorf("reality+no allow_flow: flow should default to 'xtls-rprx-vision'")
	}

	// allow_flow = true but user has no flow value — param must be absent
	if getFlow(vlessLink(userConfigNoFlow, makeInbound(true, true), addrs)[0]) != "" {
		t.Errorf("allow_flow=true, no user flow: flow param should be absent")
	}
}
