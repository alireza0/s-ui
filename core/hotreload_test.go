package core

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func portBound(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprint("127.0.0.1:", port))
	if err != nil {
		return true
	}
	l.Close()
	return false
}

// sing-box v1.14.3 dropped hot add and remove from its managers; the panel
// edits a running core, so core/manager keeps both. An inbound added to a
// running box must listen, and removing it must free the port for the next.
func TestHotInboundAddRemove(t *testing.T) {
	if raceEnabled {
		t.Skip("starting a Box trips sing-box's own race in route.NetworkManager; see race_on_test.go")
	}
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })
	if err := c.Start([]byte(emptyCoreConfig)); err != nil {
		t.Fatalf("start: %v", err)
	}

	port := freePort(t)
	inbound := []byte(fmt.Sprintf(`{"type":"mixed","tag":"in","listen":"127.0.0.1","listen_port":%d}`, port))
	for i := 0; i < 3; i++ {
		if err := c.AddInbound(inbound); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
		if !portBound(port) {
			t.Fatalf("add %d: inbound is not listening", i)
		}
		if err := c.AddInbound(inbound); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("add %d: duplicate tag accepted: %v", i, err)
		}
		if err := c.RemoveInbound("in"); err != nil {
			t.Fatalf("remove %d: %v", i, err)
		}
		if portBound(port) {
			t.Fatalf("remove %d: port still bound", i)
		}
	}
	if err := c.RemoveInbound("in"); err == nil {
		t.Error("removing a missing inbound succeeded")
	}
}

// An inbound present at boot is closed by Remove the same way as a hot one,
// and Stop afterwards must not close it a second time.
func TestBootInboundRemoveThenStop(t *testing.T) {
	if raceEnabled {
		t.Skip("starting a Box trips sing-box's own race in route.NetworkManager; see race_on_test.go")
	}
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })
	port := freePort(t)
	config := fmt.Sprintf(`{"log":{"level":"error"},"inbounds":[{"type":"mixed","tag":"in","listen":"127.0.0.1","listen_port":%d}]}`, port)
	if err := c.Start([]byte(config)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !portBound(port) {
		t.Fatal("boot inbound is not listening")
	}
	if err := c.RemoveInbound("in"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if portBound(port) {
		t.Fatal("port still bound after remove")
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// An outbound another one dials through cannot be removed; once the dependent
// is gone it can, and the tag is free to be added again.
func TestHotOutboundDependency(t *testing.T) {
	if raceEnabled {
		t.Skip("starting a Box trips sing-box's own race in route.NetworkManager; see race_on_test.go")
	}
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })
	if err := c.Start([]byte(emptyCoreConfig)); err != nil {
		t.Fatalf("start: %v", err)
	}
	base := []byte(`{"type":"direct","tag":"base"}`)
	if err := c.AddOutbound(base); err != nil {
		t.Fatalf("add base: %v", err)
	}
	if err := c.AddOutbound([]byte(`{"type":"selector","tag":"sel","outbounds":["base"]}`)); err != nil {
		t.Fatalf("add selector: %v", err)
	}
	if err := c.AddOutbound([]byte(`{"type":"socks","tag":"via","server":"127.0.0.1","server_port":1,"detour":"base"}`)); err != nil {
		t.Fatalf("add detour: %v", err)
	}
	if err := c.RemoveOutbound("base"); err == nil || !strings.Contains(err.Error(), "depended by") {
		t.Fatalf("removed an outbound still depended on: %v", err)
	}
	if err := c.RemoveOutbound("via"); err != nil {
		t.Fatalf("remove detour: %v", err)
	}
	if err := c.RemoveOutbound("sel"); err != nil {
		t.Fatalf("remove selector: %v", err)
	}
	if err := c.RemoveOutbound("base"); err != nil {
		t.Fatalf("remove base: %v", err)
	}
	if err := c.AddOutbound(base); err != nil {
		t.Fatalf("re-add base: %v", err)
	}
}
