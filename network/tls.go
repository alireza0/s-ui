package network

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type certReloader struct {
	certFile string
	keyFile  string

	mu      sync.RWMutex
	cert    *tls.Certificate
	certMod time.Time
	keyMod  time.Time
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	r := &certReloader{certFile: certFile, keyFile: keyFile}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *certReloader) reload() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.cert = &cert
	if info, err := os.Stat(r.certFile); err == nil {
		r.certMod = info.ModTime()
	}
	if info, err := os.Stat(r.keyFile); err == nil {
		r.keyMod = info.ModTime()
	}
	r.mu.Unlock()
	return nil
}

func (r *certReloader) changed() bool {
	certInfo, errC := os.Stat(r.certFile)
	keyInfo, errK := os.Stat(r.keyFile)
	if errC != nil || errK != nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return certInfo.ModTime().After(r.certMod) || keyInfo.ModTime().After(r.keyMod)
}

func (r *certReloader) getCertificate() *tls.Certificate {
	if r.changed() {
		// On failure keep serving the previous certificate.
		_ = r.reload()
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert
}

func NewTLSConfig(certFile, keyFile, domain string) (*tls.Config, error) {
	reloader, err := newCertReloader(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if domain != "" && !strings.EqualFold(hello.ServerName, domain) {
				return nil, fmt.Errorf("tls: unrecognized server name %q", hello.ServerName)
			}
			return reloader.getCertificate(), nil
		},
	}, nil
}
