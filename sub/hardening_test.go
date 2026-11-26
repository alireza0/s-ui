package sub

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/util"
)

// Every one of these used to be a bare type assertion on data that comes
// straight out of the database, so a single odd config took the whole
// subscription endpoint down for every client on the panel -- not just the one
// whose config was odd.
func TestAsBoolNeverPanics(t *testing.T) {
	testCases := []struct {
		name string
		in   interface{}
		want bool
	}{
		{"missing key", nil, false},
		{"real true", true, true},
		{"real false", false, false},
		{"string from an older schema", "true", false},
		{"number", float64(1), false},
		{"object", map[string]interface{}{}, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := asBool(tc.in); got != tc.want {
				t.Errorf("asBool(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// addClientInfo rewrites the remark of a link. A vmess payload with no "ps" is
// perfectly valid JSON, and it used to panic here.
func TestAddClientInfoOnMalformedVmess(t *testing.T) {
	s := &LinkService{}

	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"no ps at all", map[string]interface{}{"add": "192.0.2.10", "port": "443"}},
		{"ps is null", map[string]interface{}{"ps": nil}},
		{"ps is a number", map[string]interface{}{"ps": 42}},
		{"ps is an object", map[string]interface{}{"ps": map[string]interface{}{}}},
		{"empty object", map[string]interface{}{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			uri := "vmess://" + util.ByteToB64Str(raw)

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on a valid vmess payload: %v", r)
				}
			}()
			if got := s.addClientInfo(uri, " info"); got == "" {
				t.Error("returned an empty link")
			}
		})
	}
}

// Garbage that is not even a vmess payload has to come back unchanged rather
// than bringing the response down.
func TestAddClientInfoOnGarbage(t *testing.T) {
	s := &LinkService{}
	for _, uri := range []string{
		"vmess://not-base64!!",
		"vmess://" + util.ByteToB64Str([]byte("not json")),
		"vmess://",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on %q: %v", uri, r)
				}
			}()
			_ = s.addClientInfo(uri, " info")
		}()
	}
}
