package nodes

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"github.com/alireza0/s-ui/database/model"
	"golang.org/x/crypto/ssh"
	"net"
	"testing"
	"time"
)

func TestLiveSSHCancelDuringHandshakeAndChannelOpen(t *testing.T) {
	for _, handshake := range []bool{true, false} {
		name := "channel"
		if handshake {
			name = "handshake"
		}
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			_, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			signer, err := ssh.NewSignerFromKey(private)
			if err != nil {
				t.Fatal(err)
			}
			config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
			config.AddHostKey(signer)
			started := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			go func() {
				conn, e := listener.Accept()
				if e != nil {
					return
				}
				defer conn.Close()
				if handshake {
					_, _ = conn.Write([]byte("SSH-2.0-stalled-fixture\r\n"))
					close(started)
					<-release
					return
				}
				server, channels, requests, e := ssh.NewServerConn(conn, config)
				if e != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				if <-channels != nil {
					close(started)
					<-release
				}
			}()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			install := model.NodeInstallation{IP: "127.0.0.1", SSHPort: listener.Addr().(*net.TCPAddr).Port, SSHUser: "fixture"}
			done := make(chan error, 1)
			go func() {
				client, e := connectSSH(ctx, &install, "fixture-password", false)
				if e == nil {
					defer client.Close()
					_, e = client.NewSession()
				}
				done <- e
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("SSH fixture did not reach the blocked operation")
			}
			cancel()
			select {
			case e := <-done:
				if e == nil {
					t.Fatal("cancelled SSH operation succeeded")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("SSH operation ignored context cancellation")
			}
		})
	}
}
