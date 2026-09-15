package core

import (
	"net"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

type testOutbound struct {
	adapter.Outbound
	tag string
}

func (o *testOutbound) Tag() string { return o.tag }

func trackConn(t *testing.T, tracker *SessionTracker, inbound, user string) (net.Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	metadata := adapter.InboundContext{
		Inbound:     inbound,
		User:        user,
		Source:      M.ParseSocksaddr("10.0.0.1:1234"),
		Destination: M.ParseSocksaddr("example.com:443"),
	}
	tracked := tracker.RoutedConnection(t.Context(), server, metadata, nil, &testOutbound{tag: "direct"})
	return client, tracked
}

func TestSessionTrackerCountsAndForgets(t *testing.T) {
	tracker := NewSessionTracker()
	client, tracked := trackConn(t, tracker, "in", "alice")
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte("hello"))
	}()
	buffer := make([]byte, 5)
	if _, err := tracked.Read(buffer); err != nil {
		t.Fatal(err)
	}

	sessions := tracker.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].User != "alice" || sessions[0].Inbound != "in" || sessions[0].Outbound != "direct" {
		t.Fatalf("unexpected session: %+v", sessions[0])
	}
	// The name the client asked for, not just the address it resolves to.
	if sessions[0].Domain != "example.com" {
		t.Fatalf("expected the destination domain, got %q", sessions[0].Domain)
	}
	if sessions[0].Upload != 5 {
		t.Fatalf("expected 5 bytes uploaded, got %d", sessions[0].Upload)
	}

	// The same bytes must reach the aggregate counters, which is what the stats
	// cron drains.
	stats := *tracker.GetStats()
	got := map[string]int64{}
	for _, stat := range stats {
		got[stat.Resource+"/"+stat.Tag] = stat.Traffic
	}
	for _, key := range []string{"inbound/in", "outbound/direct", "user/alice"} {
		if got[key] != 5 {
			t.Fatalf("expected 5 bytes for %s, got %d", key, got[key])
		}
	}

	// Draining is destructive.
	if stats := *tracker.GetStats(); len(stats) != 0 {
		t.Fatalf("expected no traffic on second drain, got %v", stats)
	}

	if err := tracked.Close(); err != nil {
		t.Fatal(err)
	}
	if sessions := tracker.Sessions(); len(sessions) != 0 {
		t.Fatalf("expected session to be gone after close, got %d", len(sessions))
	}
}

// Traffic of a session that ends mid-cycle still has to be billed.
func TestSessionTrackerKeepsTrafficOfClosedSession(t *testing.T) {
	tracker := NewSessionTracker()
	client, tracked := trackConn(t, tracker, "in", "alice")
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte("hi"))
	}()
	buffer := make([]byte, 2)
	if _, err := tracked.Read(buffer); err != nil {
		t.Fatal(err)
	}
	tracked.Close()

	var total int64
	for _, stat := range *tracker.GetStats() {
		if stat.Resource == "user" && stat.Tag == "alice" {
			total += stat.Traffic
		}
	}
	if total != 2 {
		t.Fatalf("expected 2 bytes billed to alice, got %d", total)
	}
}

func TestSessionTrackerCloseByInboundUsers(t *testing.T) {
	tracker := NewSessionTracker()
	aliceClient, aliceConn := trackConn(t, tracker, "in", "alice")
	defer aliceClient.Close()
	bobClient, bobConn := trackConn(t, tracker, "in", "bob")
	defer bobClient.Close()
	otherClient, otherConn := trackConn(t, tracker, "other", "alice")
	defer otherClient.Close()
	defer otherConn.Close()

	closed := tracker.CloseByInboundUsers("in", map[string]struct{}{"bob": {}})
	if closed != 1 {
		t.Fatalf("expected 1 closed session, got %d", closed)
	}
	if _, err := aliceConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected alice's connection to be closed")
	}

	sessions := tracker.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 remaining sessions, got %d", len(sessions))
	}
	bobConn.Close()

	if tracker.CloseByInbound("other") != 1 {
		t.Fatal("expected the other inbound's session to be closed")
	}
}

func TestSessionTrackerCloseByUser(t *testing.T) {
	tracker := NewSessionTracker()
	aliceOne, aliceConnOne := trackConn(t, tracker, "in", "alice")
	defer aliceOne.Close()
	aliceTwo, _ := trackConn(t, tracker, "other", "alice")
	defer aliceTwo.Close()
	bobClient, bobConn := trackConn(t, tracker, "in", "bob")
	defer bobClient.Close()
	defer bobConn.Close()

	// Every inbound, not just the one the client happens to be listed under.
	if closed := tracker.CloseByUser("alice"); closed != 2 {
		t.Fatalf("expected 2 closed sessions, got %d", closed)
	}
	if _, err := aliceConnOne.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected alice's connection to be closed")
	}
	if sessions := tracker.Sessions(); len(sessions) != 1 || sessions[0].User != "bob" {
		t.Fatalf("expected only bob to be left, got %+v", sessions)
	}
	if tracker.CloseByUser("alice") != 0 {
		t.Fatal("expected no sessions left for alice")
	}
}
