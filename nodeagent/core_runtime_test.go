package nodeagent

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/alireza0/s-ui/logger"
	"github.com/op/go-logging"
	"github.com/sagernet/sing-box/adapter"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func TestCoreHotUsersAndFailedReplacementKeepWorkingListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	r := NewCoreRuntime()
	m := NewManagerV2(7, FileStoreV2{Directory: t.TempDir()}, r)
	defer m.Close()
	requireNodeTestSuccess(t, m.Restore())
	spec, grant := vlessState(m.Status().Epoch, port)
	requireNodeTestSuccess(t, m.ApplyAccess(grant))
	requireNodeTestSuccess(t, m.ApplyStructure(spec))
	box := r.core.GetInstance()
	providerContext := r.core.GetCtx()
	oldSession, peer := net.Pipe()
	defer peer.Close()
	tracked := box.SessionTracker().RoutedConnection(context.Background(), oldSession, adapter.InboundContext{Inbound: "vpn", User: "client/1"}, nil, nil)
	defer tracked.Close()
	grant.Revision++
	grant.Clients[0].Credentials["vless"] = json.RawMessage(`{"uuid":"22222222-2222-4222-8222-222222222222"}`)
	requireNodeTestSuccess(t, m.ApplyAccess(grant))
	if r.core.GetInstance() != box {
		t.Fatal("user update restarted whole core")
	}
	if providerContext.Err() != nil {
		t.Fatal("hot user update canceled certificate provider")
	}
	if sessions := box.SessionTracker().Sessions(); len(sessions) != 0 {
		t.Fatal("credential rotation left the old authenticated session alive")
	}
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	rejected, _ := vlessState(m.Status().Epoch, busy.Addr().(*net.TCPAddr).Port)
	rejected.Revision++
	if err := m.ApplyStructure(rejected); err == nil {
		t.Fatal("occupied listener was accepted")
	}
	if !m.Status().Running || m.Status().SpecRevision != 1 {
		t.Fatalf("rollback lost running state: %+v", m.Status())
	}
	if providerContext.Err() == nil {
		t.Fatal("retired core left asynchronous provider work running")
	}
	check, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err == nil {
		check.Close()
		t.Fatal("original listener was not restored")
	}
}
