package nodeagent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type StateV2 struct {
	BootstrapID   string          `json:"bootstrap_id,omitempty"`
	NodeID        uint64          `json:"node_id"`
	Epoch         string          `json:"epoch"`
	Spec          *Structure      `json:"spec,omitempty"`
	Access        AccessSnapshot  `json:"access"`
	AppliedAccess *AccessSnapshot `json:"applied_access,omitempty"`
}

type StateStoreV2 interface {
	LoadV2() (*StateV2, error)
	SaveV2(StateV2) error
}
type FileStoreV2 struct{ Directory string }

// systemd DynamicUser exposes StateDirectory through an administrator-owned
// symlink. Resolve that CLI-selected root once; artifact APIs still reject
// symlinks below the canonical private root.
func CanonicalStateDirectory(directory string) (string, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	full, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(full)
}

func (s FileStoreV2) LoadV2() (*StateV2, error) {
	body, err := os.ReadFile(filepath.Join(s.Directory, "state-v2.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Version int     `json:"version"`
		State   StateV2 `json:"state"`
		Digest  string  `json:"digest"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Version != ProtocolV2 || envelope.Digest != digestV2(envelope.State) {
		return nil, errors.New("invalid v2 state checksum")
	}
	return &envelope.State, nil
}

func (s FileStoreV2) SaveV2(state StateV2) error {
	if err := os.MkdirAll(s.Directory, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.Directory, 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		Version int     `json:"version"`
		State   StateV2 `json:"state"`
		Digest  string  `json:"digest"`
	}{ProtocolV2, state, digestV2(state)})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Directory, ".state-v2-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0o600); err == nil {
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
	if err := os.Rename(name, filepath.Join(s.Directory, "state-v2.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.Directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
