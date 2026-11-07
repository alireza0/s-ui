package util

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"github.com/sagernet/sing-box/common/tls"
)

func echTls(t *testing.T, serverEnabled bool) (*model.Tls, string) {
	t.Helper()
	configPem, _, err := tls.ECHKeygenDefault("public.example.com")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(configPem), "\n")
	client, _ := json.Marshal(map[string]interface{}{"ech": map[string]interface{}{"config": lines}})
	server, _ := json.Marshal(map[string]interface{}{
		"enabled": true, "server_name": "real.example.com",
		"ech": map[string]interface{}{"enabled": serverEnabled},
	})
	return &model.Tls{Server: server, Client: client}, strings.Join(lines[1:len(lines)-1], "")
}

// Links carried no ECH at all, so a client imported from one never used it.
// #1339
func TestLinksCarryECHConfig(t *testing.T) {
	tlsRow, want := echTls(t, true)
	fields := map[string]map[string]interface{}{
		"vless":     {"uuid": "11111111-1111-1111-1111-111111111111"},
		"trojan":    {"password": "secret"},
		"hysteria2": {"password": "secret"},
	}
	for proto, f := range fields {
		t.Run(proto, func(t *testing.T) {
			inbound := inboundFor(t, proto, nil)
			inbound.TlsId, inbound.Tls = 1, tlsRow
			links := LinkGenerator(clientConfigFor(t, proto, f), inbound, "example.test", "")
			if len(links) != 1 {
				t.Fatalf("got %d links", len(links))
			}
			u, err := url.Parse(links[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := u.Query().Get("ech"); got != want {
				t.Fatalf("ech = %q, want the bare ECHConfigList %q", got, want)
			}

			// And back: the importer has to hand sing-box a PEM block again
			out, _, err := GetOutbound(links[0], 0)
			if err != nil {
				t.Fatal(err)
			}
			ech, _ := (*out)["tls"].(map[string]interface{})["ech"].(map[string]interface{})
			config, _ := ech["config"].([]string)
			if len(config) != 3 || config[0] != "-----BEGIN ECH CONFIGS-----" || config[1] != want {
				t.Fatalf("imported ech = %v", ech)
			}
		})
	}
}

func TestVmessLinkCarriesECHConfig(t *testing.T) {
	tlsRow, want := echTls(t, true)
	inbound := inboundFor(t, "vmess", nil)
	inbound.TlsId, inbound.Tls = 1, tlsRow
	links := LinkGenerator(clientConfigFor(t, "vmess", map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111"}), inbound, "example.test", "")
	raw, err := B64StrToByte(strings.TrimPrefix(links[0], "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["ech"] != want {
		t.Fatalf("vmess ech = %v, want %s", obj["ech"], want)
	}
}

// The client half can outlive the server's switch; the server decides.
func TestLinksSkipECHWhenServerDisabled(t *testing.T) {
	tlsRow, _ := echTls(t, false)
	inbound := inboundFor(t, "trojan", nil)
	inbound.TlsId, inbound.Tls = 1, tlsRow
	links := LinkGenerator(clientConfigFor(t, "trojan", map[string]interface{}{"password": "secret"}), inbound, "example.test", "")
	if strings.Contains(links[0], "ech=") {
		t.Fatalf("ech sent while the server has it off: %s", links[0])
	}
}

func TestECHFromLinkForms(t *testing.T) {
	if ech := echFromLink("cloudflare-ech.com+https://dns.example/dns-query"); ech["query_server_name"] != "cloudflare-ech.com" {
		t.Errorf("domain+DNS form = %v", ech)
	}
	if ech := echFromLink("not base64 at all"); ech != nil {
		t.Errorf("garbage = %v, want nil", ech)
	}
	if ech := echFromLink(""); ech != nil {
		t.Errorf("empty = %v, want nil", ech)
	}
}
