package core

import (
	"strings"
	"sync"
	"testing"
)

// No inbounds: a real Box builds and starts, binds no ports, cycles cheaply.
const emptyCoreConfig = `{"log":{"level":"error"}}`

// The shape of the running panel: the watchdog and the save handler start and
// stop the core while the dashboard poll reads IsRunning and GetInstance. Core
// used to have no mutex, so this tripped -race on both fields. Without -race it
// only proves the accessors do not panic.
func TestCoreConcurrentLifecycle(t *testing.T) {
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				// Never isRunning true alongside a nil instance.
				if box, err := c.running(); err == nil && box == nil {
					t.Error("running() returned a nil box with no error")
					return
				}
				_ = c.IsRunning()
				_ = c.GetInstance()
			}
		}()
	}

	for i := 0; i < 10; i++ {
		if err := c.Start([]byte(emptyCoreConfig)); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("start %d: %v", i, err)
		}
		if err := c.Stop(); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("stop %d: %v", i, err)
		}
	}

	close(stop)
	wg.Wait()
}

// The accessors must agree across a full cycle, and a second Stop must be
// harmless: three separate paths reach it.
func TestCoreStartStopState(t *testing.T) {
	c := NewCore()

	if c.IsRunning() {
		t.Error("a fresh core reports itself running")
	}
	if c.GetInstance() != nil {
		t.Error("a fresh core hands out an instance")
	}
	if _, err := c.running(); err == nil {
		t.Error("running() succeeded on a stopped core")
	}

	if err := c.Start([]byte(emptyCoreConfig)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !c.IsRunning() {
		t.Error("core does not report itself running after start")
	}
	box, err := c.running()
	if err != nil {
		t.Fatalf("running() after start: %v", err)
	}
	if box != c.GetInstance() {
		t.Error("running() and GetInstance() disagree")
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if c.IsRunning() {
		t.Error("core still reports itself running after stop")
	}
	if c.GetInstance() != nil {
		t.Error("instance survived stop")
	}
	if err := c.Stop(); err != nil {
		t.Errorf("second stop should be a no-op, got %v", err)
	}
}

// The unmarshal error was logged and ignored, so the box was built from a
// zero-value option set: healthy, no inbounds, and IsRunning true so the
// watchdog never retried.
func TestStartRejectsMalformedConfig(t *testing.T) {
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })

	err := c.Start([]byte(`{"log": this is not json}`))
	if err == nil {
		t.Fatal("Start accepted a malformed config")
	}
	if !strings.Contains(err.Error(), "unmarshal config") {
		t.Errorf("expected an unmarshal error, got %v", err)
	}
	if c.IsRunning() {
		t.Error("core reports itself running after a rejected config")
	}
	if c.GetInstance() != nil {
		t.Error("a rejected config left an instance behind")
	}
}

// A config that parses but cannot start must not publish a half-built box.
func TestFailedStartLeavesNoInstance(t *testing.T) {
	c := NewCore()
	t.Cleanup(func() { _ = c.Stop() })

	// An unparseable listen address fails while the box is being built.
	err := c.Start([]byte(`{"inbounds":[{"type":"mixed","tag":"in","listen":"not-an-address","listen_port":1080}]}`))
	if err == nil {
		t.Skip("this config built successfully; the failure path needs another trigger")
	}
	if c.IsRunning() {
		t.Error("core reports itself running after a failed start")
	}
	if c.GetInstance() != nil {
		t.Error("a failed start published an instance")
	}
	if _, err := c.running(); err == nil {
		t.Error("running() succeeded after a failed start")
	}
}

// Core's own mutex without starting a Box, so it hammers the lock far harder
// than the tests above can. The invariant is the one the lock-free struct
// broke: no caller may observe isRunning true together with a nil instance.
func TestCoreLockConcurrency(t *testing.T) {
	c := NewCore()

	// Two groups: the readers loop until the channel closes, so they can only
	// be told to stop once the writers have finished. One group deadlocks.
	var readers, writers sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if box, err := c.running(); err == nil && box == nil {
					t.Error("running() returned a nil box with no error")
					return
				}
				if c.IsRunning() && c.GetInstance() == nil {
					t.Error("core reported running with no instance")
					return
				}
			}
		}()
	}

	for i := 0; i < 4; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for j := 0; j < 500; j++ {
				// Rejected before any box is built, so no sing-box code runs.
				_ = c.Start([]byte(`{"log": not json}`))
				_ = c.Stop()
			}
		}()
	}

	writers.Wait()
	close(stop)
	readers.Wait()
}
