package quicgrace

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/alireza0/s-ui/core/usersession"

	"github.com/sagernet/quic-go"
	sbtls "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	qtls "github.com/sagernet/sing-quic"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
)

func serverTLS(t *testing.T) *sbtls.STDServerConfig {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "quicgrace"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"quicgrace"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	config, err := sbtls.NewSTDServer(context.Background(), log.NewNOPFactory().Logger(), option.InboundTLSOptions{
		Enabled:     true,
		ALPN:        badoption.Listable[string]{"quicgrace"},
		Certificate: badoption.Listable[string]{string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))},
		Key:         badoption.Listable[string]{string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))},
	})
	if err != nil {
		t.Fatal(err)
	}
	return config.(*sbtls.STDServerConfig)
}

// connect listens through listen and connects one client. It returns the
// listener and the client once the server has accepted it.
func connect(t *testing.T, listen func(net.PacketConn) (qtls.Listener, error), idle time.Duration) (qtls.Listener, *quic.Conn) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := listen(pc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan struct{}, 1)
	go func() {
		if _, err := listener.Accept(context.Background()); err == nil {
			accepted <- struct{}{}
		}
	}()
	client, err := quic.DialAddr(context.Background(), pc.LocalAddr().String(),
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quicgrace"}},
		&quic.Config{MaxIdleTimeout: idle})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseWithError(0, "") })
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("server never accepted")
	}
	return listener, client
}

// closedWithin reports how the client learned its session ended, or nil if it
// did not within wait.
func closedWithin(client *quic.Conn, wait time.Duration) error {
	select {
	case <-client.Context().Done():
		return context.Cause(client.Context())
	case <-time.After(wait):
		return nil
	}
}

func isRemoteClose(err error) bool {
	var appErr *quic.ApplicationError
	return errors.As(err, &appErr) && appErr.Remote
}

// Through Wrap, closing the listener reaches the client as a CONNECTION_CLOSE
// at once. Without it -- sing-quic's own listen -- the client hears nothing and
// is left to its idle timeout; that half is the reason the package exists and
// is checked too, so an upstream change that fixes it shows up here.
func TestClosingTellsTheClient(t *testing.T) {
	quicConfig := &quic.Config{}

	listener, client := connect(t, func(pc net.PacketConn) (qtls.Listener, error) {
		return Wrap(serverTLS(t), Options{StatelessReset: true}).(qtls.ServerConfig).Listen(pc, quicConfig)
	}, 12*time.Second)
	_ = listener.Close()
	if err := closedWithin(client, time.Second); !isRemoteClose(err) {
		t.Fatalf("wrapped: client got %v, want a remote application close at once", err)
	}

	listener, client = connect(t, func(pc net.PacketConn) (qtls.Listener, error) {
		return qtls.ListenWithOptions(pc, serverTLS(t), quicConfig, qtls.ListenOptions{StatelessReset: true})
	}, 12*time.Second)
	_ = listener.Close()
	if err := closedWithin(client, 2*time.Second); err != nil {
		t.Logf("upstream now tells the client too (%v); quicgrace may no longer be needed", err)
	}
}

// A session closer closes the session of the client at its address, leaves
// the listener serving, and reports a miss so the registry can mute instead.
func TestSessionCloserClosesOneClient(t *testing.T) {
	sessions := NewSessions()
	_, client := connect(t, func(pc net.PacketConn) (qtls.Listener, error) {
		return Wrap(serverTLS(t), Options{Sessions: sessions}).(qtls.ServerConfig).Listen(pc, &quic.Config{})
	}, 12*time.Second)

	if err := sessions.Closer("127.0.0.1:1").Close(); !errors.Is(err, usersession.ErrNotClosed) {
		t.Fatalf("closer for an unknown address: got %v, want ErrNotClosed", err)
	}
	if err := closedWithin(client, 200*time.Millisecond); err != nil {
		t.Fatalf("a miss must not touch other sessions, client got %v", err)
	}

	// The client is bound to an unspecified address; the server sees loopback.
	source := M.SocksaddrFrom(M.ParseAddr("127.0.0.1"), M.SocksaddrFromNet(client.LocalAddr()).Port).String()
	if err := sessions.Closer(source).Close(); err != nil {
		t.Fatalf("closer for the client's address: %v", err)
	}
	if err := closedWithin(client, time.Second); !isRemoteClose(err) {
		t.Fatalf("client got %v, want a remote application close at once", err)
	}
}
