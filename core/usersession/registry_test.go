package usersession

import (
	"errors"
	"testing"
	"time"
)

type fakeConn struct {
	closed bool
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

// A session with a closer of its own is cut outright.
func TestCloseUsersClosesTrackedSession(t *testing.T) {
	registry := NewRegistry()
	alice := &fakeConn{}
	bob := &fakeConn{}
	registry.Track("10.0.0.1:1000", alice)
	registry.Bind("alice", "10.0.0.1:1000")
	registry.Track("10.0.0.2:1000", bob)
	registry.Bind("bob", "10.0.0.2:1000")

	if cut := registry.CloseUsers(map[string]struct{}{"bob": {}}); cut != 1 {
		t.Fatalf("expected 1 session cut, got %d", cut)
	}
	if !alice.closed {
		t.Fatal("expected alice's session to be closed")
	}
	if bob.closed {
		t.Fatal("expected bob's session to be kept")
	}
}

// Without a closer the session is muted instead: nothing it opens is routed.
func TestCloseUsersMutesUntrackedSession(t *testing.T) {
	registry := NewRegistry()
	registry.Bind("alice", "10.0.0.1:1000")
	registry.Bind("bob", "10.0.0.2:1000")

	registry.CloseUsers(map[string]struct{}{"bob": {}})
	if registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected alice's session to be muted")
	}
	if !registry.Allowed("10.0.0.2:1000") {
		t.Fatal("expected bob's session to stay allowed")
	}
	// An address nobody authenticated from is not affected.
	if !registry.Allowed("10.0.0.3:1000") {
		t.Fatal("expected an unknown address to be allowed")
	}

	// Re-enabling the user lifts the mute without waiting for the session to die.
	registry.CloseUsers(map[string]struct{}{"alice": {}, "bob": {}})
	if !registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected alice's session to be allowed again")
	}
}

func TestRejectReportsClose(t *testing.T) {
	conn := &fakeConn{}
	var reported error
	Reject(conn, func(err error) {
		reported = err
	})
	if !conn.closed {
		t.Fatal("expected the connection to be closed")
	}
	if !errors.Is(reported, ErrRemoved) {
		t.Fatalf("expected ErrRemoved, got %v", reported)
	}
}

func TestUntrackForgetsSession(t *testing.T) {
	registry := NewRegistry()
	registry.Bind("alice", "10.0.0.1:1000")
	registry.CloseUsers(map[string]struct{}{})
	if registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected the session to be muted")
	}
	registry.Untrack("10.0.0.1:1000")
	if !registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected the address to be free after untrack")
	}
	if cut := registry.CloseUsers(map[string]struct{}{}); cut != 0 {
		t.Fatalf("expected nothing left to cut, got %d", cut)
	}
}

// A kick cuts what it can close outright.
func TestKickUserSessionsClosesTrackedSession(t *testing.T) {
	registry := NewRegistry()
	alice := &fakeConn{}
	bob := &fakeConn{}
	registry.Track("10.0.0.1:1000", alice)
	registry.Bind("alice", "10.0.0.1:1000")
	registry.Track("10.0.0.2:1000", bob)
	registry.Bind("bob", "10.0.0.2:1000")

	if kicked := registry.KickUserSessions("alice"); kicked != 1 {
		t.Fatalf("expected 1 session kicked, got %d", kicked)
	}
	if !alice.closed {
		t.Fatal("expected alice's session to be closed")
	}
	if bob.closed {
		t.Fatal("expected bob's session to be kept")
	}
	// Nothing is left muted: the closed session is gone, so the address is free.
	if !registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected alice to be free to connect again")
	}
	if registry.KickUserSessions("") != 0 {
		t.Fatal("expected an empty user name to kick nothing")
	}
}

// A session with no closer (a QUIC one) is muted instead, and the mute lifts
// once that session stops trying, so the user is not locked out.
func TestKickUserSessionsMutesUntilSessionGivesUp(t *testing.T) {
	registry := NewRegistry()
	registry.Bind("alice", "10.0.0.1:1000")

	if kicked := registry.KickUserSessions("alice"); kicked != 1 {
		t.Fatalf("expected 1 session kicked, got %d", kicked)
	}
	// The kicked session keeps trying and keeps being refused.
	if registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected the kicked session to be muted")
	}
	if registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected the kicked session to stay muted while it retries")
	}

	// Once it has been quiet for the window, the next attempt is a new session.
	registry.access.Lock()
	registry.blocked["10.0.0.1:1000"].lastAttempt = time.Now().Add(-2 * kickQuietWindow)
	registry.access.Unlock()
	if !registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected the mute to lift after the session went quiet")
	}
}

// A removal is not a kick: it stays muted however long the client waits.
func TestCloseUsersMuteDoesNotLiftOnQuiet(t *testing.T) {
	registry := NewRegistry()
	registry.Bind("alice", "10.0.0.1:1000")
	registry.CloseUsers(map[string]struct{}{})

	registry.access.Lock()
	registry.blocked["10.0.0.1:1000"].lastAttempt = time.Now().Add(-2 * kickQuietWindow)
	registry.access.Unlock()
	if registry.Allowed("10.0.0.1:1000") {
		t.Fatal("expected a removed user to stay muted")
	}
}
