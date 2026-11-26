package nodeagent

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CertificateStatus describes public certificate material, never a private key,
// provider credentials, raw parser errors or arbitrary filesystem paths.
// Source is storage/config inspection, not an external handshake probe.
type CertificateStatus struct {
	Inbound    string `json:"inbound"`
	Provider   string `json:"provider,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	State      string `json:"state"`
	NotBefore  int64  `json:"not_before,omitempty"`
	NotAfter   int64  `json:"not_after,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

// ValidateCertificateReferences also runs on the panel before network I/O.
func ValidateCertificateReferences(config []byte) error {
	var value struct {
		Providers []struct {
			Type    string   `json:"type"`
			Tag     string   `json:"tag"`
			Domains []string `json:"domain"`
		} `json:"certificate_providers"`
		Inbounds []struct {
			Type      string `json:"type"`
			Transport struct {
				Type string `json:"type"`
			} `json:"transport"`
			TLS struct {
				Enabled  bool   `json:"enabled"`
				Provider string `json:"certificate_provider"`
				Reality  struct {
					Enabled bool `json:"enabled"`
				} `json:"reality"`
			} `json:"tls"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(config, &value); err != nil {
		return err
	}
	providers := map[string]bool{}
	for _, p := range value.Providers {
		if strings.TrimSpace(p.Tag) == "" || providers[p.Tag] {
			return errors.New("certificate provider tags must be nonempty and unique")
		}
		if p.Type != "acme" && p.Type != "cloudflare-origin-ca" {
			return errors.New("node certificate provider type is not supported")
		}
		if len(p.Domains) == 0 {
			return errors.New("certificate provider needs at least one domain")
		}
		providers[p.Tag] = true
	}
	for _, i := range value.Inbounds {
		if missingRequiredTLS(i.Type, i.Transport.Type, i.TLS.Enabled, i.TLS.Reality.Enabled) {
			return errors.New("this node inbound needs regular TLS")
		}
		if i.TLS.Enabled && i.TLS.Provider != "" && !providers[i.TLS.Provider] {
			return errors.New("TLS references an unknown node certificate provider")
		}
	}
	return nil
}

func (r *CoreRuntime) prepareCertificates(config []byte, create bool) ([]byte, error) {
	if err := ValidateCertificateReferences(config); err != nil {
		return nil, err
	}
	var value map[string]interface{}
	if err := json.Unmarshal(config, &value); err != nil {
		return nil, err
	}
	providers, _ := value["certificate_providers"].([]interface{})
	if inbounds, ok := value["inbounds"].([]interface{}); ok {
		for _, item := range inbounds {
			inbound, _ := item.(map[string]interface{})
			if err := r.resolveManagedCertificate(inbound); err != nil {
				return nil, err
			}
		}
	}
	used := map[string]bool{}
	for _, item := range providers {
		if err := r.prepareProvider(item.(map[string]interface{}), used, create); err != nil {
			return nil, err
		}
	}
	if r.stateDirectory != "" {
		resolveCertificatePaths(value["inbounds"], r.stateDirectory)
	}
	return json.Marshal(value)
}

func privateDirectory(root, relative string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("certificate storage must not contain symlinks")
		}
		if err := os.Chmod(current, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func certificateFromPEM(body []byte) *x509.Certificate {
	block, _ := pem.Decode(body)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil
	}
	cert, _ := x509.ParseCertificate(block.Bytes)
	return cert
}

func missingRequiredTLS(kind, transport string, enabled, reality bool) bool {
	return RequiresRegularTLS(kind, transport) && (!enabled || reality)
}
