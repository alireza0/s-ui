package nodes

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestProvisionFirewallOwnership(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("node firewall management runs on Linux")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("firewall ownership checks require python3")
	}
	cmd := exec.CommandContext(t.Context(), python, "-B", "test_provision_firewall.py")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("firewall ownership checks failed: %s %v", output, err)
	}
}
