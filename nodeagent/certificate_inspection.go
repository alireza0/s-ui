package nodeagent

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type inspectionProvider struct {
	Tag       string   `json:"tag"`
	Directory string   `json:"data_directory"`
	Domains   []string `json:"domain"`
}

type inspectionInbound struct {
	Tag string `json:"tag"`
	TLS struct {
		Enabled     bool     `json:"enabled"`
		ServerName  string   `json:"server_name"`
		Provider    string   `json:"certificate_provider"`
		Certificate []string `json:"certificate"`
		Path        string   `json:"certificate_path"`
		Reality     struct {
			Enabled bool `json:"enabled"`
		} `json:"reality"`
	} `json:"tls"`
}

func (r *CoreRuntime) Certificates() []CertificateStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := []CertificateStatus{}
	var config struct {
		Providers []inspectionProvider `json:"certificate_providers"`
		Inbounds  []inspectionInbound  `json:"inbounds"`
	}
	if json.Unmarshal(r.config, &config) != nil {
		return result
	}
	for _, inbound := range config.Inbounds {
		if !inbound.TLS.Enabled || inbound.TLS.Reality.Enabled {
			continue
		}
		status := CertificateStatus{Inbound: inbound.Tag, Provider: inbound.TLS.Provider, ServerName: inbound.TLS.ServerName, State: "missing"}
		cert := inspectionCertificate(inbound, config.Providers, &status)
		fillCertificateStatus(&status, cert)
		result = append(result, status)
	}
	return result
}

func inspectionCertificate(inbound inspectionInbound, providers []inspectionProvider, status *CertificateStatus) *x509.Certificate {
	if inbound.TLS.Provider != "" {
		var cert *x509.Certificate
		for _, provider := range providers {
			if provider.Tag != inbound.TLS.Provider {
				continue
			}
			if status.ServerName == "" && len(provider.Domains) == 1 {
				status.ServerName = provider.Domains[0]
			}
			scan := certificateScan{name: status.ServerName, best: cert}
			_ = filepath.WalkDir(provider.Directory, scan.visit)
			cert = scan.best
		}
		return cert
	}
	if len(inbound.TLS.Certificate) > 0 {
		return certificateFromPEM([]byte(strings.Join(inbound.TLS.Certificate, "\n")))
	}
	if inbound.TLS.Path != "" {
		if body, err := os.ReadFile(inbound.TLS.Path); err == nil {
			return certificateFromPEM(body)
		}
	}
	return nil
}

type certificateScan struct {
	name  string
	best  *x509.Certificate
	count int
}

func (s *certificateScan) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return filepath.SkipDir
	}
	if entry.IsDir() {
		return nil
	}
	if !strings.HasSuffix(entry.Name(), ".crt") && !strings.HasSuffix(entry.Name(), ".pem") {
		return nil
	}
	s.count++
	if s.count > 256 {
		return fs.SkipAll
	}
	candidate := inspectionFileCertificate(path, entry)
	if candidate == nil || candidate.IsCA {
		return nil
	}
	if s.name != "" && candidate.VerifyHostname(s.name) != nil {
		return nil
	}
	if s.best == nil || candidate.NotAfter.After(s.best.NotAfter) {
		s.best = candidate
	}
	return nil
}

func inspectionFileCertificate(path string, entry fs.DirEntry) *x509.Certificate {
	info, err := entry.Info()
	if err != nil || info.Size() > 1<<20 {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return certificateFromPEM(body)
}

func fillCertificateStatus(status *CertificateStatus, cert *x509.Certificate) {
	if cert == nil {
		return
	}
	status.NotBefore, status.NotAfter = cert.NotBefore.Unix(), cert.NotAfter.Unix()
	sum := sha256.Sum256(cert.Raw)
	status.SHA256 = hex.EncodeToString(sum[:])
	status.State = "ready"
	now := time.Now()
	if now.Before(cert.NotBefore) {
		status.State = "not_yet_valid"
	} else if !now.Before(cert.NotAfter) {
		status.State = "expired"
	} else if status.ServerName != "" && cert.VerifyHostname(status.ServerName) != nil {
		status.State = "name_mismatch"
	}
}
