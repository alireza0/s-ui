package nodeagent

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tlsCertificate(t *testing.T, expires time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"vpn.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	body, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: body})
}

func TestNodeProviderStorageIsPrivateStableAndContained(t *testing.T) {
	r := NewCoreRuntimeWithState(t.TempDir())
	config := []byte(`{"certificate_providers":[{"type":"acme","tag":"own","domain":["vpn.example"]}],"inbounds":[]}`)
	a, err := r.prepareCertificates(config, true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.prepareCertificates(config, false)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("unstable storage", err)
	}
	var parsed struct {
		Providers []struct {
			Path string `json:"data_directory"`
		} `json:"certificate_providers"`
	}
	_ = json.Unmarshal(a, &parsed)
	path := parsed.Providers[0].Path
	if !strings.HasPrefix(path, r.stateDirectory+string(filepath.Separator)) {
		t.Fatal("outside state")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal("directory missing", err)
	}
	for _, name := range []string{"../escape", "/absolute", "C:/panel", "$HOME/acme", "."} {
		var value map[string]interface{}
		_ = json.Unmarshal(config, &value)
		value["certificate_providers"].([]interface{})[0].(map[string]interface{})["data_directory"] = name
		body, _ := json.Marshal(value)
		if _, err := r.prepareCertificates(body, true); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := NewCoreRuntime().prepareCertificates(config, false); err == nil {
		t.Fatal("provider used implicit home directory")
	}
	overlap := []byte(`{"certificate_providers":[{"type":"acme","tag":"one","domain":["vpn.example"],"data_directory":"shared"},{"type":"acme","tag":"two","domain":["other.example"],"data_directory":"shared/nested"}],"inbounds":[]}`)
	if _, err := r.prepareCertificates(overlap, false); err == nil {
		t.Fatal("overlapping provider namespaces accepted")
	}
	t.Run("symlink", func(t *testing.T) {
		if err := os.Symlink(t.TempDir(), filepath.Join(r.stateDirectory, "certificates", "escape")); err != nil {
			t.Skip("symlink creation not available")
		}
		body := bytes.Replace(config, []byte(`"tag":"own"`), []byte(`"tag":"own","data_directory":"escape"`), 1)
		if _, err := r.prepareCertificates(body, true); err == nil {
			t.Fatal("symlink escaped managed certificate storage")
		}
	})
}

func TestCertificateReportTracksRotationExpiryAndDoesNotLeakKeys(t *testing.T) {
	r := NewCoreRuntimeWithState(t.TempDir())
	path := filepath.Join(r.stateDirectory, "vpn.crt")
	r.config = []byte(`{"inbounds":[{"tag":"vpn","tls":{"enabled":true,"server_name":"vpn.example","certificate_path":` + strconvQuote(path) + `}}]}`)
	if r.Certificates()[0].State != "missing" {
		t.Fatal("missing certificate reported available")
	}
	if err := os.WriteFile(path, tlsCertificate(t, time.Now().Add(time.Hour)), 0o600); err != nil {
		t.Fatal(err)
	}
	first := r.Certificates()[0]
	if first.State != "ready" || first.SHA256 == "" {
		t.Fatal(first)
	}
	if err := os.WriteFile(path, tlsCertificate(t, time.Now().Add(2*time.Hour)), 0o600); err != nil {
		t.Fatal(err)
	}
	if second := r.Certificates()[0]; second.SHA256 == first.SHA256 || second.NotAfter <= first.NotAfter {
		t.Fatal("rotation not observed")
	}
	if err := os.WriteFile(path, tlsCertificate(t, time.Now().Add(-time.Minute)), 0o600); err != nil {
		t.Fatal(err)
	}
	if r.Certificates()[0].State != "expired" {
		t.Fatal("expired certificate ready")
	}
	body, _ := json.Marshal(r.Certificates())
	if bytes.Contains(body, []byte(path)) || bytes.Contains(body, []byte("PRIVATE")) {
		t.Fatal("private status")
	}
}

func strconvQuote(value string) string { body, _ := json.Marshal(value); return string(body) }

func TestQUICAndProviderReferencesAreValidatedBeforeCoreStart(t *testing.T) {
	for _, value := range []string{`{"inbounds":[{"type":"tuic"}]}`, `{"inbounds":[{"type":"hysteria2","tls":{"enabled":true,"reality":{"enabled":true}}}]}`, `{"inbounds":[{"type":"vless","tls":{"enabled":true,"certificate_provider":"missing"}}]}`, `{"certificate_providers":[{"type":"acme","tag":"one"}]}`} {
		if ValidateCertificateReferences([]byte(value)) == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
