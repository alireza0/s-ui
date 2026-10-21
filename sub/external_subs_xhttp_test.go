package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/util"
	"gopkg.in/yaml.v3"
)

func externalSubTestLinks(t *testing.T, links []Link) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(links)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(raw)
}

func externalSubTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func externalSubTestProxies(t *testing.T, service *ClashService, outbounds []map[string]interface{}) []map[string]interface{} {
	t.Helper()
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
	return document.Proxies
}

func TestExternalSubsXHTTPClashPlaintextAndBase64(t *testing.T) {
	openClashTestSettings(t)
	service := &ClashService{}
	uris, _ := fourXHTTPLinks(t)
	// The alias and an unrelated transport exercise the same fetched URI list.
	uris = append(uris, strings.Replace(uris[2], "type=xhttp", "type=splithttp", 1))
	uris = append(uris, "vless://00000000-0000-4000-8000-000000000099@192.0.2.99:443?type=ws&security=tls&sni=ws.example&host=ws.example&path=%2Fsocket#WebSocket")
	var direct []Link
	for _, uri := range uris {
		direct = append(direct, Link{Type: "external", Uri: uri})
	}
	directRaw := externalSubTestLinks(t, direct)
	wantClash, wantTags := service.GetExternalOutboundsForClash(&directRaw)
	wantOrdinary, wantOrdinaryTags := service.GetExternalOutbounds(&directRaw)
	wantProxies := externalSubTestProxies(t, service, wantClash)
	if len(wantProxies) != 6 {
		t.Fatalf("direct reference lost nodes: %d", len(wantProxies))
	}

	for _, format := range []struct {
		name   string
		base64 bool
	}{
		{name: "plaintext"},
		{name: "base64", base64: true},
	} {
		t.Run(format.name, func(t *testing.T) {
			// Blank lines, CRLF, surrounding whitespace and malformed URIs must
			// neither lose valid nodes nor cause a nil dereference.
			decoded := "\r\n  " + strings.Join(uris, "  \r\n\r\n  ") + "  \r\nunsupported://node\r\nvless://%zz\r\n\r\n"
			body := decoded
			if format.base64 {
				body = base64.StdEncoding.EncodeToString([]byte(decoded))
			}
			server := externalSubTestServer(t, body)
			raw := externalSubTestLinks(t, []Link{{Type: "sub", Uri: server.URL}})
			originalRaw := append([]byte(nil), raw...)

			ordinaryBefore, ordinaryTags := service.GetExternalOutbounds(&raw)
			if !reflect.DeepEqual(ordinaryBefore, wantOrdinary) || !reflect.DeepEqual(ordinaryTags, wantOrdinaryTags) {
				t.Fatalf("ordinary sing-box export changed: %v", ordinaryTags)
			}
			if got := service.GetLinks(&raw, "all", ""); !reflect.DeepEqual(got, strings.Split(decoded, "\n")) {
				t.Fatal("Happ/raw export changed the fetched source text")
			}

			outbounds, tags := service.GetExternalOutboundsForClash(&raw)
			if !reflect.DeepEqual(outbounds, wantClash) || !reflect.DeepEqual(tags, wantTags) {
				t.Fatalf("External Subs differs from direct External Links: tags=%v", tags)
			}
			proxies := externalSubTestProxies(t, service, outbounds)
			if !reflect.DeepEqual(proxies, wantProxies) {
				t.Fatal("fetched URI list produced different Clash proxies from direct links")
			}
			first := proxies[0]
			if first["network"] != "xhttp" || first["tls"] != true || first["servername"] != "test-1.example" || first["client-fingerprint"] != "firefox" || !reflect.DeepEqual(first["alpn"], []interface{}{"h2", "http/1.1"}) {
				t.Fatalf("XHTTP or TLS/ALPN missing: %v", first)
			}
			opts, ok := first["xhttp-opts"].(map[string]interface{})
			if !ok || opts["host"] != "test-1.example" || opts["path"] != "/api/v1/upload" || opts["mode"] != "auto" || opts["x-padding-bytes"] != "100-1000" || opts["sc-max-each-post-bytes"] != "1000000" || opts["sc-min-posts-interval-ms"] != "30" {
				t.Fatalf("XHTTP Extra fields missing: %v", opts)
			}
			reuse, ok := opts["reuse-settings"].(map[string]interface{})
			if !ok || reuse["max-concurrency"] != "16-32" || reuse["max-connections"] != "0" || reuse["c-max-reuse-times"] != "64-128" {
				t.Fatalf("XMUX fields missing: %v", reuse)
			}
			if proxies[4]["network"] != "xhttp" || proxies[5]["network"] != "ws" {
				t.Fatalf("alias or WebSocket lost: %v / %v", proxies[4], proxies[5])
			}
			ordinaryAfter, afterTags := service.GetExternalOutbounds(&raw)
			if !reflect.DeepEqual(ordinaryAfter, ordinaryBefore) || !reflect.DeepEqual(afterTags, ordinaryTags) || !reflect.DeepEqual([]byte(raw), originalRaw) {
				t.Fatal("Clash export mutated the source or ordinary sing-box export")
			}
		})
	}
}

func TestExternalSubsSingBoxJSONUnchanged(t *testing.T) {
	const body = `{"outbounds":[
		{"type":"direct","tag":"direct"},
		{"type":"selector","tag":"select","outbounds":["node"]},
		{"type":"vless","tag":"node","server":"192.0.2.30","server_port":443,"uuid":"00000000-0000-4000-8000-000000000030",
		 "tls":{"enabled":true,"server_name":"json.example","alpn":["h2"]},
		 "transport":{"type":"ws","path":"/json","headers":{"Host":"json.example"}},"custom_field":{"keep":true}}
	]}`
	var document struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(body), &document); err != nil {
		t.Fatal(err)
	}
	want := document.Outbounds[2:]
	for _, encoded := range []bool{false, true} {
		response := body
		if encoded {
			response = base64.StdEncoding.EncodeToString([]byte(body))
		}
		server := externalSubTestServer(t, response)
		for _, mode := range []struct {
			name  string
			parse func(string) ([]map[string]interface{}, error)
		}{
			{name: "ordinary", parse: util.GetExternalSub},
			{name: "clash", parse: util.GetExternalSubForClash},
		} {
			got, err := mode.parse(server.URL)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("sing-box JSON changed (base64=%v, export=%s): got=%v err=%v", encoded, mode.name, got, err)
			}
		}
		raw := externalSubTestLinks(t, []Link{{Type: "sub", Uri: server.URL}})
		var service LinkService
		ordinary, ordinaryTags := service.GetExternalOutbounds(&raw)
		clash, clashTags := service.GetExternalOutboundsForClash(&raw)
		if !reflect.DeepEqual(ordinary, want) || !reflect.DeepEqual(clash, want) || !reflect.DeepEqual(ordinaryTags, []string{"node"}) || !reflect.DeepEqual(clashTags, ordinaryTags) {
			t.Fatal("JSON input changed through LinkService")
		}
		if got := service.GetLinks(&raw, "all", ""); !reflect.DeepEqual(got, strings.Split(body, "\n")) {
			t.Fatal("raw JSON subscription changed")
		}
	}
}

func TestExternalSubsDuplicateNamesAcrossSources(t *testing.T) {
	uris, _ := fourXHTTPLinks(t)
	server := externalSubTestServer(t, strings.Join(uris, "\n"))
	raw := externalSubTestLinks(t, []Link{
		{Type: "external", Uri: uris[0]},
		{Type: "sub", Uri: server.URL},
		{Type: "external", Uri: uris[1]},
	})
	var service LinkService
	for _, parse := range []func(*json.RawMessage) ([]map[string]interface{}, []string){
		service.GetExternalOutbounds,
		service.GetExternalOutboundsForClash,
	} {
		outbounds, tags := parse(&raw)
		want := []string{"XHTTP", "XHTTP-1", "XHTTP-2", "XHTTP-3", "XHTTP-4", "XHTTP-5"}
		if !reflect.DeepEqual(tags, want) || len(outbounds) != len(want) {
			t.Fatalf("duplicate names changed: %v", tags)
		}
		for index, outbound := range outbounds {
			if outbound["tag"] != want[index] {
				t.Fatalf("tag differs from outbound name at %d: %v", index, outbound)
			}
		}
	}
}

func TestExternalSubsEmptyAndInvalidURILists(t *testing.T) {
	for _, body := range []string{"\r\n \t\r\n", "unsupported://node\r\nvless://%zz\r\n"} {
		server := externalSubTestServer(t, body)
		for _, parse := range []func(string) ([]map[string]interface{}, error){
			util.GetExternalSub,
			util.GetExternalSubForClash,
		} {
			outbounds, err := parse(server.URL)
			if err == nil || len(outbounds) != 0 {
				t.Fatalf("empty/invalid list should yield no result: %v, %v", outbounds, err)
			}
		}
	}
}
