package util

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
)

func xhttpTestURI(values url.Values) string {
	return "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:2053?" + values.Encode() + "#XHTTP%20test"
}

func TestGetOutboundForClashXHTTP(t *testing.T) {
	for _, transportType := range []string{"xhttp", "splithttp", "XHTTP"} {
		for _, extra := range []string{"", "null", "{}", `{"headers":null,"downloadSettings":null}`, "{invalid", "[]", `"string"`, "42", "true", "{} {}", `{"path":"/extra"} {}`, `{"path":"/extra"} trailing`} {
			t.Run(transportType+"/"+extra, func(t *testing.T) {
				query := url.Values{
					"type": {transportType}, "host": {"example.com"}, "path": {"/api/v1/upload"}, "mode": {"auto"},
					"security": {"tls"}, "sni": {"example.com"}, "alpn": {"h2,http/1.1"}, "fp": {"firefox"},
				}
				if extra != "" {
					query.Set("extra", extra)
				}
				uri := xhttpTestURI(query)
				outbound, tag, err := GetOutboundForClash(uri, 2)
				if err != nil || outbound == nil {
					t.Fatalf("conversion failed: %v", err)
				}
				transport := (*outbound)["transport"].(map[string]interface{})
				if transport["type"] != "xhttp" || transport["host"] != "example.com" || transport["path"] != "/api/v1/upload" || transport["mode"] != "auto" {
					t.Fatalf("lost transport settings: %v", transport)
				}
				if _, present := transport["extra"]; present != (extra == `{"headers":null,"downloadSettings":null}`) {
					t.Fatalf("unexpected Extra parsing for %q: %v", extra, transport)
				}
				if tag != "2.XHTTP test" || (*outbound)["server"] != "192.0.2.1" || (*outbound)["server_port"] != 2053 {
					t.Fatalf("lost endpoint or tag: %v", *outbound)
				}
				tls := (*outbound)["tls"].(map[string]interface{})
				if tls["enabled"] != true || tls["server_name"] != "example.com" || !reflect.DeepEqual(tls["alpn"], []string{"h2", "http/1.1"}) {
					t.Fatalf("lost TLS settings: %v", tls)
				}
				// The ordinary parser and JSON callers must still see their original
				// representation, including the unsupported/empty XHTTP transport.
				ordinary, _, err := GetOutbound(uri, 2)
				if err != nil {
					t.Fatal(err)
				}
				if len((*ordinary)["transport"].(map[string]interface{})) != 0 {
					t.Fatalf("XHTTP leaked into sing-box JSON path: %v", *ordinary)
				}
				if !reflect.DeepEqual((*ordinary)["tls"].(map[string]interface{})["alpn"], []string{"h2", "http/1.1"}) {
					t.Fatal("ordinary parser TLS was changed")
				}
			})
		}
	}
}

func TestGetOutboundForClashKeepsOtherProtocols(t *testing.T) {
	for _, uri := range []string{
		"vless://test@192.0.2.1:443?security=tls&type=ws&host=example.com&path=%2Fws&alpn=h2#ws",
		"vless://test@192.0.2.1:443?security=reality&type=grpc&serviceName=test&pbk=public&sid=1234&fp=chrome#grpc",
		"trojan://test@192.0.2.1:443?security=tls&type=ws&path=%2Ftest#trojan",
		"hy2://test@192.0.2.1:443?sni=example.com&obfs=salamander&obfs-password=test#hy2",
		"tuic://00000000-0000-4000-8000-000000000001:test@192.0.2.1:443?sni=example.com&congestion_control=bbr#tuic",
	} {
		t.Run(uri[:10], func(t *testing.T) {
			want, wantTag, wantErr := GetOutbound(uri, 1)
			got, gotTag, gotErr := GetOutboundForClash(uri, 1)
			if wantErr != nil || gotErr != nil || gotTag != wantTag || !reflect.DeepEqual(got, want) {
				t.Fatalf("changed unrelated URI: want=%v got=%v errors=%v/%v", want, got, wantErr, gotErr)
			}
		})
	}
}

func TestGetOutboundForClashTLSVerification(t *testing.T) {
	for _, value := range []string{"", "0", "false", "1", "true"} {
		for _, key := range []string{"allowInsecure", "insecure"} {
			query := url.Values{"type": {"xhttp"}, "security": {"tls"}, key: {value}}
			outbound, _, err := GetOutboundForClash(xhttpTestURI(query), 0)
			if err != nil {
				t.Fatal(err)
			}
			tls := (*outbound)["tls"].(map[string]interface{})
			insecure, _ := tls["insecure"].(bool)
			if insecure != (value == "1" || value == "true") {
				t.Errorf("%s=%q produced insecure=%v", key, value, insecure)
			}
		}
	}
}

func TestGetOutboundForClashExtraNumbersAndPresence(t *testing.T) {
	query := url.Values{
		"type": {"xhttp"}, "host": {""}, "mode": {"packet-up"},
		"extra": {`{"host":"extra.example","path":"/extra","xmux":{"maxConnections":4},"scMaxEachPostBytes":1000000}` + " \r\n\t"},
	}
	outbound, _, err := GetOutboundForClash(xhttpTestURI(query), 0)
	if err != nil {
		t.Fatal(err)
	}
	transport := (*outbound)["transport"].(map[string]interface{})
	if host, exists := transport["host"]; !exists || host != "" {
		t.Fatal("explicit empty top-level host lost its precedence")
	}
	if transport["mode"] != "packet-up" {
		t.Fatal("packet-up changed")
	}
	extra := transport["extra"].(map[string]interface{})
	if extra["scMaxEachPostBytes"] != json.Number("1000000") {
		t.Fatalf("numeric precision/type changed: %v", extra)
	}
}
