package web

import "testing"

func TestExposedListen(t *testing.T) {
	for _, listen := range []string{"", "0.0.0.0", "::"} {
		if !exposedListen(listen) {
			t.Errorf("exposedListen(%q) = false, want true", listen)
		}
	}
	for _, listen := range []string{"127.0.0.1", "::1", "192.168.1.10", "10.0.0.2"} {
		if exposedListen(listen) {
			t.Errorf("exposedListen(%q) = true, want false", listen)
		}
	}
}
