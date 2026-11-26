package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestSpecRejectsResolvedRealityHandshakeLoops(t *testing.T) {
	newEntry := func(key, listen string, port int, target string, targetPort int) SpecEntry {
		return SpecEntry{
			Key: key, Name: key, Protocol: "vless", Enable: true,
			Options: json.RawMessage(fmt.Sprintf(`{"listen":%q,"listen_port":%d,"tls":{"enabled":true,"reality":{"enabled":true,"handshake":{"server":%q,"server_port":%d}}}}`, listen, port, target, targetPort)),
		}
	}
	bound := newEntry("bound", "::", 443, "", 443)
	bound.Sources = map[string]ValueSource{
		"/listen_port":                  {Kind: "node_generated", Purpose: "listen_port"},
		"/tls/reality/handshake/server": {Kind: "node_parameter", Name: "primary_domain"},
	}
	ctx := ResolveContext{
		Parameters:     map[string]json.RawMessage{"primary_domain": json.RawMessage(`"NODE.EXAMPLE."`)},
		Generated:      map[string]json.RawMessage{"bound/listen_port": json.RawMessage(`443`)},
		LocalAddresses: []string{"node.example", "192.0.2.10"},
	}
	for _, test := range []struct {
		name    string
		entries []SpecEntry
		loop    bool
	}{
		{"domain source and automatic port", []SpecEntry{bound}, true},
		{"local management IP", []SpecEntry{newEntry("one", "::", 443, "::ffff:192.0.2.10", 443)}, true},
		{"loopback", []SpecEntry{newEntry("one", "::", 443, "[::1]", 443)}, true},
		{"separate local TLS service", []SpecEntry{newEntry("one", "::", 443, "localhost", 8443)}, false},
		{"different bind on the same port", []SpecEntry{newEntry("one", "192.0.2.10", 443, "127.0.0.1", 443)}, false},
		{"external TLS service", []SpecEntry{newEntry("one", "::", 443, "target.example", 443)}, false},
		{"two connection cycle", []SpecEntry{
			newEntry("one", "::", 443, "node.example", 8443),
			newEntry("two", "::", 8443, "127.0.0.1", 443),
		}, true},
		{"two connection chain", []SpecEntry{
			newEntry("one", "::", 443, "node.example", 8443),
			newEntry("two", "::", 8443, "target.example", 443),
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveSpec(TemplateSpec{SchemaVersion: 1, Config: json.RawMessage(`{}`), Entries: test.entries}, ctx)
			if test.loop && !errors.Is(err, errRealityHandshakeLoop) || !test.loop && err != nil {
				t.Fatalf("expected loop = %t, got %v", test.loop, err)
			}
		})
	}
}
