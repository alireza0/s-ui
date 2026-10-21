package sub

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alireza0/s-ui/util"
	"gopkg.in/yaml.v3"
)

func TestClashALPNFromParsedLinkAndJSON(t *testing.T) {
	openClashTestSettings(t)
	const uri = "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:443?type=ws&path=%2Fsocket&host=ws.example&security=tls&sni=ws.example&alpn=h2%2Chttp%2F1.1#WebSocket"
	for _, decodedJSON := range []bool{false, true} {
		name := "parsed-link"
		if decodedJSON {
			name = "decoded-json"
		}
		t.Run(name, func(t *testing.T) {
			outbound, _, err := util.GetOutbound(uri, 0)
			if err != nil || outbound == nil {
				t.Fatalf("parse external link: %v", err)
			}
			var wantInput interface{} = []string{"h2", "http/1.1"}
			if decodedJSON {
				data, err := json.Marshal(outbound)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, outbound); err != nil {
					t.Fatal(err)
				}
				wantInput = []interface{}{"h2", "http/1.1"}
			}
			tls := (*outbound)["tls"].(map[string]interface{})
			if !reflect.DeepEqual(tls["alpn"], wantInput) {
				t.Fatalf("unexpected ALPN representation: %#v", tls["alpn"])
			}
			outbounds := []map[string]interface{}{*outbound}
			result, err := (&ClashService{}).ConvertToClashMeta(&outbounds, "mode: rule\n")
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Proxies []map[string]interface{} `yaml:"proxies"`
			}
			if err := yaml.Unmarshal([]byte(result), &config); err != nil {
				t.Fatal(err)
			}
			if len(config.Proxies) != 1 {
				t.Fatalf("got %d proxies, want one", len(config.Proxies))
			}
			proxy := config.Proxies[0]
			if proxy["network"] != "ws" || proxy["tls"] != true || proxy["servername"] != "ws.example" {
				t.Fatalf("WebSocket TLS parameters changed: %v", proxy)
			}
			if want := []interface{}{"h2", "http/1.1"}; !reflect.DeepEqual(proxy["alpn"], want) {
				t.Fatalf("Clash YAML ALPN = %#v, want %#v", proxy["alpn"], want)
			}
			if !reflect.DeepEqual(tls["alpn"], wantInput) {
				t.Fatalf("conversion mutated source ALPN: %#v", tls["alpn"])
			}
		})
	}
}
