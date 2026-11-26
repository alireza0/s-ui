package nodeagent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/alireza0/s-ui/core"
	socksproxy "golang.org/x/net/proxy"
)

func freeV2Port(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	requireNodeTestSuccess(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestV2ActualVLESSTrafficDeniedAfterRevokeAndRestart(t *testing.T) {
	serverPort, clientPort := freeV2Port(t), freeV2Port(t)
	state := t.TempDir()
	m := NewManagerV2(7, FileStoreV2{Directory: state}, NewCoreRuntimeWithState(state))
	requireNodeTestSuccess(t, m.Restore())
	spec, grant := vlessState(m.Status().Epoch, serverPort)
	requireNodeTestSuccess(t, m.ApplyAccess(grant))
	requireNodeTestSuccess(t, m.ApplyStructure(spec))
	defer m.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "authenticated-v2") }))
	defer backend.Close()
	proxy := core.NewCoreWithContext(context.Background())
	defer proxy.Stop()
	config := []byte(fmt.Sprintf(`{"inbounds":[{"type":"socks","tag":"socks","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"vless","tag":"proxy","server":"127.0.0.1","server_port":%d,"uuid":"11111111-1111-4111-8111-111111111111"}],"route":{"final":"proxy"}}`, clientPort, serverPort))
	requireNodeTestSuccess(t, proxy.Start(config))
	proxyURL, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", clientPort))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	fetch := func() bool {
		response, err := client.Get(backend.URL)
		if err != nil {
			return false
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return string(body) == "authenticated-v2"
	}
	if !fetch() {
		t.Fatal("real VLESS traffic failed before revoke")
	}
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	requireNodeTestSuccess(t, err)
	defer echo.Close()
	go func() {
		for {
			conn, err := echo.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); _, _ = io.Copy(conn, conn) }()
		}
	}()
	dialer, err := socksproxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", clientPort), nil, socksproxy.Direct)
	requireNodeTestSuccess(t, err)
	openSession := func() net.Conn {
		t.Helper()
		conn, err := dialer.Dial("tcp", echo.Addr().String())
		requireNodeTestSuccess(t, err)
		t.Cleanup(func() { conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write([]byte("ping")); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "ping" {
			t.Fatal("persistent VLESS session failed", err)
		}
		_ = conn.SetDeadline(time.Time{})
		return conn
	}
	oldSession := openSession()
	deny := AccessSnapshot{NodeID: 7, Epoch: m.Status().Epoch, Revision: 2, Clients: []Grant{}}
	requireNodeTestSuccess(t, m.ApplyAccess(deny))
	if fetch() {
		t.Fatal("old VLESS credential still authenticated after revoke")
	}
	assertRevokedVLESSSessionClosed(t, oldSession)
	requireNodeTestSuccess(t, m.Close())
	next := NewManagerV2(7, FileStoreV2{Directory: state}, NewCoreRuntimeWithState(state))
	defer next.Close()
	requireNodeTestSuccess(t, next.Restore())
	if fetch() {
		t.Fatal("restart re-enabled old VLESS credential")
	}
	grant.Revision = 3
	grant.Clients[0].Name = "client/1/g2"
	requireNodeTestSuccess(t, next.ApplyAccess(grant))
	if !fetch() {
		t.Fatal("explicit re-enable did not restore valid traffic")
	}
	oldSession = openSession()
	grant.Revision = 4
	grant.Clients[0].Name = "client/1/g3"
	requireNodeTestSuccess(t, next.ApplyAccess(grant))
	assertRevokedVLESSSessionClosed(t, oldSession)
	if !fetch() {
		t.Fatal("generation change broke fresh valid sessions")
	}
}

func assertRevokedVLESSSessionClosed(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte("ping"))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err == nil {
		t.Fatal("old authenticated session survived access generation change")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("session remained open after access revocation")
	}
}
