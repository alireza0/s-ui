package util

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"strings"
)

func CertPEMFromTLS(tlsConfig map[string]interface{}) string {
	if tlsConfig == nil {
		return ""
	}
	switch c := tlsConfig["certificate"].(type) {
	case string:
		if c != "" {
			return c
		}
	case []interface{}:
		lines := make([]string, 0, len(c))
		for _, l := range c {
			if s, ok := l.(string); ok {
				lines = append(lines, s)
			}
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	if path, ok := tlsConfig["certificate_path"].(string); ok && path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return ""
}

func parseLeafCert(pemData string) *x509.Certificate {
	rest := []byte(pemData)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil
			}
			return cert
		}
	}
}

// CertPublicKeySha256 returns the base64-encoded SHA256 of the certificate's
// SubjectPublicKeyInfo (sing-box `certificate_public_key_sha256` / link pinSHA256).
func CertPublicKeySha256(pemData string) string {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// CertSha256Hex returns the lowercase hex SHA256 of the whole certificate (DER),
// matching `openssl x509 -fingerprint -sha256` and Clash/mihomo's `fingerprint`.
func CertSha256Hex(pemData string) string {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}
