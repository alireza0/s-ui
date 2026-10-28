package util

import (
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// Regression: the Profile-Title header was built from the client's
// remark (or name) verbatim. A control character in either made Go's
// net/http refuse to write the subscription response at all, breaking
// that client's link/clash/json subscriptions until the remark was
// edited. GetHeaders must strip control characters from the title.
func TestGetHeadersStripsControlCharsFromTitle(t *testing.T) {
	client := &model.Client{Name: "alice", Remark: "home\r\nInjected: 1"}
	headers := GetHeaders(client, 24)
	if len(headers) != 3 {
		t.Fatalf("got %d headers, want 3", len(headers))
	}
	if strings.ContainsAny(headers[2], "\r\n") {
		t.Fatalf("title still contains CR/LF: %q", headers[2])
	}
	if headers[2] != "homeInjected: 1" {
		t.Fatalf("title = %q, want %q", headers[2], "homeInjected: 1")
	}

	// Falls back to the name when no remark is set, sanitized the same way.
	client.Remark = ""
	client.Name = "bo\tb"
	headers = GetHeaders(client, 24)
	if headers[2] != "bob" {
		t.Fatalf("title = %q, want %q", headers[2], "bob")
	}

	// Ordinary titles, including non-ASCII, pass through untouched.
	client.Name = "alice"
	client.Remark = "家庭宽带"
	headers = GetHeaders(client, 24)
	if headers[2] != "家庭宽带" {
		t.Fatalf("title = %q, want %q", headers[2], "家庭宽带")
	}
}
