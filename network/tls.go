package network

import (
	"crypto/tls"
	"fmt"
	"strings"
)

func NewTLSConfig(cert tls.Certificate, domain string) *tls.Config {
	if domain == "" {
		return &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	crt := cert
	return &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if !strings.EqualFold(hello.ServerName, domain) {
				return nil, fmt.Errorf("tls: unrecognized server name %q", hello.ServerName)
			}
			return &crt, nil
		},
	}
}
