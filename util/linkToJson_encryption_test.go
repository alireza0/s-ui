package util

import (
	"net/url"
	"reflect"
	"testing"
)

func TestGetOutboundForClashVLESSEncryption(t *testing.T) {
	const encrypted = "mlkem768x25519plus.native.1rtt.A+B/C=="
	// Encryption must survive URI decoding for every VLESS transport.
	for _, transport := range []string{"xhttp", "ws"} {
		for _, test := range []struct {
			name       string
			present    bool
			encryption string
		}{
			{name: "absent"},
			{name: "empty", present: true},
			{name: "none", present: true, encryption: "none"},
			{name: "encrypted", present: true, encryption: encrypted},
		} {
			t.Run(transport+"/"+test.name, func(t *testing.T) {
				query := url.Values{"type": {transport}, "security": {"tls"}, "sni": {"test.example"}, "path": {"/upload"}}
				if test.present {
					query.Set("encryption", test.encryption)
				}
				uri := "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:443?" + query.Encode() + "#VLESS"
				outbound, tag, err := GetOutboundForClash(uri, 0)
				if err != nil || outbound == nil {
					t.Fatalf("parse Clash link: %v", err)
				}
				got, present := (*outbound)["encryption"]
				if present != (test.encryption != "") || present && got != test.encryption {
					t.Errorf("encryption = %v (present=%v), want %q", got, present, test.encryption)
				}
				ordinary, ordinaryTag, err := GetOutbound(uri, 0)
				if err != nil || ordinary == nil || ordinaryTag != tag {
					t.Fatalf("parse ordinary link: %v", err)
				}
				query.Del("encryption")
				plainURI := "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:443?" + query.Encode() + "#VLESS"
				want, _, err := GetOutbound(plainURI, 0)
				if err != nil || !reflect.DeepEqual(ordinary, want) {
					t.Fatal("Clash encryption changed ordinary sing-box parsing")
				}
			})
		}
	}
}

func TestGetOutboundForClashIgnoresOtherProtocolEncryption(t *testing.T) {
	const uri = "trojan://password@192.0.2.1:443?security=tls&type=ws&path=%2Fsocket&encryption=unused#Trojan"
	want, wantTag, err := GetOutbound(uri, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, gotTag, err := GetOutboundForClash(uri, 0)
	if err != nil || gotTag != wantTag || !reflect.DeepEqual(got, want) {
		t.Fatal("VLESS encryption handling changed another protocol")
	}
}
