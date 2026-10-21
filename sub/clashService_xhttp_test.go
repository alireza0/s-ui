package sub

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gopkg.in/yaml.v3"
)

func openClashTestSettings(t *testing.T) {
	t.Helper()
	if err := database.OpenDB(filepath.Join(t.TempDir(), "settings.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

// XHTTP links may carry populated, zero-valued, or null extra settings.
func fourXHTTPLinks(t *testing.T) ([]string, json.RawMessage) {
	t.Helper()
	fullExtra := `{
		"host":"","path":"","mode":"","headers":null,
		"xPaddingBytes":"100-1000","xPaddingObfsMode":false,
		"xPaddingKey":"","xPaddingHeader":"","xPaddingPlacement":"","xPaddingMethod":"",
		"uplinkHTTPMethod":"","sessionIDPlacement":"","sessionIDKey":"","sessionIDTable":"","sessionIDLength":"0",
		"seqPlacement":"","seqKey":"","uplinkDataPlacement":"","uplinkDataKey":"","uplinkChunkSize":"0",
		"noGRPCHeader":false,"noSSEHeader":false,"scMaxEachPostBytes":"1000000","scMinPostsIntervalMs":"30",
		"scMaxBufferedPosts":0,"scStreamUpServerSecs":"0","serverMaxHeaderBytes":0,
		"xmux":{"maxConcurrency":"16-32","maxConnections":"0","cMaxReuseTimes":"64-128","hMaxRequestTimes":"0","hMaxReusableSecs":"0","hKeepAlivePeriod":0},
		"downloadSettings":null,"extra":null
	}`
	var extra map[string]interface{}
	if err := json.Unmarshal([]byte(fullExtra), &extra); err != nil {
		t.Fatal(err)
	}
	extra["scMaxEachPostBytes"] = "0"
	extra["scMinPostsIntervalMs"] = "0"
	zeroExtra, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}
	extras := []string{fullExtra, string(zeroExtra), "null", fullExtra}
	var uris []string
	var links []Link
	for index, rawExtra := range extras {
		host := fmt.Sprintf("test-%d.example", index+1)
		query := url.Values{
			"type": {"xhttp"}, "host": {host}, "path": {"/api/v1/upload"}, "mode": {"auto"},
			"security": {"tls"}, "sni": {host}, "fp": {"firefox"}, "alpn": {"h2,http/1.1"},
			"encryption": {"none"}, "extra": {rawExtra},
		}
		uri := fmt.Sprintf("vless://00000000-0000-4000-8000-00000000000%d@192.0.2.%d:2053?%s#XHTTP", index+1, index+1, query.Encode())
		uris = append(uris, uri)
		links = append(links, Link{Type: "external", Uri: uri})
	}
	raw, err := json.Marshal(links)
	if err != nil {
		t.Fatal(err)
	}
	return uris, json.RawMessage(raw)
}

func TestExternalXHTTPClashSubscription(t *testing.T) {
	openClashTestSettings(t)
	service := &ClashService{}
	uris, raw := fourXHTTPLinks(t)
	originalJSON := append([]byte(nil), raw...)
	if got := service.LinkService.GetLinks(&raw, "all", ""); !reflect.DeepEqual(got, uris) {
		t.Fatal("Happ/raw subscription changed the source URI")
	}
	ordinaryBefore, ordinaryTags := service.LinkService.GetExternalOutbounds(&raw)
	outbounds, tags := service.LinkService.GetExternalOutboundsForClash(&raw)
	if len(outbounds) != 4 || !reflect.DeepEqual(tags, []string{"XHTTP", "XHTTP-1", "XHTTP-2", "XHTTP-3"}) {
		t.Fatalf("four external nodes or unique names missing: %v", tags)
	}
	result, err := service.ConvertToClashMeta(&outbounds, "mode: rule\nrules:\n  - MATCH,Proxy\n")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(result), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Proxies) != 4 {
		t.Fatalf("wrong proxy count: %d", len(document.Proxies))
	}
	for index, proxy := range document.Proxies {
		host := fmt.Sprintf("test-%d.example", index+1)
		if proxy["name"] != tags[index] || proxy["type"] != "vless" || proxy["network"] != "xhttp" || proxy["server"] != fmt.Sprintf("192.0.2.%d", index+1) || proxy["port"] != 2053 {
			t.Fatalf("endpoint/name/transport lost for node %d: %v", index, proxy)
		}
		if proxy["uuid"] != fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", index+1) || proxy["tls"] != true || proxy["servername"] != host || proxy["client-fingerprint"] != "firefox" {
			t.Fatalf("identity or TLS lost for node %d: %v", index, proxy)
		}
		if !reflect.DeepEqual(proxy["alpn"], []interface{}{"h2", "http/1.1"}) {
			t.Fatalf("ALPN lost for node %d: %v", index, proxy["alpn"])
		}
		if insecure, _ := proxy["skip-cert-verify"].(bool); insecure {
			t.Fatal("certificate verification disabled without URI permission")
		}
		opts := proxy["xhttp-opts"].(map[string]interface{})
		if opts["host"] != host || opts["path"] != "/api/v1/upload" || opts["mode"] != "auto" {
			t.Fatalf("top-level XHTTP options lost for node %d: %v", index, opts)
		}
		if index == 2 {
			if len(opts) != 3 {
				t.Fatalf("null extra produced unexpected fields: %v", opts)
			}
			continue
		}
		if opts["x-padding-bytes"] != "100-1000" {
			t.Fatalf("padding range lost: %v", opts)
		}
		reuse := opts["reuse-settings"].(map[string]interface{})
		if reuse["max-concurrency"] != "16-32" || reuse["max-connections"] != "0" || reuse["c-max-reuse-times"] != "64-128" {
			t.Fatalf("XMUX settings lost: %v", reuse)
		}
		for _, key := range []string{"session-length", "uplink-chunk-size", "download-settings", "no-sse-header", "sc-max-buffered-posts", "sc-stream-up-server-secs", "server-max-header-bytes"} {
			if _, exists := opts[key]; exists {
				t.Errorf("default-zero/server-only option must be omitted: %s", key)
			}
		}
		if index == 1 {
			for _, key := range []string{"sc-max-each-post-bytes", "sc-min-posts-interval-ms"} {
				if _, exists := opts[key]; exists {
					t.Fatalf("Mihomo rejects explicit zero sc option: %s=%v", key, opts[key])
				}
			}
		} else if opts["sc-max-each-post-bytes"] != "1000000" || opts["sc-min-posts-interval-ms"] != "30" {
			t.Fatalf("sc parameters must remain string ranges: %v", opts)
		}
	}
	ordinaryAfter, afterTags := service.LinkService.GetExternalOutbounds(&raw)
	if !reflect.DeepEqual(ordinaryBefore, ordinaryAfter) || !reflect.DeepEqual(ordinaryTags, afterTags) || !reflect.DeepEqual([]byte(raw), originalJSON) {
		t.Fatal("Clash export mutated raw links or ordinary sing-box JSON generation")
	}
	for _, outbound := range ordinaryAfter {
		if len(outbound["transport"].(map[string]interface{})) != 0 {
			t.Fatal("Clash-only transport escaped into ordinary JSON generation")
		}
	}
}

func TestXHTTPOptionsPrecedenceAndPacketUp(t *testing.T) {
	extra := map[string]interface{}{"host": "extra.example", "path": "/extra", "mode": "auto"}
	got := xhttpOptsFromTransport(map[string]interface{}{
		"host": "uri.example", "path": "/uri", "mode": "packet-up", "extra": extra,
	})
	want := map[string]interface{}{"host": "uri.example", "path": "/uri", "mode": "packet-up"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("URI settings lost precedence: %v", got)
	}
	got = xhttpOptsFromTransport(map[string]interface{}{"host": "", "extra": extra})
	if _, exists := got["host"]; exists || got["path"] != "/extra" || got["mode"] != "auto" {
		t.Fatalf("explicit empty and absent fields conflated: %v", got)
	}
}

func TestXHTTPRangeMapping(t *testing.T) {
	for _, test := range []struct {
		input interface{}
		want  string
	}{
		{"100-1000", "100-1000"}, {" 10 - 20 ", "10-20"}, {json.Number("30"), "30"}, {float64(30), "30"},
		{"0", ""}, {"0-10", ""}, {"10-1", ""}, {"-1", ""}, {"1-2-3", ""}, {"1.5", ""},
		{"2147483647", "2147483647"}, {"2147483648", ""}, {"1-2147483648", ""},
		{float64(1.5), ""}, {true, ""}, {nil, ""}, {"9223372036854775808", ""},
	} {
		got := make(map[string]interface{})
		xhttpPutRange(got, "range", test.input, true)
		if test.want == "" {
			if len(got) != 0 {
				t.Errorf("invalid/zero range %v emitted as %v", test.input, got)
			}
		} else if got["range"] != test.want {
			t.Errorf("range %v: got %v want %q", test.input, got, test.want)
		}
	}
	reuse := xhttpMapReuse(map[string]interface{}{"maxConcurrency": "0", "maxConnections": json.Number("4"), "hKeepAlivePeriod": float64(-1)})
	if reuse["max-concurrency"] != "0" || reuse["max-connections"] != "4" || reuse["h-keep-alive-period"] != int64(-1) {
		t.Fatalf("zero concurrency erased connection limit or signed keepalive: %v", reuse)
	}
}

func TestXHTTPExtraMappingAndMalformedValues(t *testing.T) {
	var extra map[string]interface{}
	if err := json.Unmarshal([]byte(`{
		"headers":{"User-Agent":"test","Bad":42},"noGRPCHeader":false,"xPaddingObfsMode":true,
		"xPaddingKey":"custom","xPaddingHeader":"","sessionIDLength":"16-32","uplinkChunkSize":"2000-3000",
		"scMaxEachPostBytes":"1000000-2000000","scMinPostsIntervalMs":"30-60",
		"xmux":{"maxConcurrency":4,"maxConnections":0,"hKeepAlivePeriod":1.5},
		"downloadSettings":"invalid","unknown":"ignored"
	}`), &extra); err != nil {
		t.Fatal(err)
	}
	opts := xhttpOptsFromTransport(map[string]interface{}{"extra": extra})
	for key, want := range map[string]interface{}{
		"no-grpc-header": false, "x-padding-obfs-mode": true, "x-padding-key": "custom", "x-padding-header": "X-Padding",
		"x-padding-placement": "queryInHeader", "x-padding-method": "repeat-x", "session-length": "16-32",
		"uplink-chunk-size": "2000-3000", "sc-max-each-post-bytes": "1000000-2000000", "sc-min-posts-interval-ms": "30-60",
	} {
		if opts[key] != want {
			t.Errorf("%s: got %v want %v", key, opts[key], want)
		}
	}
	if !reflect.DeepEqual(opts["headers"], map[string]interface{}{"User-Agent": "test"}) {
		t.Errorf("malformed header not filtered: %v", opts["headers"])
	}
	if _, exists := opts["download-settings"]; exists {
		t.Fatal("malformed optional downloadSettings emitted")
	}
	if _, exists := opts["unknown"]; exists {
		t.Fatal("unknown Xray extra leaked into Mihomo")
	}
	for _, malformed := range []interface{}{nil, "bad", []interface{}{}, float64(1), true} {
		got := xhttpOptsFromTransport(map[string]interface{}{"host": "example.com", "extra": malformed})
		if !reflect.DeepEqual(got, map[string]interface{}{"host": "example.com"}) {
			t.Fatalf("malformed extra affected base transport: %v", got)
		}
	}
}

func TestXHTTPDownloadStreamConfig(t *testing.T) {
	var extra map[string]interface{}
	if err := json.Unmarshal([]byte(`{"downloadSettings":{
		"address":"download.example","port":8443,"security":"tls",
		"tlsSettings":{"serverName":"download-sni.example","fingerprint":"firefox","alpn":["h2"],"allowInsecure":false},
		"xhttpSettings":{"host":"download-host.example","path":"/download","headers":{"X-Test":"yes"},
			"extra":{"xmux":{"maxConcurrency":"8-16","maxConnections":"0"}}}
	}}`), &extra); err != nil {
		t.Fatal(err)
	}
	opts := xhttpOptsFromTransport(map[string]interface{}{"extra": extra})
	download := opts["download-settings"].(map[string]interface{})
	for key, want := range map[string]interface{}{
		"server": "download.example", "port": int64(8443), "tls": true, "servername": "download-sni.example",
		"client-fingerprint": "firefox", "skip-cert-verify": false, "host": "download-host.example", "path": "/download",
	} {
		if download[key] != want {
			t.Errorf("download %s: got %v want %v", key, download[key], want)
		}
	}
	if !reflect.DeepEqual(download["alpn"], []string{"h2"}) || !reflect.DeepEqual(download["headers"], map[string]interface{}{"X-Test": "yes"}) {
		t.Fatalf("download TLS/HTTP fields lost: %v", download)
	}
	reuse := download["reuse-settings"].(map[string]interface{})
	if reuse["max-concurrency"] != "8-16" {
		t.Fatalf("nested download XMUX lost: %v", reuse)
	}
	if got := xhttpMapDownload(map[string]interface{}{"security": "none"}); !reflect.DeepEqual(got, map[string]interface{}{
		"tls": false, "skip-cert-verify": false, "reality-opts": map[string]interface{}{},
	}) {
		t.Fatalf("explicit plaintext download override lost: %v", got)
	}
	if got := xhttpMapDownload(map[string]interface{}{"port": float64(65536), "tlsSettings": "bad", "xhttpSettings": []interface{}{}}); len(got) != 0 {
		t.Fatalf("malformed download fields emitted: %v", got)
	}
}

func TestXHTTPDownloadUsesSelectedSecurity(t *testing.T) {
	for _, test := range []struct {
		security    string
		wantTLS     bool
		wantSNI     string
		wantReality bool
	}{
		{"tls", true, "tls.example", false},
		{"reality", true, "reality.example", true},
		{"none", false, "", false},
		{"", false, "", false},
	} {
		t.Run(test.security, func(t *testing.T) {
			got := xhttpMapDownload(map[string]interface{}{
				"security":        test.security,
				"tlsSettings":     map[string]interface{}{"serverName": "tls.example", "allowInsecure": true},
				"realitySettings": map[string]interface{}{"serverName": "reality.example", "publicKey": "test-key"},
			})
			if got["tls"] != test.wantTLS {
				t.Fatalf("wrong TLS selection: %v", got)
			}
			sni, _ := got["servername"].(string)
			if sni != test.wantSNI {
				t.Errorf("inactive security mode overwrote SNI: %v", got)
			}
			reality, exists := got["reality-opts"].(map[string]interface{})
			if !exists || (len(reality) > 0) != test.wantReality {
				t.Errorf("inactive or inherited REALITY settings changed selected security: %v", got)
			}
			if got["skip-cert-verify"] != (test.security == "tls") {
				t.Errorf("inactive TLS settings changed certificate verification: %v", got)
			}
		})
	}
	got := xhttpMapDownload(map[string]interface{}{"security": "tls", "tlsSettings": map[string]interface{}{"serverName": "verified.example"}})
	if got["skip-cert-verify"] != false || !reflect.DeepEqual(got["reality-opts"], map[string]interface{}{}) {
		t.Fatalf("TLS download may inherit insecure or REALITY uplink settings: %v", got)
	}
	reuse := xhttpMapReuse(map[string]interface{}{"maxConcurrency": "0-9223372036854775807", "maxConnections": "4"})
	if _, exists := reuse["max-concurrency"]; exists || reuse["max-connections"] != "4" {
		t.Fatalf("overflowing range emitted or valid independent range lost: %v", reuse)
	}
}

func TestExternalNonXHTTPAndLocalLinksUnchanged(t *testing.T) {
	links := []Link{
		{Type: "external", Uri: "vless://test@192.0.2.1:443?type=grpc&security=reality&serviceName=test&pbk=public&sid=1234#grpc"},
		{Type: "external", Uri: "vless://test@192.0.2.1:443?type=ws&security=tls&host=example.com&path=%2Fws#ws"},
		{Type: "external", Uri: "hy2://test@192.0.2.1:443?sni=example.com#hy2"},
		{Type: "external", Uri: "tuic://00000000-0000-4000-8000-000000000001:test@192.0.2.1:443?sni=example.com#tuic"},
		{Type: "local", Uri: "vless://local@192.0.2.1:443?type=ws#local"},
	}
	data, err := json.Marshal(links)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(data)
	service := &LinkService{}
	want, wantTags := service.GetExternalOutbounds(&raw)
	got, gotTags := service.GetExternalOutboundsForClash(&raw)
	if len(got) != 4 || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotTags, wantTags) {
		t.Fatalf("other external transports/local handling changed: %v", got)
	}
	if gotLinks := service.GetLinks(&raw, "all", ""); len(gotLinks) != 5 || gotLinks[4] != links[4].Uri {
		t.Fatal("local/raw link generation changed")
	}
}
