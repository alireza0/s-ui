package sub

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// Snell had no client side tab, so its addresses could not be set; the JSON
// subscription builds one outbound per address once they are. #1329
func TestGetOutboundsSnellPerAddress(t *testing.T) {
	inbound := &model.Inbound{
		Type:    "snell",
		Tag:     "snell",
		OutJson: json.RawMessage(`{"type": "snell", "tag": "snell", "server": "origin.example.com", "server_port": 8443, "version": 6, "psk": "secret"}`),
		Addrs:   json.RawMessage(`[{"server": "one.example.com", "server_port": 443, "remark": "-a"}, {"server": "two.example.com", "server_port": 8443, "remark": "-b"}]`),
	}
	outbounds, tags, err := (&JsonService{}).getOutbounds(json.RawMessage(`{}`), []*model.Inbound{inbound}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(*outbounds) != 2 {
		t.Fatalf("got %d outbounds, want one per address", len(*outbounds))
	}
	for i, want := range []string{"one.example.com", "two.example.com"} {
		out := (*outbounds)[i]
		if out["server"] != want || out["psk"] != "secret" || out["version"] != float64(6) {
			t.Errorf("outbound %d = %v", i, out)
		}
	}
	if (*tags)[0] == (*tags)[1] {
		t.Errorf("tags collide: %v", *tags)
	}
}
