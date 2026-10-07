// Package quicgrace gives the panel a handle on the QUIC sessions of an
// inbound, which sing-quic otherwise keeps to itself.
//
// sing-quic's services listen through a single-use quic-go Transport, and
// closing it "abruptly terminates all existing connections, without sending a
// CONNECTION_CLOSE to the peers" (quic-go's own words). Each listener also has
// a fresh random stateless-reset key, so the listener that replaces it cannot
// reset the old connections either. A client therefore learns that the server
// is gone only from its idle timeout -- 30 seconds for hysteria -- every time
// the inbound is rebuilt or the core restarts (#1278).
//
// The same missing handle is why a QUIC session could only be muted, never
// closed, when its user was disconnected or removed (#1271).
//
// sing-quic hands listening to the TLS config when it implements its exported
// ServerConfig interface. Wrap returns such a config: it listens exactly as
// sing-quic would and keeps the connections it accepts. Closing the listener
// sends each of them a CONNECTION_CLOSE first, so clients reconnect at once,
// and Sessions closes one client's session on demand. No upstream code is
// copied or reached into.
package quicgrace

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"io"
	"net"
	"sync"

	"github.com/alireza0/s-ui/core/usersession"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	qtls "github.com/sagernet/sing-quic"
	M "github.com/sagernet/sing/common/metadata"
	aTLS "github.com/sagernet/sing/common/tls"
)

// Options mirror the sing-quic ListenOptions a service would have passed; the
// ServerConfig hook does not carry them, so the inbound states them here.
type Options struct {
	DisableVersionNegotiationPackets bool
	StatelessReset                   bool
	// Sessions, when set, records every accepted connection so one can be
	// closed by its client address.
	Sessions *Sessions
}

// Wrap returns config with QUIC listening taken over. Everything else is
// config's own.
func Wrap(config aTLS.ServerConfig, options Options) aTLS.ServerConfig {
	if config == nil {
		return nil
	}
	return &serverConfig{ServerConfig: config, options: options}
}

type serverConfig struct {
	aTLS.ServerConfig
	options Options
}

var _ qtls.ServerConfig = (*serverConfig)(nil)

func (c *serverConfig) ConfigureHTTP3() {
	if inner, ok := c.ServerConfig.(qtls.ServerConfig); ok {
		inner.ConfigureHTTP3()
		return
	}
	if std, err := c.STDConfig(); err == nil {
		http3.ConfigureTLSConfig(std)
	}
}

func (c *serverConfig) Listen(conn net.PacketConn, config *quic.Config) (qtls.Listener, error) {
	if inner, ok := c.ServerConfig.(qtls.ServerConfig); ok {
		listener, err := inner.Listen(conn, config)
		if err != nil {
			return nil, err
		}
		return c.track(listener), nil
	}
	transport, std, err := c.transport(conn)
	if err != nil {
		return nil, err
	}
	listener, err := transport.Listen(std, config)
	if err != nil {
		return nil, qtls.WrapError(err)
	}
	return c.track(listener), nil
}

func (c *serverConfig) ListenEarly(conn net.PacketConn, config *quic.Config) (qtls.EarlyListener, error) {
	if inner, ok := c.ServerConfig.(qtls.ServerConfig); ok {
		listener, err := inner.ListenEarly(conn, config)
		if err != nil {
			return nil, err
		}
		return c.track(listener), nil
	}
	transport, std, err := c.transport(conn)
	if err != nil {
		return nil, err
	}
	listener, err := transport.ListenEarly(std, config)
	if err != nil {
		return nil, qtls.WrapError(err)
	}
	return c.track(listener), nil
}

// transport is the one sing-quic's ListenWithOptions builds.
func (c *serverConfig) transport(conn net.PacketConn) (*quic.Transport, *tls.Config, error) {
	std, err := c.STDConfig()
	if err != nil {
		return nil, nil, err
	}
	transport := &quic.Transport{Conn: conn, DisableVersionNegotiationPackets: c.options.DisableVersionNegotiationPackets}
	transport.SetSingleUse(true)
	if c.options.StatelessReset {
		var key quic.StatelessResetKey
		if _, err := rand.Read(key[:]); err != nil {
			return nil, nil, err
		}
		transport.StatelessResetKey = &key
	}
	return transport, std, nil
}

func (c *serverConfig) track(inner acceptor) *listener {
	return &listener{acceptor: inner, sessions: c.options.Sessions, conns: make(map[*quic.Conn]struct{})}
}

type acceptor interface {
	Accept(ctx context.Context) (*quic.Conn, error)
	Close() error
	Addr() net.Addr
}

// listener remembers what it accepted until it is closed or the connection
// ends on its own.
type listener struct {
	acceptor
	sessions *Sessions
	mu       sync.Mutex
	conns    map[*quic.Conn]struct{}
	closed   bool
}

func (l *listener) Accept(ctx context.Context) (*quic.Conn, error) {
	conn, err := l.acceptor.Accept(ctx)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = conn.CloseWithError(0, "")
		return nil, quic.ErrServerClosed
	}
	l.conns[conn] = struct{}{}
	l.mu.Unlock()
	l.sessions.add(conn)
	go func() {
		<-conn.Context().Done()
		l.mu.Lock()
		delete(l.conns, conn)
		l.mu.Unlock()
		l.sessions.remove(conn)
	}()
	return conn, nil
}

// Close sends every accepted connection a CONNECTION_CLOSE (application error
// 0, no reason: nothing for a prober to read), then closes the listener and,
// with it, the transport.
func (l *listener) Close() error {
	l.mu.Lock()
	l.closed = true
	conns := make([]*quic.Conn, 0, len(l.conns))
	for conn := range l.conns {
		conns = append(conns, conn)
	}
	l.mu.Unlock()
	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Go(func() { _ = conn.CloseWithError(0, "") })
	}
	wg.Wait()
	return l.acceptor.Close()
}

// Sessions holds the live QUIC connections of one inbound, across every
// listener it opens.
type Sessions struct {
	mu    sync.Mutex
	conns map[*quic.Conn]struct{}
}

func NewSessions() *Sessions {
	return &Sessions{conns: make(map[*quic.Conn]struct{})}
}

func (s *Sessions) add(conn *quic.Conn) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *Sessions) remove(conn *quic.Conn) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// Closer returns a closer for the session of the client at source, in the
// form sing-quic reports it to the inbound. The address is matched when the
// closer runs, not now: a hysteria2 client with port hopping moves its
// session to a new port every few seconds. Finding none, it reports
// usersession.ErrNotClosed so the registry mutes the address instead.
func (s *Sessions) Closer(source string) io.Closer {
	return &sessionCloser{sessions: s, source: source}
}

type sessionCloser struct {
	sessions *Sessions
	source   string
}

func (c *sessionCloser) Close() error {
	c.sessions.mu.Lock()
	var matched []*quic.Conn
	for conn := range c.sessions.conns {
		if M.SocksaddrFromNet(conn.RemoteAddr()).Unwrap().String() == c.source {
			matched = append(matched, conn)
		}
	}
	c.sessions.mu.Unlock()
	if len(matched) == 0 {
		return usersession.ErrNotClosed
	}
	for _, conn := range matched {
		_ = conn.CloseWithError(0, "")
	}
	return nil
}
