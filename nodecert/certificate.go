// Package nodecert owns certificate validation and immutable private artifacts.
package nodecert

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Bundle struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}
type Metadata struct {
	Domain          string `json:"domain"`
	SHA256          string `json:"sha256"`
	Digest          string `json:"digest"`
	NotBefore       int64  `json:"not_before"`
	NotAfter        int64  `json:"not_after"`
	CertificatePath string `json:"certificate_path"`
	KeyPath         string `json:"key_path"`
}

func Digest(bundle Bundle) string {
	sum := sha256.Sum256([]byte(bundle.Certificate + "\x00" + bundle.Key))
	return hex.EncodeToString(sum[:])
}

func Validate(bundle Bundle, domain string, roots *x509.CertPool, now time.Time) (Metadata, error) {
	var result Metadata
	if len(bundle.Certificate) > 1<<20 || len(bundle.Key) > 1<<20 {
		return result, errors.New("certificate_material_too_large")
	}
	pair, err := tls.X509KeyPair([]byte(bundle.Certificate), []byte(bundle.Key))
	if err != nil || len(pair.Certificate) == 0 {
		return result, errors.New("certificate_key_pair_invalid")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return result, errors.New("certificate_invalid")
	}
	if leaf.VerifyHostname(domain) != nil {
		return result, errors.New("certificate_hostname_mismatch")
	}
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return result, errors.New("certificate_expired_or_not_yet_valid")
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return result, errors.New("certificate_chain_invalid")
		}
		intermediates.AddCert(cert)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: domain, Roots: roots, Intermediates: intermediates, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		return result, errors.New("certificate_chain_untrusted")
	}
	sha := sha256.Sum256(leaf.Raw)
	result = Metadata{Domain: domain, SHA256: hex.EncodeToString(sha[:]), Digest: Digest(bundle), NotBefore: leaf.NotBefore.Unix(), NotAfter: leaf.NotAfter.Unix()}
	result.CertificatePath = filepath.ToSlash(filepath.Join("certificates", "managed", result.Digest, certificateFileName))
	result.KeyPath = filepath.ToSlash(filepath.Join("certificates", "managed", result.Digest, privateKeyFileName))
	return result, nil
}

func NeedsRenewal(meta Metadata, now time.Time) bool {
	life := time.Duration(meta.NotAfter-meta.NotBefore) * time.Second
	window := life / 3
	if window > 30*24*time.Hour {
		window = 30 * 24 * time.Hour
	}
	if window < time.Hour {
		window = time.Hour
	}
	return meta.NotAfter == 0 || time.Unix(meta.NotAfter, 0).Sub(now) <= window
}

func PrivateDirectory(root, relative string) error {
	if filepath.IsAbs(relative) || strings.Contains(relative, "\\") || relative == ".." || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return errors.New("invalid_private_directory")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid_private_root")
	}
	if err := os.Chmod(root, 0700); err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if part == "." {
			continue
		}
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("private_directory_contains_symlink")
		}
	}
	return nil
}
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func WriteJSON(root, name string, value interface{}) error {
	if filepath.Base(name) != name {
		return errors.New("invalid_state_name")
	}
	if err := PrivateDirectory(root, "."); err != nil {
		return err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(body)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(f.Name(), filepath.Join(root, name)); err != nil {
		return err
	}
	return syncDirectory(root)
}
func Install(root string, bundle Bundle, domain string, roots *x509.CertPool) (Metadata, error) {
	meta, err := Validate(bundle, domain, roots, time.Now())
	if err != nil {
		return meta, err
	}
	parent := filepath.Join("certificates", "managed")
	if err := PrivateDirectory(root, parent); err != nil {
		return meta, err
	}
	target := filepath.Join(root, parent, meta.Digest)
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return meta, errors.New("certificate_artifact_invalid")
		}
		cert, err := os.ReadFile(filepath.Join(target, certificateFileName))
		if err != nil {
			return meta, err
		}
		key, err := os.ReadFile(filepath.Join(target, privateKeyFileName))
		if err != nil {
			return meta, err
		}
		if Digest(Bundle{string(cert), string(key)}) != meta.Digest {
			return meta, errors.New("certificate_artifact_corrupt")
		}
		return meta, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return meta, err
	}
	temp, err := os.MkdirTemp(filepath.Join(root, parent), ".artifact-*")
	if err != nil {
		return meta, err
	}
	defer os.RemoveAll(temp)
	for name, body := range map[string]string{certificateFileName: bundle.Certificate, privateKeyFileName: bundle.Key} {
		f, err := os.OpenFile(filepath.Join(temp, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return meta, err
		}
		_, err = f.WriteString(body)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return meta, err
		}
		if closeErr != nil {
			return meta, closeErr
		}
	}
	if err := syncDirectory(temp); err != nil {
		return meta, err
	}
	if err := os.Rename(temp, target); err != nil {
		return meta, err
	}
	return meta, syncDirectory(filepath.Join(root, parent))
}

func LeafPEM(bundle Bundle) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(bundle.Certificate))
	if block == nil {
		return nil, errors.New("certificate_invalid")
	}
	return x509.ParseCertificate(block.Bytes)
}
