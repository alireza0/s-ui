package nodes

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractReleaseAgent writes only the fixed files shipped in the agent bundle.
func ExtractReleaseAgent(stage, name string, body io.Reader) error {
	var mode os.FileMode
	switch name {
	case "s-ui-node-linux-amd64", "s-ui-node-linux-arm64":
		mode = 0755
	case "manifest.json", "s-ui-node-linux-amd64.sha256", "s-ui-node-linux-arm64.sha256":
		mode = 0644
	default:
		return nil
	}
	folder := filepath.Join(stage, "agents")
	if err := os.MkdirAll(folder, 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(folder, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, body)
	return errors.Join(err, file.Close())
}

// InstallReleaseAgents switches to a verified bundle before the panel binary
// is replaced. Finish commits the change or rolls back to the previous bundle.
func InstallReleaseAgents(stage, installDir string) (func(bool) error, error) {
	incoming := filepath.Join(stage, "agents")
	if _, err := os.Stat(incoming); errors.Is(err, os.ErrNotExist) {
		return func(bool) error { return nil }, nil
	} else if err != nil {
		return nil, err
	}
	if err := validateReleaseAgents(incoming); err != nil {
		return nil, err
	}
	backup, err := os.MkdirTemp(installDir, ".agents-update-")
	if err != nil {
		return nil, err
	}
	target, previous := filepath.Join(installDir, "agents"), filepath.Join(backup, "agents")
	err = os.Rename(target, previous)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(err, os.Remove(backup))
	}
	hadPrevious := err == nil
	restore := func() error {
		if hadPrevious {
			if err := os.Rename(previous, target); err != nil {
				return fmt.Errorf("restore agent bundle from %s: %w", previous, err)
			}
		}
		return os.Remove(backup)
	}
	if err := os.Rename(incoming, target); err != nil {
		return nil, errors.Join(err, restore())
	}
	return func(commit bool) error {
		if commit {
			return os.RemoveAll(backup)
		}
		if err := os.Rename(target, incoming); err != nil {
			return fmt.Errorf("roll back agent bundle, previous bundle retained at %s: %w", previous, err)
		}
		return restore()
	}, nil
}

func validateReleaseAgents(folder string) error {
	bundle, err := loadAgentBundle(folder)
	if err != nil {
		return err
	}
	for _, arch := range []string{"amd64", "arm64"} {
		digest, err := verifyAgentArtifact(filepath.Join(folder, "s-ui-node-linux-"+arch))
		if err != nil {
			return err
		}
		if digest != bundle.Agents[arch] {
			return errors.New("agent bundle checksum mismatch")
		}
	}
	return nil
}
