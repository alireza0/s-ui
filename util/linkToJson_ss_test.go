package util

import (
	"encoding/base64"
	"testing"
)

// Shadowsocks links come base64-encoded in several flavours; strict standard
// decoding rejected most of them and the node vanished from Clash output,
// while the link subscription passed the line through untouched (#1281).
func TestShadowsocksLinkFormats(t *testing.T) {
	const method = "2022-blake3-aes-256-gcm"
	const password = "bWFza2VkLXZhbHVlMDAwMDAwMDAwMDAwMDAwMDAwMDA=:bWFza2VkLXZhbHVlMDAwMDAwMDAwMDAwMDAwMDAwMDA="
	userInfo := method + ":" + password

	cases := map[string]string{
		"padded standard":   base64.StdEncoding.EncodeToString([]byte(userInfo)) + "@[240e:1:2::1234]:22851",
		"unpadded url-safe": base64.RawURLEncoding.EncodeToString([]byte(userInfo)) + "@[240e:1:2::1234]:22851",
		"plugin path":       base64.RawURLEncoding.EncodeToString([]byte(userInfo)) + "@[240e:1:2::1234]:22851/?plugin=none",
		"legacy whole":      base64.StdEncoding.EncodeToString([]byte(userInfo + "@[240e:1:2::1234]:22851")),
		"legacy unpadded":   base64.RawURLEncoding.EncodeToString([]byte(userInfo + "@[240e:1:2::1234]:22851")),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			out, tag, err := GetOutbound("ss://"+body+"#node", 0)
			if err != nil {
				t.Fatal(err)
			}
			o := *out
			if tag != "node" || o["method"] != method || o["password"] != password ||
				o["server"] != "240e:1:2::1234" || o["server_port"] != 22851 {
				t.Errorf("got %v", o)
			}
		})
	}
}

// vmess payloads are often unpadded too.
func TestVmessUnpaddedLink(t *testing.T) {
	payload := base64.RawStdEncoding.EncodeToString([]byte(`{"v":"2","ps":"vm","add":"192.0.2.10","port":"443","id":"b831381d-6324-4d53-ad4f-8cda48b30811","net":"tcp"}`))
	out, _, err := GetOutbound("vmess://"+payload, 0)
	if err != nil {
		t.Fatal(err)
	}
	if (*out)["server"] != "192.0.2.10" {
		t.Errorf("got %v", *out)
	}
}
