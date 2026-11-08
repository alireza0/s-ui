package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Clients send no SNI when they dial an IP address, so a panel or subscription
// domain set to the server's IP refused every TLS handshake.
func TestTLSConfigServerName(t *testing.T) {
	certFile, keyFile := writeTestCert(t)
	for _, tc := range []struct {
		domain, sni string
		ok          bool
	}{
		{"", "", true},
		{"", "any.example", true},
		{"panel.example", "panel.example", true},
		{"panel.example", "Panel.Example", true},
		{"panel.example", "", false},
		{"panel.example", "other.example", false},
		{"192.0.2.1", "", true},
		{"2001:db8::1", "", true},
		{"192.0.2.1", "other.example", false},
	} {
		c, err := NewTLSConfig(certFile, keyFile, tc.domain)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := c.GetCertificate(&tls.ClientHelloInfo{ServerName: tc.sni})
		if ok := err == nil && cert != nil; ok != tc.ok {
			t.Errorf("domain %q, SNI %q: accepted %v, want %v (err %v)", tc.domain, tc.sni, ok, tc.ok, err)
		}
	}
}

func writeTestCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		IPAddresses:  []net.IP{net.ParseIP("192.0.2.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}
