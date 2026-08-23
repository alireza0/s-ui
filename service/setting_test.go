package service

import "testing"

func TestNormalizeSettingValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "certificate path",
			value: " \t/root/certificate files/private key.pem \r\n",
			want:  "/root/certificate files/private key.pem",
		},
		{
			name:  "URL",
			value: "  https://example.com/panel%20path/  ",
			want:  "https://example.com/panel%20path/",
		},
		{
			name:  "web path",
			value: "  /panel path/  ",
			want:  "/panel path/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeSettingValue(test.value); got != test.want {
				t.Errorf("normalizeSettingValue() = %q, want %q", got, test.want)
			}
		})
	}
}
