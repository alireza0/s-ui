package nodeagent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid/v5"
)

// Keep the ledger identity and cumulative counters across agent restarts.
// A status sample is published only after its counters reach durable storage.
type trafficEnvelope struct {
	Version int           `json:"version"`
	Sample  TrafficSample `json:"sample"`
	Digest  string        `json:"digest"`
}

func (r *CoreRuntime) loadTraffic() error {
	body, err := os.ReadFile(filepath.Join(r.stateDirectory, "traffic.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var data trafficEnvelope
	if json.Unmarshal(body, &data) != nil || data.Version != 1 || data.Digest != digestV2(data.Sample) {
		return errors.New("invalid traffic ledger")
	}
	if id, err := uuid.FromString(data.Sample.Process); err != nil || id == uuid.Nil {
		return errors.New("invalid traffic ledger identity")
	}
	r.traffic = data.Sample
	return nil
}

func (r *CoreRuntime) persistTraffic() (err error) {
	defer func() { r.trafficWriteError = err }()
	if r.trafficLoadError != nil {
		return r.trafficLoadError
	}
	if r.traffic.Process == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		r.traffic.Process = id.String()
		r.trafficDirty = true
	}
	if r.stateDirectory == "" || !r.trafficDirty {
		return nil
	}
	if err := os.MkdirAll(r.stateDirectory, 0700); err != nil {
		return err
	}
	body, err := json.Marshal(trafficEnvelope{1, r.traffic, digestV2(r.traffic)})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(r.stateDirectory, ".traffic-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(body); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, filepath.Join(r.stateDirectory, "traffic.json")); err != nil {
		return err
	}
	dir, err := os.Open(r.stateDirectory)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return err
	}
	r.trafficDirty = false
	return nil
}
