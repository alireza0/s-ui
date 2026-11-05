package util

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// nastyPasswords are what an operator can legitimately type into the password
// box. Every one of them used to produce a link that url.Parse rejected, and
// addParams dereferenced the nil it got back -- so one client with a space in
// their password took down link generation for the whole panel.
var nastyPasswords = []string{
	"plain",
	"p ss",
	"p%ss",
	"pü",
	"p#ss",
	"p/ss",
	"p:ss",
	"p?ss",
	"p@ss",
	"p&ss=x",
	"p\"ss",
	"🎈",
}

func inboundFor(t *testing.T, typ string, options map[string]interface{}) *model.Inbound {
	t.Helper()
	if options == nil {
		options = map[string]interface{}{}
	}
	options["listen_port"] = 443
	raw, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	return &model.Inbound{
		Type:    typ,
		Tag:     "inbound-tag",
		Options: json.RawMessage(raw),
		Addrs:   json.RawMessage(`[]`),
	}
}

func clientConfigFor(t *testing.T, proto string, fields map[string]interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{proto: fields})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Every protocol that puts credentials in the userinfo, over every password a
// user can actually have. The link has to parse back, and the credential has to
// survive the round trip.
func TestLinkGeneratorSurvivesAwkwardPasswords(t *testing.T) {
	protocols := []struct {
		typ     string
		proto   string
		fields  func(pass string) map[string]interface{}
		options map[string]interface{}
	}{
		{"socks", "socks", func(p string) map[string]interface{} {
			return map[string]interface{}{"username": "us er", "password": p}
		}, nil},
		{"http", "http", func(p string) map[string]interface{} {
			return map[string]interface{}{"username": "us er", "password": p}
		}, nil},
		{"trojan", "trojan", func(p string) map[string]interface{} {
			return map[string]interface{}{"password": p}
		}, nil},
		{"hysteria2", "hysteria2", func(p string) map[string]interface{} {
			return map[string]interface{}{"password": p}
		}, nil},
		{"anytls", "anytls", func(p string) map[string]interface{} {
			return map[string]interface{}{"password": p}
		}, nil},
		{"tuic", "tuic", func(p string) map[string]interface{} {
			return map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111", "password": p}
		}, nil},
		{"naive", "naive", func(p string) map[string]interface{} {
			return map[string]interface{}{"username": "us er", "password": p}
		}, nil},
		{"shadowsocks", "shadowsocks", func(p string) map[string]interface{} {
			return map[string]interface{}{"password": p}
		}, map[string]interface{}{"method": "aes-128-gcm"}},
	}

	for _, p := range protocols {
		for _, pass := range nastyPasswords {
			t.Run(p.typ+"/"+pass, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panicked generating a %s link for password %q: %v", p.typ, pass, r)
					}
				}()

				inbound := inboundFor(t, p.typ, p.options)
				links := LinkGenerator(clientConfigFor(t, p.proto, p.fields(pass)), inbound, "example.test", "someone")
				if len(links) == 0 {
					t.Fatalf("no link generated for %s", p.typ)
				}
				for _, link := range links {
					if _, err := url.Parse(link); err != nil {
						t.Errorf("generated link does not parse: %q: %v", link, err)
					}
				}
			})
		}
	}
}

// The password has to come back out, not just survive parsing. A link that
// parses but carries the wrong credential fails silently on the client.
func TestUserinfoRoundTrips(t *testing.T) {
	for _, pass := range nastyPasswords {
		t.Run(pass, func(t *testing.T) {
			inbound := inboundFor(t, "trojan", nil)
			links := LinkGenerator(clientConfigFor(t, "trojan",
				map[string]interface{}{"password": pass}), inbound, "example.test", "someone")
			if len(links) == 0 {
				t.Fatal("no link generated")
			}
			u, err := url.Parse(links[0])
			if err != nil {
				t.Fatalf("link does not parse: %v", err)
			}
			if got := u.User.Username(); got != pass {
				t.Errorf("password round trip: got %q, want %q", got, pass)
			}
		})
	}
}

// Remarks are user-controlled too, and they land in the fragment.
func TestRemarksWithAwkwardCharacters(t *testing.T) {
	for _, remark := range []string{"plain", "a b", "a#b", "a?b", "نام", "🎈"} {
		t.Run(remark, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on remark %q: %v", remark, r)
				}
			}()
			inbound := inboundFor(t, "trojan", nil)
			links := LinkGenerator(clientConfigFor(t, "trojan",
				map[string]interface{}{"password": "p"}), inbound, "example.test", remark)
			if len(links) == 0 {
				t.Fatal("no link generated")
			}
			u, err := url.Parse(links[0])
			if err != nil {
				t.Fatalf("link does not parse: %v", err)
			}
			if !strings.Contains(u.Fragment, remark) {
				t.Errorf("remark %q is not in the fragment %q", remark, u.Fragment)
			}
		})
	}
}

// A naive http2 link carries its credentials base64'd and unescaped, so the
// authority has to be split at the last @, not the first.
func TestNaiveLinkRoundTripsAwkwardCredentials(t *testing.T) {
	for _, pass := range []string{"plain", "p@ss", "p@s@s", "p:ss", "p ss"} {
		t.Run(pass, func(t *testing.T) {
			inbound := inboundFor(t, "naive", nil)
			links := LinkGenerator(clientConfigFor(t, "naive",
				map[string]interface{}{"username": "user", "password": pass}), inbound, "example.test", "someone")
			if len(links) == 0 {
				t.Fatal("no link generated")
			}

			var http2Link string
			for _, l := range links {
				if strings.HasPrefix(l, "http2://") {
					http2Link = l
					break
				}
			}
			if http2Link == "" {
				t.Fatalf("no http2 link among %v", links)
			}

			u, err := url.Parse(http2Link)
			if err != nil {
				t.Fatalf("link does not parse: %v", err)
			}
			out, _, err := parseNaiveLink(u, 0)
			if err != nil {
				t.Fatalf("parseNaiveLink: %v", err)
			}
			got := (*out)
			if got["server"] != "example.test" {
				t.Errorf("server = %v, want example.test", got["server"])
			}
			if got["username"] != "user" {
				t.Errorf("username = %v, want user", got["username"])
			}
			if got["password"] != pass {
				t.Errorf("password round trip: got %v, want %q", got["password"], pass)
			}
		})
	}
}

// grpc carries its service name in the transport, and the vmess JSON has no
// field for it -- it goes in "path". It used to be dropped, so every grpc vmess
// link pointed at the default service.
func TestVmessGrpcCarriesTheServiceName(t *testing.T) {
	inbound := inboundFor(t, "vmess", map[string]interface{}{
		"transport": map[string]interface{}{"type": "grpc", "service_name": "MyService"},
	})
	links := LinkGenerator(clientConfigFor(t, "vmess",
		map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111"}), inbound, "example.test", "someone")
	if len(links) == 0 {
		t.Fatal("no link generated")
	}

	raw, err := B64StrToByte(strings.TrimPrefix(links[0], "vmess://"))
	if err != nil {
		t.Fatalf("vmess payload is not base64: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("vmess payload is not JSON: %v", err)
	}
	if obj["net"] != "grpc" {
		t.Errorf("net = %v, want grpc", obj["net"])
	}
	if obj["path"] != "MyService" {
		t.Errorf("path = %v, want MyService -- the grpc service name was dropped", obj["path"])
	}
}

func TestWsLinksFallBackToTheSniForHost(t *testing.T) {
	addrs := json.RawMessage(`[
		{"server": "a.example.com", "server_port": 443, "tls": {"server_name": "one.example.com"}},
		{"server": "b.example.com", "server_port": 443, "tls": {"server_name": "two.example.com"}}
	]`)
	transport := map[string]interface{}{"type": "ws", "path": "/ws"}
	uid := map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111"}
	for _, proto := range []string{"vless", "trojan", "vmess"} {
		t.Run(proto, func(t *testing.T) {
			inbound := inboundFor(t, proto, map[string]interface{}{"transport": transport})
			inbound.Addrs = addrs
			inbound.TlsId = 1
			inbound.Tls = &model.Tls{
				Server: json.RawMessage(`{"enabled": true, "server_name": "origin.example.com"}`),
				Client: json.RawMessage(`{}`),
			}
			fields := uid
			if proto == "trojan" {
				fields = map[string]interface{}{"password": "secret"}
			}
			links := LinkGenerator(clientConfigFor(t, proto, fields), inbound, "example.test", "")
			if len(links) != 2 {
				t.Fatalf("got %d links, want 2", len(links))
			}
			for i, want := range []string{"one.example.com", "two.example.com"} {
				if got := linkHost(t, links[i]); got != want {
					t.Errorf("link %d host = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestWsLinksKeepAnExplicitHost(t *testing.T) {
	inbound := inboundFor(t, "vless", map[string]interface{}{"transport": map[string]interface{}{
		"type": "ws", "path": "/ws", "headers": map[string]interface{}{"Host": "set.example.com"},
	}})
	inbound.TlsId = 1
	inbound.Tls = &model.Tls{
		Server: json.RawMessage(`{"enabled": true, "server_name": "sni.example.com"}`),
		Client: json.RawMessage(`{}`),
	}
	links := LinkGenerator(clientConfigFor(t, "vless",
		map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111"}), inbound, "example.test", "")
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if got := linkHost(t, links[0]); got != "set.example.com" {
		t.Errorf("host = %q, want the one set on the transport", got)
	}
}

func TestWsLinksHaveOneHostWhenStoredEmpty(t *testing.T) {
	transports := map[string]map[string]interface{}{
		"ws":          {"type": "ws", "path": "/ws", "headers": map[string]interface{}{"Host": ""}},
		"httpupgrade": {"type": "httpupgrade", "path": "/u", "host": ""},
	}
	clients := map[string]map[string]interface{}{
		"vless":  {"uuid": "11111111-1111-1111-1111-111111111111"},
		"trojan": {"password": "secret"},
	}
	for name, transport := range transports {
		for proto, fields := range clients {
			t.Run(proto+"/"+name, func(t *testing.T) {
				inbound := inboundFor(t, proto, map[string]interface{}{"transport": transport})
				inbound.TlsId = 1
				inbound.Tls = &model.Tls{
					Server: json.RawMessage(`{"enabled": true, "server_name": "sni.example.com"}`),
					Client: json.RawMessage(`{}`),
				}
				links := LinkGenerator(clientConfigFor(t, proto, fields), inbound, "example.test", "")
				if len(links) != 1 {
					t.Fatalf("got %d links, want 1", len(links))
				}
				parsed, err := url.Parse(links[0])
				if err != nil {
					t.Fatal(err)
				}
				hosts := parsed.Query()["host"]
				if len(hosts) != 1 || hosts[0] != "sni.example.com" {
					t.Errorf("host params = %q, want only the SNI", hosts)
				}
			})
		}
	}
}

func linkHost(t *testing.T, link string) string {
	t.Helper()
	if strings.HasPrefix(link, "vmess://") {
		raw, err := B64StrToByte(strings.TrimPrefix(link, "vmess://"))
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatal(err)
		}
		host, _ := obj["host"].(string)
		return host
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query().Get("host")
}

func TestDefaultTransportHost(t *testing.T) {
	tls := map[string]interface{}{"enabled": true, "server_name": "sni.example.com"}
	tests := []struct {
		name      string
		transport map[string]interface{}
		tls       map[string]interface{}
		want      string
	}{
		{"ws", map[string]interface{}{"type": "ws"}, tls, "sni.example.com"},
		{"httpupgrade", map[string]interface{}{"type": "httpupgrade"}, tls, "sni.example.com"},
		{"ws with Host", map[string]interface{}{"type": "ws", "headers": map[string]interface{}{"Host": "set.example.com"}}, tls, ""},
		{"ws with Host list", map[string]interface{}{"type": "ws", "headers": map[string]interface{}{"Host": []interface{}{"set.example.com"}}}, tls, ""},
		{"httpupgrade with host", map[string]interface{}{"type": "httpupgrade", "host": "set.example.com"}, tls, ""},
		{"ws with empty Host", map[string]interface{}{"type": "ws", "headers": map[string]interface{}{"Host": ""}}, tls, "sni.example.com"},
		{"httpupgrade with empty host", map[string]interface{}{"type": "httpupgrade", "host": ""}, tls, "sni.example.com"},
		{"grpc", map[string]interface{}{"type": "grpc"}, tls, ""},
		{"tls off", map[string]interface{}{"type": "ws"}, map[string]interface{}{"enabled": false, "server_name": "sni.example.com"}, ""},
		{"no server_name", map[string]interface{}{"type": "ws"}, map[string]interface{}{"enabled": true}, ""},
		{"no tls", map[string]interface{}{"type": "ws"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultTransportHost(tt.transport, tt.tls); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetTransportHostCopies(t *testing.T) {
	ws := map[string]interface{}{"type": "ws", "headers": map[string]interface{}{"User-Agent": "test"}}
	headers, _ := SetTransportHost(ws, "host.example.com")["headers"].(map[string]interface{})
	if headers["Host"] != "host.example.com" || headers["User-Agent"] != "test" {
		t.Errorf("headers = %v", headers)
	}
	if _, changed := ws["headers"].(map[string]interface{})["Host"]; changed {
		t.Error("the shared transport was changed")
	}

	upgrade := map[string]interface{}{"type": "httpupgrade"}
	if got := SetTransportHost(upgrade, "host.example.com")["host"]; got != "host.example.com" {
		t.Errorf("httpupgrade host = %v", got)
	}
	if _, changed := upgrade["host"]; changed {
		t.Error("the shared transport was changed")
	}
}
