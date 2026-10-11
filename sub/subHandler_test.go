package sub

import (
	"reflect"
	"testing"
)

func TestParseCustomHeaders(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		want []customHeader
	}{
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
		{
			name: "whitespace only",
			in:   "   \n\t  \r\n   ",
			want: nil,
		},
		{
			name: "comments test",
			in:   `# 424242`,
			want: nil,
		},
		{
			name: "valid headers with extra spaces",
			in: `
			announce :  42! 
			announce-url: t.me/4242
			`,
			want: []customHeader{
				{key: "announce", value: "42!"},
				{key: "announce-url", value: "t.me/4242"},
			},
		},
		{
			name: "urls containing colons and ports are preserved",
			in:   "Profile-Web-Page-Url: https://sub.example.com:8443/login?redirect=https://a.com",
			want: []customHeader{
				{key: "Profile-Web-Page-Url", value: "https://sub.example.com:8443/login?redirect=https://a.com"},
			},
		},
		{
			name: "windows crlf line endings and inline comments",
			in:   "# header 1\r\nX-Custom: value1\r\n// comment 2\r\nX-Other: value2\r\n",
			want: []customHeader{
				{key: "X-Custom", value: "value1"},
				{key: "X-Other", value: "value2"},
			},
		},
		{
			name: "malformed lines are safely skipped without panic",
			in: `
random text without colon
: missing key
empty value: 
:
valid-key: ok
`,
			want: []customHeader{
				{key: "valid-key", value: "ok"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCustomHeaders(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCustomHeaders() mismatch for %q:\n got:  %+v\n want: %+v", tc.name, got, tc.want)
			}
		})
	}
}
