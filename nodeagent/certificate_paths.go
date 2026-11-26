package nodeagent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
)

func (r *CoreRuntime) resolveManagedCertificate(inbound map[string]interface{}) error {
	settings, _ := inbound["tls"].(map[string]interface{})
	cert, _ := settings["certificate_path"].(string)
	key, _ := settings["key_path"].(string)
	if !strings.HasPrefix(cert, "managed:") && !strings.HasPrefix(key, "managed:") {
		return nil
	}
	if cert != "managed:primary" || key != "managed:primary" || r.certificateResolver == nil {
		return errors.New("managed_certificate_binding_invalid")
	}
	name, _ := settings["server_name"].(string)
	meta, err := r.certificateResolver(name)
	if err != nil {
		return err
	}
	settings["certificate_path"] = filepath.Join(r.stateDirectory, filepath.FromSlash(meta.CertificatePath))
	settings["key_path"] = filepath.Join(r.stateDirectory, filepath.FromSlash(meta.KeyPath))
	return nil
}

func (r *CoreRuntime) providerDirectory(provider map[string]interface{}) (string, string, error) {
	if r.stateDirectory == "" {
		return "", "", errors.New("node certificate providers require a persistent state directory")
	}
	name, _ := provider["data_directory"].(string)
	if name == "" {
		sum := sha256.Sum256([]byte(provider["type"].(string) + "/" + provider["tag"].(string)))
		name = hex.EncodeToString(sum[:12])
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" || strings.ContainsAny(name, "$\\:") {
		return "", "", errors.New("provider data directory must be relative to node certificate storage")
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("invalid provider data directory")
	}
	return clean, filepath.Join(r.stateDirectory, "certificates", clean), nil
}

func relativeDirectoryInside(relative string) bool {
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (r *CoreRuntime) prepareProvider(provider map[string]interface{}, used map[string]bool, create bool) error {
	clean, path, err := r.providerDirectory(provider)
	if err != nil {
		return err
	}
	for previous := range used {
		one, _ := filepath.Rel(previous, path)
		two, _ := filepath.Rel(path, previous)
		if relativeDirectoryInside(one) || relativeDirectoryInside(two) {
			return errors.New("certificate provider data directories must not overlap")
		}
	}
	used[path] = true
	if create {
		if err := privateDirectory(r.stateDirectory, filepath.Join("certificates", clean)); err != nil {
			return err
		}
	}
	provider["data_directory"] = path
	return nil
}

// Relative certificate paths belong to the node, never the panel or process cwd.
func resolveCertificatePaths(value interface{}, directory string) {
	switch current := value.(type) {
	case map[string]interface{}:
		for key, item := range current {
			if key == "certificate_path" || key == "key_path" || key == "client_certificate_path" {
				current[key] = resolveCertificatePath(item, directory)
			} else {
				resolveCertificatePaths(item, directory)
			}
		}
	case []interface{}:
		for _, item := range current {
			resolveCertificatePaths(item, directory)
		}
	}
}

func resolveCertificatePath(value interface{}, directory string) interface{} {
	switch current := value.(type) {
	case string:
		if current != "" && !filepath.IsAbs(current) {
			return filepath.Join(directory, current)
		}
	case []interface{}:
		for index, item := range current {
			if name, ok := item.(string); ok && name != "" && !filepath.IsAbs(name) {
				current[index] = filepath.Join(directory, name)
			}
		}
	}
	return value
}
