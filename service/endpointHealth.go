package service

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
)

// A WARP endpoint can stop passing traffic while sing-box still reports it as
// running: the peer address is resolved once when the endpoint starts and the
// tunnel keeps using that address and source port, so a path that goes bad
// stays bad. The core watchdog only notices a stopped core, so until now the
// only cure was restarting the whole core, which drops every user. This job
// probes each WARP endpoint and rebuilds just the stuck one -- the same
// remove/add that editing it in the panel already does.
//
// It never restarts the core or the panel. Rebuilds are rate limited per
// endpoint, so an endpoint that stays broken costs a few rebuilds a day, not a
// loop.
const (
	// Consecutive failed probes before an endpoint counts as stuck. One lost
	// probe is noise; the job runs every two minutes, so three is six minutes.
	endpointFailThreshold = 3
	// Minimum time between two rebuilds of the same endpoint.
	endpointRebuildCooldown = 20 * time.Minute
	// At most endpointRebuildMax rebuilds per endpoint in any endpointRebuildWindow.
	endpointRebuildWindow = 24 * time.Hour
	endpointRebuildMax    = 3

	// Probed alongside the endpoints. When it fails as well the server itself
	// is offline, and rebuilding tunnels would not help.
	endpointControlTag = "direct"

	// A panel edit swaps the live endpoint before its database write commits.
	// Rebuilding from the database inside that gap would put the old config
	// back, so rebuilds wait this long after any save.
	endpointSaveQuiet = time.Minute
)

// endpointCore is the slice of core.Core the health check uses, so tests can
// stand in for a running core.
type endpointCore interface {
	IsRunning() bool
	CheckOutbound(tag string, link string) core.CheckOutboundResult
	RemoveEndpoint(tag string) error
	AddEndpoint(config []byte) error
}

type endpointState struct {
	fails    int
	rebuilds []time.Time
	// Set once the rebuild cap is reached, so the operator gets one warning
	// instead of one every two minutes.
	capWarned bool
}

type EndpointHealthService struct {
	SettingService

	mu     sync.Mutex
	states map[string]*endpointState

	// Overridable in tests.
	now          func() time.Time
	core         func() endpointCore
	loadEndpoint func(tag string) (*model.Endpoint, error)
	warpTags     func() ([]string, error)
	lastSave     func() time.Time
}

func NewEndpointHealthService() *EndpointHealthService {
	return &EndpointHealthService{}
}

func (s *EndpointHealthService) init() {
	if s.states == nil {
		s.states = map[string]*endpointState{}
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.core == nil {
		s.core = func() endpointCore {
			// A nil *core.Core in an interface is not a nil interface.
			if corePtr == nil {
				return nil
			}
			return corePtr
		}
	}
	if s.loadEndpoint == nil {
		s.loadEndpoint = func(tag string) (*model.Endpoint, error) {
			var endpoint model.Endpoint
			err := database.GetDB().Where("tag = ?", tag).First(&endpoint).Error
			return &endpoint, err
		}
	}
	if s.warpTags == nil {
		s.warpTags = func() ([]string, error) {
			var tags []string
			err := database.GetDB().Model(model.Endpoint{}).Where("type = ?", "warp").Pluck("tag", &tags).Error
			return tags, err
		}
	}
	if s.lastSave == nil {
		// Read the same way CheckChanges reads it.
		s.lastSave = func() time.Time { return time.Unix(LastUpdate, 0) }
	}
}

// Run probes every WARP endpoint once and rebuilds the ones that have failed
// endpointFailThreshold times in a row.
func (s *EndpointHealthService) Run() {
	enabled, err := s.GetEndpointHealth()
	if err != nil || !enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()

	c := s.core()
	if c == nil || !c.IsRunning() {
		return
	}

	tags, err := s.warpTags()
	if err != nil {
		logger.Warning("endpoint health: unable to list warp endpoints: ", err)
		return
	}
	// Forget endpoints that were deleted, so a new one reusing the tag starts
	// clean. Done first: with no endpoints left, the round stops here.
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		seen[tag] = true
	}
	for tag := range s.states {
		if !seen[tag] {
			delete(s.states, tag)
		}
	}
	if len(tags) == 0 {
		return
	}

	// An empty link makes sing-box use its default test URL.
	if control := c.CheckOutbound(endpointControlTag, ""); !control.OK && control.Error != "outbound not found" {
		logger.Debug("endpoint health: direct probe failed, skipping round: ", control.Error)
		return
	}

	// Probed in parallel: a dead endpoint takes the full 15-second test
	// timeout, and a handful of them in a row would overrun the two-minute
	// interval.
	results := make([]core.CheckOutboundResult, len(tags))
	var wg sync.WaitGroup
	for i, tag := range tags {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.CheckOutbound(tag, "")
		}()
	}
	wg.Wait()

	for i, tag := range tags {
		s.check(c, tag, results[i])
	}
}

func (s *EndpointHealthService) check(c endpointCore, tag string, result core.CheckOutboundResult) {
	state := s.states[tag]
	if state == nil {
		state = &endpointState{}
		s.states[tag] = state
	}

	// The core restarted during the round. A restart brings up every endpoint
	// fresh, so the old streak no longer describes the one now running.
	if result.Error == "core not running" {
		state.fails = 0
		return
	}
	// "outbound not found" counts as a failure too: it is what a rebuild that
	// removed the endpoint but could not add it back leaves behind, and the
	// next rebuild repairs it.
	if result.OK {
		state.fails = 0
		return
	}
	state.fails++
	logger.Debug("endpoint health: ", tag, " probe failed (", state.fails, "): ", result.Error)
	if state.fails < endpointFailThreshold {
		return
	}

	now := s.now()
	state.rebuilds = recentRebuilds(state.rebuilds, now)
	if n := len(state.rebuilds); n > 0 && now.Sub(state.rebuilds[n-1]) < endpointRebuildCooldown {
		return
	}
	if len(state.rebuilds) >= endpointRebuildMax {
		if !state.capWarned {
			state.capWarned = true
			logger.Warning("endpoint health: ", tag, " is still failing after ", endpointRebuildMax,
				" rebuilds in ", endpointRebuildWindow, "; leaving it alone until the window allows another: ", result.Error)
		}
		return
	}
	if now.Sub(s.lastSave()) < endpointSaveQuiet {
		return
	}

	// Counted before the attempt, so a rebuild that fails still uses up its slot.
	state.rebuilds = append(state.rebuilds, now)
	state.fails = 0
	state.capWarned = false
	logger.Warning("endpoint health: ", tag, " failed ", endpointFailThreshold, " probes in a row (",
		result.Error, "), rebuilding it")
	if err := s.rebuild(c, tag); err != nil {
		logger.Error("endpoint health: rebuilding ", tag, " failed: ", err)
	}
}

// rebuild replaces a running endpoint with a fresh instance built from the
// stored config, without touching the rest of the core.
//
// It deliberately does not take lifecycleMu. restartCoreWithConfig only
// TryLocks it and returns without restarting when it is held, so holding it
// here could silently drop a saved config change. Each core call takes the
// core's own lock; if a restart lands between the remove and the add, the new
// box already has the endpoint and the add fails as a duplicate, harmlessly.
func (s *EndpointHealthService) rebuild(c endpointCore, tag string) error {
	endpoint, err := s.loadEndpoint(tag)
	if err != nil {
		return err
	}
	config, err := endpoint.MarshalJSON()
	if err != nil {
		return err
	}
	if err := c.RemoveEndpoint(tag); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return c.AddEndpoint(config)
}

// recentRebuilds drops rebuild times that have left the rate-limit window.
func recentRebuilds(times []time.Time, now time.Time) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if now.Sub(t) < endpointRebuildWindow {
			kept = append(kept, t)
		}
	}
	return kept
}
