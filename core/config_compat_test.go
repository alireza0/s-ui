package core

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"
)

// collectManager records deprecation notes instead of writing them to stderr,
// so tests can assert on which ones a config triggers.
type collectManager struct{ notes []deprecated.Note }

func (c *collectManager) ReportDeprecated(feature deprecated.Note) {
	c.notes = append(c.notes, feature)
}

// writeTestCert generates a throwaway self-signed cert for the TLS-bearing
// inbounds in testdata, returning the certificate and key paths.
func writeTestCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "t.example.com"},
		DNSNames:     []string{"t.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
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
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)
	return certPath, keyPath
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestConfigCompat starts a box for each config in testdata/configs and reports
// the deprecation notes it triggers. It guards the sing-box upgrade: configs
// s-ui's UI can produce must keep starting, and any new deprecation shows up in
// the test log so it can be migrated before release.
func TestConfigCompat(t *testing.T) {
	certPath, keyPath := writeTestCert(t)
	cacheDir := t.TempDir()

	files, err := filepath.Glob(filepath.Join("testdata", "configs", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test configs found")
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			data := strings.NewReplacer(
				"__CERT__", certPath,
				"__KEY__", keyPath,
				"__CACHEDIR__", cacheDir,
			).Replace(string(raw))

			ctx := Context(context.Background(), InboundRegistry(), OutboundRegistry(),
				EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
			notes := &collectManager{}
			ctx = service.ContextWith[deprecated.Manager](ctx, notes)

			var opts option.Options
			if err = opts.UnmarshalJSONContext(ctx, []byte(data)); err != nil {
				t.Fatalf("parse: %v", err)
			}

			instance, err := NewBox(Options{Context: ctx, Options: opts})
			if err != nil {
				t.Fatalf("create box: %v", err)
			}
			defer instance.Close()
			if err = instance.Start(); err != nil {
				t.Fatalf("start: %v", err)
			}

			for _, note := range notes.notes {
				t.Logf("deprecated: %s -- %s", note.Name, note.Description)
			}
		})
	}
}

// TestConfigCompatClean builds each config in testdata/clean and asserts it
// raises no deprecation warning. It covers the post-migration shapes produced
// by database.migrateSingBox114, so the migration is known to actually resolve
// what it claims to.
//
// These configs are built but not started, which is what lets them cover cases
// TestConfigCompat cannot: an ACME provider would reach out to Let's Encrypt on
// start, and a bridge outbound needs privileges to create its tun. Every
// deprecation these shapes could raise is reported while the box is built.
func TestConfigCompatClean(t *testing.T) {
	acmeDir := t.TempDir()
	cacheDir := t.TempDir()
	certPath, keyPath := writeTestCert(t)

	files, err := filepath.Glob(filepath.Join("testdata", "clean", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no clean test configs found")
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			data := strings.NewReplacer(
				"__ACMEDIR__", acmeDir,
				"__CACHEDIR__", cacheDir,
				"__CERT__", certPath,
				"__KEY__", keyPath,
			).Replace(string(raw))

			ctx := Context(context.Background(), InboundRegistry(), OutboundRegistry(),
				EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
			notes := &collectManager{}
			ctx = service.ContextWith[deprecated.Manager](ctx, notes)

			var opts option.Options
			if err = opts.UnmarshalJSONContext(ctx, []byte(data)); err != nil {
				t.Fatalf("parse: %v", err)
			}
			instance, err := NewBox(Options{Context: ctx, Options: opts})
			skipIfFeatureMissing(t, err)
			if err != nil {
				t.Fatalf("create box: %v", err)
			}
			instance.Close()

			for _, note := range notes.notes {
				t.Errorf("migrated config still reports deprecation %q: %s", note.Name, note.Description)
			}
		})
	}
}
