package util

import (
	"reflect"
	"testing"
)

func TestGetOutboundHTTPProxy(t *testing.T) {
	// HTTP links must keep credentials and remain usable without a fragment.
	for _, test := range []struct {
		name, uri, tag, host, username, password string
		port, index                              int
	}{
		{name: "named", uri: "http://user:pass@192.0.2.1:3071#USA", tag: "USA", host: "192.0.2.1", port: 3071, username: "user", password: "pass"},
		{name: "unnamed", uri: "http://user:pass@proxy.example:3071", tag: "proxy.example:3071", host: "proxy.example", port: 3071, username: "user", password: "pass"},
		{name: "escaped", uri: "http://user%40name:p%40ss%3Aword%2B%23%25@proxy.example:8080#%D0%A1%D0%A8%D0%90%20%7C%20HTTP%2520", tag: "США | HTTP%20", host: "proxy.example", port: 8080, username: "user@name", password: "p@ss:word+#%"},
		{name: "no-auth", uri: "http://192.0.2.2:80", tag: "192.0.2.2:80", host: "192.0.2.2", port: 80},
		{name: "username-only", uri: "http://user@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user"},
		{name: "empty-password", uri: "http://user:@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user"},
		{name: "password-colon", uri: "http://user:pass:word@proxy.example:8080#Proxy", tag: "Proxy", host: "proxy.example", port: 8080, username: "user", password: "pass:word"},
		{name: "indexed", uri: "http://user:pass@proxy.example:3071#USA", tag: "3.USA", host: "proxy.example", port: 3071, username: "user", password: "pass", index: 3},
		{name: "indexed-unnamed", uri: "http://proxy.example:3071", tag: "2.proxy.example:3071", host: "proxy.example", port: 3071, index: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := map[string]interface{}{"type": "http", "tag": test.tag, "server": test.host, "server_port": test.port, "username": test.username, "password": test.password}
			for _, parse := range []func(string, int) (*map[string]interface{}, string, error){GetOutbound, GetOutboundForClash} {
				outbound, tag, err := parse(test.uri, test.index)
				if err != nil || outbound == nil {
					t.Fatalf("parse HTTP proxy: %v", err)
				}
				if tag != test.tag || !reflect.DeepEqual(*outbound, want) {
					t.Errorf("got %v / %q, want %v / %q", *outbound, tag, want, test.tag)
				}
			}
		})
	}
}

func TestGetOutboundRejectsInvalidHTTPProxy(t *testing.T) {
	for _, uri := range []string{
		"http://user:pass@:3071", "http://proxy.example", "http://proxy.example:",
		"http://proxy.example:0", "http://proxy.example:65536", "http://proxy.example:abc",
		"http:///missing-host:8080", "http://user:%zz@proxy.example:8080", "http://proxy.example:8080#%zz",
	} {
		t.Run(uri, func(t *testing.T) {
			outbound, tag, err := GetOutbound(uri, 0)
			if err == nil || outbound != nil || tag != "" {
				t.Errorf("invalid HTTP link accepted: %v / %q / %v", outbound, tag, err)
			}
		})
	}
}
