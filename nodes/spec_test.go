package nodes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func specExample() json.RawMessage {
	return json.RawMessage(`{"schema_version":1,"config":{},"entries":[{"connection_key":"vless-primary","name":"VLESS","protocol":"vless","enable":true,"options":{"listen":"::","tls":{"enabled":true}},"sources":{"/listen_port":{"kind":"node_generated","purpose":"listen_port"},"/tls/server_name":{"kind":"node_parameter","name":"primary_domain"}}}]}`)
}

func specContext(domain string) ResolveContext {
	return ResolveContext{Parameters: map[string]json.RawMessage{"primary_domain": json.RawMessage(`"` + domain + `"`)}, Generated: map[string]json.RawMessage{"vless-primary/listen_port": json.RawMessage(`443`)}}
}

func TestSpecResolvesNodeSourcesWithoutCopyingOrMutation(t *testing.T) {
	spec, err := NormalizeSpec(specExample(), nil)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(spec)
	first, err := ResolveSpec(spec, specContext("first.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ResolveSpec(spec, specContext("second.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Entries[0].Key != second.Entries[0].Key || !strings.Contains(string(first.Entries[0].Options), "first.example.com") || !strings.Contains(string(second.Entries[0].Options), "second.example.com") {
		t.Fatal("node-specific values or connection identity were copied")
	}
	after, _ := json.Marshal(spec)
	if string(before) != string(after) {
		t.Fatal("resolution mutated the reusable template")
	}
	repeated, _ := ResolveSpec(spec, specContext("first.example.com"))
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(repeated)
	if string(a) != string(b) {
		t.Fatal("resolution is not deterministic")
	}
}

func TestSpecExplicitOverrideSurvivesMatchingOldDefault(t *testing.T) {
	spec, err := NormalizeSpec(specExample(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := specContext("updated.example.com")
	ctx.Overrides = []SpecOverride{{ConnectionKey: "vless-primary", Path: "/tls/server_name", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"original.example.com"`)}}}
	resolved, err := ResolveSpec(spec, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resolved.Entries[0].Options), "original.example.com") {
		t.Fatal("explicit override was inferred away")
	}
	ctx.Overrides = nil
	resolved, err = ResolveSpec(spec, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resolved.Entries[0].Options), "updated.example.com") {
		t.Fatal("reset did not restore inherited node source")
	}
}

func TestSpecRejectsSecretsInvalidSourcesAndProtocolMutation(t *testing.T) {
	for _, raw := range []string{
		`{"schema_version":2,"config":{},"entries":[]}`,
		`{"schema_version":1,"config":{"users":[]},"entries":[]}`,
		`{"schema_version":1,"config":{},"unknown":true,"entries":[]}`,
		`{"schema_version":1,"config":{},"entries":[{"connection_key":"a","name":"a","protocol":"vless","options":{"private_key":"synthetic-secret"}}]}`,
		`{"schema_version":1,"config":{},"entries":[{"connection_key":"a","name":"a","protocol":"vless","options":{},"sources":{"/tls/reality/private_key":{"kind":"literal","value":"synthetic-secret"}}}]}`,
		`{"schema_version":1,"config":{},"entries":[{"connection_key":"a","name":"a","protocol":"vless","options":{},"sources":{"/alpn/0":{"kind":"literal","value":"h3"}}}]}`,
		`{"schema_version":1,"config":{},"entries":[{"connection_key":"a","name":"a","protocol":"vless","options":{},"sources":{"/users":{"kind":"profile","name":"arbitrary"}}}]}`,
	} {
		if _, err := NormalizeSpec(json.RawMessage(raw), nil); err == nil {
			t.Fatal("invalid template accepted")
		}
	}
	prior, _ := NormalizeSpec(specExample(), nil)
	mutated := strings.Replace(string(specExample()), `"protocol":"vless"`, `"protocol":"tuic"`, 1)
	if _, err := NormalizeSpec(json.RawMessage(mutated), &prior); err == nil {
		t.Fatal("protocol of an existing key changed")
	}
	if _, err := ResolveSpec(prior, ResolveContext{}); err == nil {
		t.Fatal("missing node material was silently generated")
	}
}

func TestSpecArraysAreAtomicAndTopologyCannotBeOverridden(t *testing.T) {
	spec, _ := NormalizeSpec(specExample(), nil)
	ctx := specContext("node.example.com")
	ctx.Overrides = []SpecOverride{{ConnectionKey: "vless-primary", Path: "/tls/alpn", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`["h2","http/1.1"]`)}}}
	result, err := ResolveSpec(spec, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Entries[0].Enable || !strings.Contains(string(result.Entries[0].Options), `["h2","http/1.1"]`) {
		t.Fatal("override changed topology or array semantics")
	}
	ctx.Overrides[0].Path = "/enable"
	if _, err := ResolveSpec(spec, ctx); err == nil {
		t.Fatal("override can resurrect a removed protocol")
	}
}

func TestDisabledSpecDoesNotDemandRuntimeMaterial(t *testing.T) {
	spec, _ := NormalizeSpec(specExample(), nil)
	spec.Entries[0].Enable = false
	resolved, err := ResolveSpec(spec, ResolveContext{})
	if err != nil || resolved.Entries[0].Enable || len(resolved.Entries[0].Sources) == 0 {
		t.Fatal("disabled definition demanded material or lost its future sources", err)
	}
	ctx := specContext("node.example.com")
	spec.Entries[0].Enable = true
	ctx.Overrides = []SpecOverride{{ConnectionKey: "vless-primary", Path: "/tls", Operation: "set", Source: ValueSource{Kind: "profile", Name: "base"}}, {ConnectionKey: "vless-primary", Path: "/tls/server_name", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"custom.example.com"`)}}}
	if _, err := ResolveSpec(spec, ctx); err == nil {
		t.Fatal("overlapping overrides were silently ordered")
	}
	spec.Entries[0].Key = ""
	if _, err := ResolveSpec(spec, ctx); err == nil {
		t.Fatal("resolution generated a persistent connection key")
	}
}

func TestNodeObfuscationOverrideAndClientPorts(t *testing.T) {
	raw := json.RawMessage(`{"schema_version":1,"config":{},"entries":[{"connection_key":"hy","name":"Hysteria2","protocol":"hysteria2","enable":true,"options":{"listen":"::","listen_port":443,"tls":{"enabled":true}},"client_options":{"server_ports":["443:8443"],"hop_interval":"30s"}}]}`)
	spec, err := NormalizeSpec(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ResolveContext{
		Generated: map[string]json.RawMessage{"hy/hysteria2_obfs_password": json.RawMessage(`"synthetic-obfuscation"`)},
		Overrides: []SpecOverride{
			{ConnectionKey: "hy", Path: "/obfs/type", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"salamander"`)}},
			{ConnectionKey: "hy", Path: "/obfs/password", Operation: "set", Source: ValueSource{Kind: "node_generated", Purpose: "hysteria2_obfs_password"}},
		},
	}
	result, err := ResolveSpec(spec, ctx)
	if err != nil {
		t.Fatal(err)
	}
	entry := result.Entries[0]
	if !strings.Contains(string(entry.Options), `"obfs":{"password":"synthetic-obfuscation","type":"salamander"}`) {
		t.Fatal("generated obfuscation was not installed in resolved options")
	}
	addresses, outbound, err := clientPublication(entry.ClientOptions, "node.example.com", 443)
	if err != nil || !strings.Contains(string(addresses), `"server":"node.example.com"`) || !strings.Contains(string(outbound), `"hop_interval":"30s"`) {
		t.Fatal("client publication lost the node address or client port settings", err)
	}
}

func TestVLESSQUICUsesUDP(t *testing.T) {
	if listenerNetwork("vless", listenerTransport{Type: "quic"}) != "udp" || listenerNetwork("vless", listenerTransport{Type: "ws"}) != "tcp" {
		t.Fatal("VLESS listener must use its transport network")
	}
	spec := TemplateSpec{Entries: []SpecEntry{{Key: "vless", Protocol: "vless", Enable: true, Options: json.RawMessage(`{"listen_port":443,"transport":{"type":"ws"}}`)}}}
	overrides := map[string][]SpecOverride{"vless": {{Path: "/transport/type", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"quic"`)}}}}
	requests, err := listenerRequests(nil, model.Node{}, spec, overrides)
	if err != nil || len(requests) != 1 || requests[0].Network != "udp" {
		t.Fatal("an individual QUIC override must reserve and probe UDP", err)
	}
}

func TestNodeRealmUsesSeparateCredentials(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	entry := SpecEntry{Key: "hysteria", Protocol: "hysteria2", Options: json.RawMessage(`{"realm":{"server_url":"https://realm.example","token":"synthetic-server"}}`), ClientOptions: json.RawMessage(`{"realm":{"token":"synthetic-client"}}`)}
	if err := sealRealmDraft(&entry); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(entry.Options)+string(entry.ClientOptions), "synthetic-") || entry.RealmServerSecret == "" || entry.RealmClientSecret == "" {
		t.Fatal("Realm draft exposes credentials or lost encrypted material")
	}
	swapped := entry
	swapped.RealmClientSecret = entry.RealmServerSecret
	if err := openRealmDraft(&swapped); err == nil {
		t.Fatal("the server credential can be reused as the client credential")
	}
	if err := openRealmDraft(&entry); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry.Options), "synthetic-server") || !strings.Contains(string(entry.ClientOptions), "synthetic-client") {
		t.Fatal("Realm credentials were restored to the wrong sides")
	}
	inbound := model.NodeInbound{Options: entry.Options, ClientOptions: entry.ClientOptions}
	redactInbound(&inbound)
	if strings.Contains(string(inbound.Options)+string(inbound.ClientOptions), "synthetic-") {
		t.Fatal("materialized Realm credentials entered the editor projection")
	}
}
