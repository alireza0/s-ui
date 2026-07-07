package util

import "testing"

func TestShadowsocksClientConfigKey(t *testing.T) {
	tests := map[string]string{
		"2022-blake3-aes-128-gcm":       "shadowsocks16",
		"2022-blake3-aes-256-gcm":       "shadowsocks32",
		"2022-blake3-chacha20-poly1305": "shadowsocks32",
		"chacha20-ietf-poly1305":        "shadowsocks",
		"aes-256-gcm":                   "shadowsocks",
	}

	for method, want := range tests {
		if got := ShadowsocksClientConfigKey(method); got != want {
			t.Fatalf("ShadowsocksClientConfigKey(%q) = %q, want %q", method, got, want)
		}
	}
}
