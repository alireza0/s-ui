package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// fakeEndpointCore answers probes from a per-tag script and records rebuilds.
type fakeEndpointCore struct {
	running bool
	healthy map[string]bool // tags not listed fail
	missing map[string]bool // tags the core does not know
	// Tags probed while the core is mid-restart. Kept apart from running,
	// which the job checks before probing.
	restarting map[string]bool
	removed    []string
	added      []string
	addError   error
}

func (f *fakeEndpointCore) IsRunning() bool { return f.running }

func (f *fakeEndpointCore) CheckOutbound(tag string, link string) core.CheckOutboundResult {
	if f.restarting[tag] {
		return core.CheckOutboundResult{Error: "core not running"}
	}
	if f.missing[tag] {
		return core.CheckOutboundResult{Error: "outbound not found"}
	}
	if f.healthy[tag] {
		return core.CheckOutboundResult{OK: true, Delay: 50}
	}
	return core.CheckOutboundResult{Error: "context deadline exceeded"}
}

func (f *fakeEndpointCore) RemoveEndpoint(tag string) error {
	f.removed = append(f.removed, tag)
	if f.missing[tag] {
		return os.ErrInvalid
	}
	return nil
}

func (f *fakeEndpointCore) AddEndpoint(config []byte) error {
	if f.addError != nil {
		return f.addError
	}
	var c struct {
		Type string `json:"type"`
		Tag  string `json:"tag"`
	}
	if err := json.Unmarshal(config, &c); err != nil {
		return err
	}
	if c.Type != "wireguard" {
		return errors.New("warp endpoint must reach the core as wireguard, got " + c.Type)
	}
	f.added = append(f.added, c.Tag)
	delete(f.missing, c.Tag)
	return nil
}

type healthHarness struct {
	svc      *EndpointHealthService
	core     *fakeEndpointCore
	clock    time.Time
	tags     []string
	lastSave time.Time
}

func newHealthHarness(t *testing.T, enabled bool, tags ...string) *healthHarness {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	if enabled {
		if err := (&SettingService{}).setString("endpointHealth", "true"); err != nil {
			t.Fatal(err)
		}
	}
	h := &healthHarness{
		core:  &fakeEndpointCore{running: true, healthy: map[string]bool{"direct": true}, missing: map[string]bool{}},
		clock: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		tags:  tags,
	}
	h.svc = &EndpointHealthService{
		now:  func() time.Time { return h.clock },
		core: func() endpointCore { return h.core },
		loadEndpoint: func(tag string) (*model.Endpoint, error) {
			return &model.Endpoint{Type: "warp", Tag: tag, Options: json.RawMessage(`{"mtu":1280}`)}, nil
		},
		warpTags: func() ([]string, error) { return h.tags, nil },
		lastSave: func() time.Time { return h.lastSave },
	}
	return h
}

// tick runs the job once and advances the clock by the cron interval.
func (h *healthHarness) tick() {
	h.svc.Run()
	h.clock = h.clock.Add(2 * time.Minute)
}

func (h *healthHarness) ticks(n int) {
	for i := 0; i < n; i++ {
		h.tick()
	}
}

func TestEndpointHealthThreshold(t *testing.T) {
	cases := []struct {
		name     string
		healthy  bool
		ticks    int
		rebuilds int
	}{
		// A working endpoint is never touched.
		{"healthy", true, 10, 0},
		// Two lost probes are noise, not a stuck tunnel.
		{"below threshold", false, endpointFailThreshold - 1, 0},
		{"at threshold", false, endpointFailThreshold, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHealthHarness(t, true, "warp-1")
			h.core.healthy["warp-1"] = tc.healthy
			h.ticks(tc.ticks)
			if len(h.core.added) != tc.rebuilds {
				t.Errorf("rebuilt %d times, want %d", len(h.core.added), tc.rebuilds)
			}
		})
	}
}

// A failure streak that is broken by one good probe starts over.
func TestEndpointHealthRecoveryResetsStreak(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.ticks(endpointFailThreshold - 1)
	h.core.healthy["warp-1"] = true
	h.tick()
	h.core.healthy["warp-1"] = false
	h.ticks(endpointFailThreshold - 1)
	if len(h.core.added) != 0 {
		t.Errorf("rebuilt after an interrupted streak: %v", h.core.added)
	}
}

// The point of the job: only the stuck endpoint is replaced, with the stored
// config, and nothing else in the core is touched.
func TestEndpointHealthRebuildsOnlyTheStuckEndpoint(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1", "warp-2")
	h.core.healthy["warp-2"] = true
	h.ticks(endpointFailThreshold)
	if want := []string{"warp-1"}; !equalTags(h.core.removed, want) || !equalTags(h.core.added, want) {
		t.Errorf("removed %v, added %v, want only warp-1", h.core.removed, h.core.added)
	}
}

// An endpoint that stays broken must not be rebuilt in a loop: once per
// cooldown, and no more than endpointRebuildMax times per window.
func TestEndpointHealthRateLimit(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")

	h.ticks(endpointFailThreshold)
	if len(h.core.added) != 1 {
		t.Errorf("first rebuild: got %d", len(h.core.added))
	}

	// Still failing, but inside the cooldown.
	h.ticks(int(endpointRebuildCooldown/(2*time.Minute)) - 1)
	if len(h.core.added) != 1 {
		t.Errorf("rebuilt again inside the cooldown: %d", len(h.core.added))
	}

	// Hours more of failures stop at the cap, well before the window ends.
	h.ticks(int(6 * time.Hour / (2 * time.Minute)))
	if len(h.core.added) != endpointRebuildMax {
		t.Errorf("rebuilt %d times, want the cap of %d", len(h.core.added), endpointRebuildMax)
	}

	// Once the window has moved past the earlier rebuilds, it may try again.
	h.ticks(int(endpointRebuildWindow / (2 * time.Minute)))
	if len(h.core.added) <= endpointRebuildMax {
		t.Errorf("never retried after the window rolled over: %d", len(h.core.added))
	}
}

// When direct traffic fails too, the server is offline. Rebuilding tunnels
// then would only burn the rebuild budget.
func TestEndpointHealthSkipsWhenServerOffline(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.core.healthy["direct"] = false
	h.ticks(endpointFailThreshold * 3)
	if len(h.core.added) != 0 {
		t.Errorf("rebuilt while direct was failing: %v", h.core.added)
	}
	// The offline rounds are not counted against the endpoint either.
	h.core.healthy["direct"] = true
	h.ticks(endpointFailThreshold - 1)
	if len(h.core.added) != 0 {
		t.Errorf("offline rounds counted as endpoint failures: %v", h.core.added)
	}
}

// A config without an outbound tagged direct still gets checked.
func TestEndpointHealthWithoutDirectOutbound(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.core.missing["direct"] = true
	h.ticks(endpointFailThreshold)
	if len(h.core.added) != 1 {
		t.Errorf("rebuilt %d times, want 1", len(h.core.added))
	}
}

// A core restart during a round must not count as a failure: it replaces every
// endpoint, so a streak from before the restart must not get a fresh endpoint
// rebuilt on its first lost probe.
func TestEndpointHealthCoreRestartResetsStreak(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.core.restarting = map[string]bool{}
	h.ticks(endpointFailThreshold - 1)
	h.core.restarting["warp-1"] = true
	h.tick()
	h.core.restarting["warp-1"] = false
	h.ticks(endpointFailThreshold - 1)
	if len(h.core.added) != 0 {
		t.Errorf("rebuilt after a core restart: %v", h.core.added)
	}
}

// A rebuild whose add step failed leaves the endpoint missing from the core.
// The next rebuild has to put it back rather than give up on a tag the core no
// longer knows.
func TestEndpointHealthRepairsMissingEndpoint(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.core.addError = errors.New("bind: address in use")
	h.ticks(endpointFailThreshold)
	h.core.missing["warp-1"] = true
	h.core.addError = nil

	h.ticks(int(endpointRebuildCooldown / (2 * time.Minute)))
	if !equalTags(h.core.added, []string{"warp-1"}) {
		t.Errorf("missing endpoint not re-added: %v", h.core.added)
	}
	if h.core.missing["warp-1"] {
		t.Error("endpoint still missing after repair")
	}
}

// restartCoreWithConfig only TryLocks lifecycleMu and gives up when it is held.
// A rebuild that took the lock could therefore make a saved config change
// silently never apply. The rebuild must neither take nor wait for it.
func TestEndpointHealthLeavesLifecycleLockAlone(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	done := make(chan struct{})
	go func() {
		h.ticks(endpointFailThreshold)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("health check blocked on lifecycleMu")
	}
	if len(h.core.added) != 1 {
		t.Errorf("rebuilt %d times while the lock was held elsewhere, want 1", len(h.core.added))
	}
}

// A panel edit swaps the live endpoint before its database write commits; a
// rebuild from the database in that gap would restore the old config.
func TestEndpointHealthWaitsAfterSave(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	// Probes run every two minutes from the start clock; the third failing one
	// comes half a minute after this save.
	thirdProbe := h.clock.Add((endpointFailThreshold - 1) * 2 * time.Minute)
	h.lastSave = thirdProbe.Add(-30 * time.Second)
	h.ticks(endpointFailThreshold)
	if len(h.core.added) != 0 {
		t.Errorf("rebuilt right after a save: %v", h.core.added)
	}
	// Still failing at the next probe, past the quiet period: now it may rebuild.
	h.tick()
	if len(h.core.added) != 1 {
		t.Errorf("rebuilt %d times after the quiet period, want 1", len(h.core.added))
	}
}

// Probes now run in parallel; each result must still land on its own endpoint.
func TestEndpointHealthProbesAllEndpoints(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1", "warp-2", "warp-3")
	h.ticks(endpointFailThreshold)
	if want := []string{"warp-1", "warp-2", "warp-3"}; !equalTags(h.core.added, want) {
		t.Errorf("rebuilt %v, want %v", h.core.added, want)
	}
}

func TestEndpointHealthIdle(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		running bool
	}{
		// Off by default: a rebuild drops live connections.
		{"disabled", false, true},
		{"core stopped", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHealthHarness(t, tc.enabled, "warp-1")
			h.core.running = tc.running
			h.ticks(endpointFailThreshold * 2)
			if len(h.core.removed) != 0 {
				t.Errorf("acted while %s: %v", tc.name, h.core.removed)
			}
		})
	}
}

// A deleted endpoint's history must not follow a new endpoint that reuses the tag.
func TestEndpointHealthForgetsDeletedEndpoints(t *testing.T) {
	h := newHealthHarness(t, true, "warp-1")
	h.ticks(endpointFailThreshold - 1)
	h.tags = nil
	h.tick()
	h.tags = []string{"warp-1"}
	h.ticks(endpointFailThreshold - 1)
	if len(h.core.added) != 0 {
		t.Errorf("old failures carried over to a new endpoint: %v", h.core.added)
	}
}

func equalTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
