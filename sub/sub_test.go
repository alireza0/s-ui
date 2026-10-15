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
		{"empty string", "", nil},
		{"whitespace only", "   \n\t  \r\n   ", nil},
		{"comment", "# 424242", nil},
		{
			"extra spaces",
			"\n\tannounce :  42! \n\tannounce-url: t.me/4242\n\t",
			[]customHeader{{"announce", "42!"}, {"announce-url", "t.me/4242"}},
		},
		{
			"values keep their colons",
			"Profile-Web-Page-Url: https://sub.example.com:8443/login?redirect=https://a.com",
			[]customHeader{{"Profile-Web-Page-Url", "https://sub.example.com:8443/login?redirect=https://a.com"}},
		},
		{
			"crlf line endings and comments",
			"# header 1\r\nX-Custom: value1\r\n// comment 2\r\nX-Other: value2\r\n",
			[]customHeader{{"X-Custom", "value1"}, {"X-Other", "value2"}},
		},
		{
			"malformed lines are skipped",
			"random text without colon\n: missing key\nempty value: \n:\nvalid-key: ok\n",
			[]customHeader{{"valid-key", "ok"}},
		},
		{
			// A name an HTTP header cannot carry would go out malformed.
			"invalid names are skipped",
			"Bad Name: x\nBad(Name): x\nGood-Name: x\n",
			[]customHeader{{"Good-Name", "x"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseCustomHeaders(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
