package core

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/database/model"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Counter struct {
	read  *atomic.Int64
	write *atomic.Int64
}

// Session is one routed connection. The same counters it exposes are also
// wired into the aggregate maps below, so draining stats never has to walk the
// session list and traffic of a session that closed mid-cycle is not lost.
type Session struct {
	ID          uint64
	Inbound     string
	User        string
	Outbound    string
	Network     string
	Source      M.Socksaddr
	Destination M.Socksaddr
	// Domain is the name the connection is really for: what the client asked
	// for, or what sniffing found when the client only sent an address.
	Domain    string
	Rule      string
	CreatedAt time.Time
	Upload    *atomic.Int64
	Download  *atomic.Int64

	closer io.Closer
}

// SessionInfo is the snapshot handed to the API; the live atomics stay inside.
type SessionInfo struct {
	ID          string `json:"id"`
	Inbound     string `json:"inbound,omitempty"`
	User        string `json:"user,omitempty"`
	Outbound    string `json:"outbound,omitempty"`
	Network     string `json:"network,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Rule        string `json:"rule,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	Upload      int64  `json:"up"`
	Download    int64  `json:"down"`
}

var _ adapter.ConnectionTracker = (*SessionTracker)(nil)

// SessionTracker replaces the former StatsTracker/ConnTracker pair: one wrap
// per connection that both counts traffic and keeps the session closable, so a
// user who was just removed from an inbound can actually be cut off.
type SessionTracker struct {
	access    sync.Mutex
	inbounds  map[string]Counter
	outbounds map[string]Counter
	users     map[string]Counter
	sessions  map[uint64]*Session
	// nextID only has to be unique among live sessions, so it is a counter
	// rather than a UUID: this runs on every connection.
	nextID uint64
}

func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		inbounds:  make(map[string]Counter),
		outbounds: make(map[string]Counter),
		users:     make(map[string]Counter),
		sessions:  make(map[uint64]*Session),
	}
}

func (t *SessionTracker) loadOrCreateCounter(obj map[string]Counter, name string) Counter {
	counter, loaded := obj[name]
	if loaded {
		return counter
	}
	counter = Counter{read: &atomic.Int64{}, write: &atomic.Int64{}}
	obj[name] = counter
	return counter
}

// newSession registers the session and returns the counter slices to hand to
// the counting conn: index 0 is the session's own counter, the rest are the
// aggregates GetStats drains.
func (t *SessionTracker) newSession(metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound, network string) (*Session, []*atomic.Int64, []*atomic.Int64) {
	// The router fills RouteRule and RouteOutbound in just before it calls the
	// trackers, so both are reused rather than formatted a second time: a rule's
	// String() is not free and this runs per connection.
	outbound := metadata.RouteOutbound
	if outbound == "" && matchOutbound != nil {
		outbound = matchOutbound.Tag()
	}
	session := &Session{
		Inbound:     metadata.Inbound,
		User:        metadata.User,
		Outbound:    outbound,
		Network:     network,
		Source:      metadata.Source,
		Destination: metadata.Destination,
		CreatedAt:   time.Now(),
		Upload:      &atomic.Int64{},
		Download:    &atomic.Int64{},
	}
	// Same precedence as the clash API: what sniffing found wins, because the
	// destination is only a name when the client sent one.
	if metadata.Domain != "" {
		session.Domain = metadata.Domain
	} else {
		session.Domain = metadata.Destination.Fqdn
	}
	session.Rule = metadata.RouteRule
	if session.Rule == "" && matchedRule != nil {
		session.Rule = matchedRule.String()
	}

	readCounter := []*atomic.Int64{session.Upload}
	writeCounter := []*atomic.Int64{session.Download}

	t.access.Lock()
	defer t.access.Unlock()
	t.nextID++
	session.ID = t.nextID
	if session.Inbound != "" {
		counter := t.loadOrCreateCounter(t.inbounds, session.Inbound)
		readCounter = append(readCounter, counter.read)
		writeCounter = append(writeCounter, counter.write)
	}
	if session.Outbound != "" {
		counter := t.loadOrCreateCounter(t.outbounds, session.Outbound)
		readCounter = append(readCounter, counter.read)
		writeCounter = append(writeCounter, counter.write)
	}
	if session.User != "" {
		counter := t.loadOrCreateCounter(t.users, session.User)
		readCounter = append(readCounter, counter.read)
		writeCounter = append(writeCounter, counter.write)
	}
	t.sessions[session.ID] = session
	return session, readCounter, writeCounter
}

func (t *SessionTracker) leave(session *Session) {
	t.access.Lock()
	defer t.access.Unlock()
	delete(t.sessions, session.ID)
}

func (t *SessionTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	session, readCounter, writeCounter := t.newSession(metadata, matchedRule, matchOutbound, N.NetworkTCP)
	tracked := &sessionConn{
		ExtendedConn: bufio.NewInt64CounterConn(conn, readCounter, writeCounter),
		tracker:      t,
		session:      session,
	}
	session.closer = tracked
	return tracked
}

func (t *SessionTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	session, readCounter, writeCounter := t.newSession(metadata, matchedRule, matchOutbound, N.NetworkUDP)
	tracked := &sessionPacketConn{
		PacketConn: bufio.NewInt64CounterPacketConn(conn, readCounter, nil, writeCounter, nil),
		tracker:    t,
		session:    session,
	}
	session.closer = tracked
	return tracked
}

func (t *SessionTracker) RoutedFlow(ctx context.Context, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) tun.FlowTracker {
	return nil
}

// GetStats drains the aggregate counters. Destructive: every counter is reset,
// so the caller owns the returned traffic.
func (t *SessionTracker) GetStats() *[]model.Stats {
	t.access.Lock()
	defer t.access.Unlock()

	dt := time.Now().Unix()

	// Emit only directions that actually moved traffic; a zero-traffic row would
	// just bloat the stats table without changing any chart bucket.
	s := []model.Stats{}
	appendStat := func(resource, tag string, down, up int64) {
		if down > 0 {
			s = append(s, model.Stats{DateTime: dt, Resource: resource, Tag: tag, Direction: false, Traffic: down})
		}
		if up > 0 {
			s = append(s, model.Stats{DateTime: dt, Resource: resource, Tag: tag, Direction: true, Traffic: up})
		}
	}

	for inbound, counter := range t.inbounds {
		appendStat("inbound", inbound, counter.write.Swap(0), counter.read.Swap(0))
	}
	for outbound, counter := range t.outbounds {
		appendStat("outbound", outbound, counter.write.Swap(0), counter.read.Swap(0))
	}
	for user, counter := range t.users {
		appendStat("user", user, counter.write.Swap(0), counter.read.Swap(0))
	}
	return &s
}

// Sessions returns a snapshot of the live sessions, newest counters included.
func (t *SessionTracker) Sessions() []SessionInfo {
	// Only the list is taken under the lock. Formatting every address of a
	// busy server takes milliseconds, and holding the tracker that long would
	// stall every connection being opened or closed meanwhile; a session's
	// fields do not change after it is created, and its counters are atomic.
	t.access.Lock()
	live := make([]*Session, 0, len(t.sessions))
	for _, session := range t.sessions {
		live = append(live, session)
	}
	t.access.Unlock()

	sessions := make([]SessionInfo, 0, len(live))
	for _, session := range live {
		sessions = append(sessions, SessionInfo{
			ID:          strconv.FormatUint(session.ID, 10),
			Inbound:     session.Inbound,
			User:        session.User,
			Outbound:    session.Outbound,
			Network:     session.Network,
			Source:      session.Source.String(),
			Destination: session.Destination.String(),
			Domain:      session.Domain,
			Rule:        session.Rule,
			CreatedAt:   session.CreatedAt.Unix(),
			Upload:      session.Upload.Load(),
			Download:    session.Download.Load(),
		})
	}
	return sessions
}

// takeSessions removes every session matching the predicate and returns their
// closers. Closing happens outside the lock: a closer runs leave(), which takes
// the same lock.
func (t *SessionTracker) takeSessions(match func(*Session) bool) []io.Closer {
	t.access.Lock()
	var closers []io.Closer
	for id, session := range t.sessions {
		if !match(session) {
			continue
		}
		delete(t.sessions, id)
		if session.closer != nil {
			closers = append(closers, session.closer)
		}
	}
	t.access.Unlock()

	for _, closer := range closers {
		_ = closer.Close()
	}
	return closers
}

// CloseByInbound closes every session of an inbound, for a restart or a config
// change that invalidates all of them.
func (t *SessionTracker) CloseByInbound(inbound string) int {
	return len(t.takeSessions(func(session *Session) bool {
		return session.Inbound == inbound
	}))
}

// CloseByInboundUsers closes the sessions of an inbound whose user is no longer
// in keepUsers, i.e. users just disabled or removed.
func (t *SessionTracker) CloseByInboundUsers(inbound string, keepUsers map[string]struct{}) int {
	return len(t.takeSessions(func(session *Session) bool {
		if session.Inbound != inbound {
			return false
		}
		_, keep := keepUsers[session.User]
		return !keep
	}))
}

// CloseByUser closes every routed connection of a user across all inbounds.
func (t *SessionTracker) CloseByUser(user string) int {
	if user == "" {
		return 0
	}
	return len(t.takeSessions(func(session *Session) bool {
		return session.User == user
	}))
}

type sessionConn struct {
	N.ExtendedConn
	tracker   *SessionTracker
	session   *Session
	closeOnce sync.Once
}

func (c *sessionConn) Close() error {
	c.closeOnce.Do(func() {
		c.tracker.leave(c.session)
	})
	return c.ExtendedConn.Close()
}

func (c *sessionConn) Upstream() any {
	return c.ExtendedConn
}

func (c *sessionConn) ReaderReplaceable() bool {
	return true
}

func (c *sessionConn) WriterReplaceable() bool {
	return true
}

type sessionPacketConn struct {
	N.PacketConn
	tracker   *SessionTracker
	session   *Session
	closeOnce sync.Once
}

func (c *sessionPacketConn) Close() error {
	c.closeOnce.Do(func() {
		c.tracker.leave(c.session)
	})
	return c.PacketConn.Close()
}

func (c *sessionPacketConn) Upstream() any {
	return c.PacketConn
}

func (c *sessionPacketConn) ReaderReplaceable() bool {
	return true
}

func (c *sessionPacketConn) WriterReplaceable() bool {
	return true
}
