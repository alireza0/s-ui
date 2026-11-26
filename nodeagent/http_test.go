package nodeagent

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func controlCertificates(t *testing.T) (string, string, string, *x509.CertPool, tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test control CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	issue := func(serial int64, usage x509.ExtKeyUsage) ([]byte, []byte) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "control"}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
		der, e := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		if e != nil {
			t.Fatal(e)
		}
		keyDER, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			t.Fatal(e)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	}
	serverCert, serverKey := issue(2, x509.ExtKeyUsageServerAuth)
	certPath, keyPath := filepath.Join(dir, "server.pem"), filepath.Join(dir, "server-key.pem")
	if err := os.WriteFile(certPath, serverCert, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, serverKey, 0o600); err != nil {
		t.Fatal(err)
	}
	clientCert, clientKey := issue(3, x509.ExtKeyUsageClientAuth)
	client, e := tls.X509KeyPair(clientCert, clientKey)
	if e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	return certPath, keyPath, caPath, pool, client
}

func TestControlRequiresClientCertificateAndReturnsNoCredentials(t *testing.T) {
	cert, key, ca, roots, identity := controlCertificates(t)
	tlsConfig, err := ServerTLS(cert, key, ca)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManagerV2(7, FileStoreV2{Directory: t.TempDir()}, &safeFake{})
	requireNodeTestSuccess(t, m.Restore())
	defer m.Close()
	_, grant := vlessState(m.Status().Epoch, 14443)
	server := httptest.NewUnstartedServer(HandlerV2(m))
	server.TLS = tlsConfig
	server.StartTLS()
	defer server.Close()
	unauthorized := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}}, Timeout: 3 * time.Second}
	if response, err := unauthorized.Get(server.URL + "/v2/status"); err == nil {
		response.Body.Close()
		t.Fatal("control accepted a connection without client certificate")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{identity}, MinVersion: tls.VersionTLS13}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	body, _ := json.Marshal(grant)
	request, _ := http.NewRequest(http.MethodPut, server.URL+"/v2/access", bytes.NewReader(body))
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("status %d: %s", response.StatusCode, result)
	}
	if bytes.Contains(result, []byte("11111111")) || bytes.Contains(result, []byte(`"config"`)) {
		t.Fatal("status exposed credentials")
	}
	request, _ = http.NewRequest(http.MethodPut, server.URL+"/v2/access", bytes.NewBufferString(string(body)+` {}`))
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("trailing body accepted")
	}
	grant.NodeID = 8
	grant.Revision++
	body, _ = json.Marshal(grant)
	request, _ = http.NewRequest(http.MethodPut, server.URL+"/v2/access", bytes.NewReader(body))
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 409 {
		t.Fatal("foreign node identity accepted")
	}
}
