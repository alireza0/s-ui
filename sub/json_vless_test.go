package sub

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestJsonOutboundsPreserveVlessVisionFlowForDefaultTcpTLS(t *testing.T) {
	outbound := vlessOutboundFromOptions(t, json.RawMessage(`{"transport":{}}`), map[string]interface{}{"enabled": true})

	if got := outbound["flow"]; got != "xtls-rprx-vision" {
		t.Fatalf("flow was not preserved: %v", got)
	}
}

func TestJsonOutboundsStripVlessVisionFlowForGrpcTLS(t *testing.T) {
	outbound := vlessOutboundFromOptions(t, json.RawMessage(`{"transport":{"type":"grpc"}}`), map[string]interface{}{"enabled": true})

	if got, ok := outbound["flow"]; ok {
		t.Fatalf("flow was not stripped: %v", got)
	}
}

func TestJsonOutboundsStripVlessVisionFlowWithoutTLS(t *testing.T) {
	outbound := vlessOutboundFromOptions(t, json.RawMessage(`{"transport":{}}`), nil)

	if got, ok := outbound["flow"]; ok {
		t.Fatalf("flow was not stripped: %v", got)
	}
}

func vlessOutboundFromOptions(t *testing.T, options json.RawMessage, tls map[string]interface{}) map[string]interface{} {
	t.Helper()

	inbound := &model.Inbound{
		TlsId:   0,
		Options: options,
		Addrs:   json.RawMessage(`[]`),
		OutJson: json.RawMessage(`{"type":"vless","tag":"vless-test","server":"example.com","server_port":443,"transport":{}}`),
	}
	if tls != nil {
		outbound := map[string]interface{}{
			"type":        "vless",
			"tag":         "vless-test",
			"server":      "example.com",
			"server_port": float64(443),
			"tls":         tls,
			"transport":   map[string]interface{}{},
		}
		outJson, err := json.Marshal(outbound)
		if err != nil {
			t.Fatal(err)
		}
		inbound.TlsId = 1
		inbound.OutJson = outJson
	}

	outbounds, _, err := (&JsonService{}).getOutbounds(
		json.RawMessage(`{"vless":{"uuid":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}}`),
		[]*model.Inbound{inbound},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(*outbounds) != 1 {
		t.Fatalf("got %d outbounds", len(*outbounds))
	}
	return (*outbounds)[0]
}
