package nodes

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"testing"
	"time"
)

func TestLiveProvisionCompressedArtifactAndInterruptedReceiver(t *testing.T) {
	body := bytes.Repeat([]byte("verified-node-artifact\n"), 100000)
	if err := compressedAgentStream(bytes.NewReader(body), func(input io.Reader) error {
		source, err := gzip.NewReader(input)
		if err != nil {
			return err
		}
		defer source.Close()
		actual, err := io.ReadAll(source)
		if err == nil && !bytes.Equal(actual, body) {
			t.Error("artifact changed in transport")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("receiver interrupted")
	done := make(chan error, 1)
	go func() { done <- compressedAgentStream(bytes.NewReader(body), func(io.Reader) error { return failure }) }()
	select {
	case err := <-done:
		if !errors.Is(err, failure) {
			t.Fatal("lost receiver error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("compression worker leaked after SSH failure")
	}
}
