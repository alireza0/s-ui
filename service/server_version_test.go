package service

import (
	"strings"
	"testing"
)

// The dashboard shows the linked sing-box version next to the panel's. #1338
func TestSingboxVersionFromBuildInfo(t *testing.T) {
	if singboxVersion == "" || strings.HasPrefix(singboxVersion, "v") {
		t.Fatalf("singboxVersion = %q, want the module version without the v prefix", singboxVersion)
	}
	if got := (&ServerService{}).GetSystemInfo()["coreVersion"]; got != singboxVersion {
		t.Fatalf("coreVersion = %v, want %s", got, singboxVersion)
	}
}
