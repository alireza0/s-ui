package protocol

import (
	"encoding/json"
	"testing"
)

func TestVlessVisionFlowAllowed(t *testing.T) {
	tests := []struct {
		name      string
		hasTLS    bool
		transport interface{}
		want      bool
	}{
		{name: "no tls", hasTLS: false, want: false},
		{name: "no transport", hasTLS: true, want: true},
		{name: "empty transport", hasTLS: true, transport: map[string]interface{}{}, want: true},
		{name: "tcp transport", hasTLS: true, transport: map[string]interface{}{"type": "tcp"}, want: true},
		{name: "grpc transport", hasTLS: true, transport: map[string]interface{}{"type": "grpc"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := VlessVisionFlowAllowed(test.hasTLS, test.transport); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestVlessVisionFlowAllowedFromOptions(t *testing.T) {
	tests := []struct {
		name    string
		hasTLS  bool
		options json.RawMessage
		want    bool
	}{
		{name: "no tls", hasTLS: false, options: json.RawMessage(`{"transport":{"type":"tcp"}}`), want: false},
		{name: "tcp transport", hasTLS: true, options: json.RawMessage(`{"transport":{"type":"tcp"}}`), want: true},
		{name: "grpc transport", hasTLS: true, options: json.RawMessage(`{"transport":{"type":"grpc"}}`), want: false},
		{name: "empty transport", hasTLS: true, options: json.RawMessage(`{"transport":{}}`), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := VlessVisionFlowAllowedFromOptions(test.hasTLS, test.options); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
