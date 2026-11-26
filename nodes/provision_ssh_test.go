package nodes

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"golang.org/x/crypto/ssh"
)

func testSSHServer(t *testing.T, address string) (string, func()) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if string(password) != "bootstrap-secret" {
			return nil, errors.New("wrong password")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					conn.Close()
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					channel.Reject(ssh.Prohibited, "test server")
				}
			}()
		}
	}()
	return listener.Addr().String(), func() { listener.Close() }
}

func TestProvisionSSHHostChangeRequiresExplicitRecovery(t *testing.T) {
	address, stop := testSSHServer(t, "127.0.0.1:0")
	host, rawport, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(rawport)
	install := model.NodeInstallation{IP: host, SSHPort: port, SSHUser: "root"}
	client, err := connectSSH(context.Background(), &install, "bootstrap-secret", false)
	if err != nil {
		stop()
		t.Fatal(err)
	}
	client.Close()
	old := install.HostKey
	stop()
	_, stop = testSSHServer(t, address)
	defer stop()
	if client, err = connectSSH(context.Background(), &install, "bootstrap-secret", false); err == nil {
		client.Close()
		t.Fatal("changed host key accepted")
	}
	if install.HostKey != old {
		t.Fatal("failed verification overwrote pinned key")
	}
	client, err = connectSSH(context.Background(), &install, "bootstrap-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if install.HostKey == old || sshFingerprint(install.HostKey) == "" {
		t.Fatal("explicit recovery did not pin the new key")
	}
}
