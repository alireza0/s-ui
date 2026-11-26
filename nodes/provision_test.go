package nodes

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"golang.org/x/crypto/ssh"
)

func TestProvisionSecretTamperAndMissingStateKey(t *testing.T) {
	folder := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", folder)
	secret := "password-not-for-storage"
	sealed, err := sealSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, secret) {
		t.Fatal("plaintext stored")
	}
	if plain, err := openSecret(sealed); err != nil || plain != secret {
		t.Fatal("roundtrip", err)
	}
	altered := []byte(sealed)
	altered[len(altered)/2] = '!'
	if _, err = openSecret(string(altered)); err == nil {
		t.Fatal("tamper accepted")
	}
	path := filepath.Join(folder, "node-provision.key")
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = openSecret(sealed); err == nil {
		t.Fatal("missing state key accepted")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reading a missing key silently replaced it")
	}
}

func TestProvisionIdentityIsMutuallyAuthenticatedAndSSHKeyUsable(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	var node model.Node
	var install model.NodeInstallation
	if err := newControlIdentity(&node, &install); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(node.ControlCA)) {
		t.Fatal("CA")
	}
	cert, err := openSecret(install.ServerCertificate)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(cert))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "127.0.0.1", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair([]byte(node.ControlCertificate), []byte(node.ControlKey))
	if err != nil {
		t.Fatal(err)
	}
	controller, _ := x509.ParseCertificate(pair.Certificate[0])
	if _, err = controller.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatal(err)
	}
	private, public, err := newSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		t.Fatal(err)
	}
	if string(ssh.MarshalAuthorizedKey(signer.PublicKey())) != public {
		t.Fatal("SSH identity mismatch")
	}
}

func TestProvisionInputValidationAndSecretRedaction(t *testing.T) {
	good := ProvisionRequest{Name: "Amsterdam", IP: "203.0.113.20", Protocols: []string{"vless", "vless"}}
	if err := normalizeProvision(&good); err != nil || good.SSHPort != 22 || good.SSHUser != "root" || len(good.Protocols) != 1 {
		t.Fatal("defaults", err)
	}
	bad := good
	bad.IP = "https://host/;echo"
	if normalizeProvision(&bad) == nil {
		t.Fatal("bad IP accepted")
	}
	bad = good
	bad.Protocols = []string{"xray"}
	if normalizeProvision(&bad) == nil {
		t.Fatal("unsupported engine accepted")
	}
	bad = good
	bad.ReplaceHostKey = true
	if normalizeProvision(&bad) == nil {
		t.Fatal("silent host replacement accepted")
	}
	data, _ := json.Marshal(model.NodeInstallation{PrivateKey: "SECRET-SSH", ServerKey: "SECRET-TLS", HostKey: "HOST-KEY"})
	if strings.Contains(string(data), "SECRET") || strings.Contains(string(data), "HOST-KEY") {
		t.Fatal("private installation fields exposed")
	}
}
