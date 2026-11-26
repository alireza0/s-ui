package nodes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"
	"golang.org/x/crypto/ssh"
)

var stateKeyLock sync.Mutex

func provisionCipher(create bool) (cipher.AEAD, error) {
	stateKeyLock.Lock()
	defer stateKeyLock.Unlock()
	p := filepath.Join(config.GetDBFolderPath(), "node-provision.key")
	key, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		if !create {
			return nil, err
		}
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return nil, err
		}
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("invalid node provisioning state key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func sealSecret(value string) (string, error) {
	a, err := provisionCipher(true)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, []byte(value), []byte("s-ui-node-provision-v1"))), nil
}
func openSecret(value string) (string, error) {
	a, err := provisionCipher(false)
	if err != nil {
		return "", err
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(b) < a.NonceSize() {
		return "", errors.New("invalid encrypted node credential")
	}
	p, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte("s-ui-node-provision-v1"))
	return string(p), err
}
func privatePEM(key ed25519.PrivateKey) (string, error) {
	b, err := x509.MarshalPKCS8PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b})), err
}
func newSSHKey() (private, public string, err error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	private, err = privatePEM(key)
	if err != nil {
		return "", "", err
	}
	pub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return "", "", err
	}
	return private, string(ssh.MarshalAuthorizedKey(pub)), nil
}
func newControlIdentity(node *model.Node, install *model.NodeInstallation) error {
	pub, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	serial := func() *big.Int { n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120)); return n }
	start := time.Now().Add(-5 * time.Minute)
	end := time.Now().AddDate(1, 0, 0)
	ca := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "S-UI node control"}, NotBefore: start, NotAfter: end.AddDate(4, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, caKey)
	if err != nil {
		return err
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	node.ControlCA = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	issue := func(server bool) (string, string, error) {
		p, k, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return "", "", e
		}
		c := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "S-UI node"}, NotBefore: start, NotAfter: end, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		if server {
			c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			c.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
			c.DNSNames = []string{"localhost"}
		}
		d, e := x509.CreateCertificate(rand.Reader, c, ca, p, caKey)
		if e != nil {
			return "", "", e
		}
		s, e := privatePEM(k)
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: d})), s, e
	}
	node.ControlCertificate, node.ControlKey, err = issue(false)
	if err != nil {
		return err
	}
	cert, key, err := issue(true)
	if err != nil {
		return err
	}
	install.ServerCertificate, err = sealSecret(cert)
	if err != nil {
		return err
	}
	install.ServerKey, err = sealSecret(key)
	return err
}
