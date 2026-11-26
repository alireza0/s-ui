package nodecert

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"
)

func NewAccountKey() (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}
func Directory(ca string) (string, error) {
	switch ca {
	case "", "letsencrypt":
		return "https://acme-v02.api.letsencrypt.org/directory", nil
	case "staging":
		return "https://acme-staging-v02.api.letsencrypt.org/directory", nil
	}
	return "", errors.New("unsupported_certificate_authority")
}

// Error keeps provider details out of public status while allowing a private
// diagnostic caller to inspect the underlying ACME failure explicitly.
type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string { return e.Code }
func (e *Error) Unwrap() error { return e.Cause }
func Issue(ctx context.Context, ca, email, accountKey, domain, method string, solver acmez.Solver) (Bundle, error) {
	directory, err := Directory(ca)
	if err != nil {
		return Bundle{}, err
	}
	client := &acme.Client{Directory: directory, HTTPClient: &http.Client{Timeout: 30 * time.Second}, UserAgent: "s-ui-node", PollTimeout: 3 * time.Minute}
	return issue(ctx, client, email, accountKey, domain, method, solver)
}
func issue(ctx context.Context, client *acme.Client, email, accountKey, domain, method string, solver acmez.Solver) (Bundle, error) {
	block, _ := pem.Decode([]byte(accountKey))
	if block == nil {
		return Bundle{}, errors.New("invalid_acme_account_key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return Bundle{}, errors.New("invalid_acme_account_key")
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return Bundle{}, errors.New("invalid_acme_account_key")
	}
	account := acme.Account{TermsOfServiceAgreed: true, PrivateKey: signer}
	if email != "" {
		account.Contact = []string{"mailto:" + email}
	}
	account, err = client.NewAccount(ctx, account)
	if err != nil {
		return Bundle{}, &Error{Code: "acme_account_failed", Cause: err}
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Bundle{}, err
	}
	issuer := &acmez.Client{Client: client, ChallengeSolvers: map[string]acmez.Solver{method: solver}}
	certs, err := issuer.ObtainCertificateForSANs(ctx, account, certKey, []string{domain})
	if err != nil || len(certs) == 0 {
		return Bundle{}, &Error{Code: "acme_order_failed", Cause: err}
	}
	der, err := x509.MarshalPKCS8PrivateKey(certKey)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{Certificate: string(certs[0].ChainPEM), Key: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))}, nil
}

// HTTP01 owns only its listener. A busy port is an error, never an invitation
// to stop an existing web server. Issuance does not acquire the VPN runtime gate.
type HTTP01 struct {
	mu      sync.Mutex
	Address string
	server  *http.Server
	tokens  map[string]string
}

func (h *HTTP01) Present(ctx context.Context, c acme.Challenge) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := h.startLocked(); err != nil {
		return err
	}
	h.tokens[strings.ToLower(c.Identifier.Value)+c.HTTP01ResourcePath()] = c.KeyAuthorization
	return nil
}
func (h *HTTP01) Reserve() error { h.mu.Lock(); defer h.mu.Unlock(); return h.startLocked() }
func (h *HTTP01) startLocked() error {
	if h.server == nil {
		address := h.Address
		if address == "" {
			address = ":80"
		}
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return errors.New("http01_port_occupied")
		}
		h.tokens = map[string]string{}
		h.server = &http.Server{Handler: http.HandlerFunc(h.serve), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
		go h.server.Serve(listener)
	}
	return nil
}
func (h *HTTP01) serve(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	h.mu.Lock()
	value := h.tokens[strings.ToLower(host)+r.URL.Path]
	h.mu.Unlock()
	if r.Method != http.MethodGet || value == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(value))
}
func (h *HTTP01) CleanUp(_ context.Context, c acme.Challenge) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.tokens, strings.ToLower(c.Identifier.Value)+c.HTTP01ResourcePath())
	if len(h.tokens) == 0 && h.server != nil {
		err := h.server.Close()
		h.server = nil
		return err
	}
	return nil
}
func (h *HTTP01) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.server == nil {
		return nil
	}
	err := h.server.Close()
	h.server = nil
	h.tokens = nil
	return err
}
