package service

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/nodes"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.6.7", "1.6.6", true},
		{"v1.7.0", "1.6.6", true},
		{"1.10.0", "1.9.9", true},
		{"1.6.6", "1.6.6", false},
		{"1.6.5", "1.6.6", false},
		{"1.6.6.1", "1.6.6", true},
		{"1.6", "1.6.0", false},
		{"1.7.0-beta", "1.6.6", false},
		{"", "1.6.6", false},
		{"1.6.7", "dev", false},
	}
	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aaa  s-ui-linux-amd64.tar.gz\nBBB *s-ui-linux-arm64.tar.gz\nccc  s-ui-windows-amd64.zip\n"
	if got := checksumFor(sums, "s-ui-linux-amd64.tar.gz"); got != "aaa" {
		t.Errorf("plain entry = %q", got)
	}
	if got := checksumFor(sums, "s-ui-linux-arm64.tar.gz"); got != "bbb" {
		t.Errorf("binary entry = %q", got)
	}
	if got := checksumFor(sums, "s-ui-linux-386.tar.gz"); got != "" {
		t.Errorf("missing entry = %q", got)
	}
}

func TestExtractReleaseIgnoresOtherPaths(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "r.tar.gz")
	writeTestRelease(t, archive, map[string]string{
		"s-ui/sui":          "bin",
		"s-ui/s-ui.sh":      "sh",
		"../sui":            "evil",
		"s-ui/../../x":      "evil",
		"other/s-ui.sh":     "evil",
		"s-ui/s-ui.service": "unit",
	})

	out := filepath.Join(dir, "out")
	os.Mkdir(out, 0700)
	if err := extractRelease(archive, out); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 3 {
		t.Fatalf("extracted %d files, want 3", len(entries))
	}
	if b, _ := os.ReadFile(filepath.Join(out, "sui")); string(b) != "bin" {
		t.Errorf("sui = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "s-ui.sh")); string(b) != "sh" {
		t.Errorf("s-ui.sh = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err == nil {
		t.Error("an entry escaped the destination")
	}
}

func TestReleaseAgentsInstallAndRollback(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "release.tar.gz")
	amd64, arm64 := "amd64 test agent", "arm64 test agent"
	amd64Sum, arm64Sum := sha256.Sum256([]byte(amd64)), sha256.Sum256([]byte(arm64))
	writeTestRelease(t, archive, map[string]string{
		"s-ui/sui":                                 "panel",
		"s-ui/agents/s-ui-node-linux-amd64":        amd64,
		"s-ui/agents/s-ui-node-linux-arm64":        arm64,
		"s-ui/agents/s-ui-node-linux-amd64.sha256": fmt.Sprintf("%x  s-ui-node-linux-amd64\n", amd64Sum),
		"s-ui/agents/s-ui-node-linux-arm64.sha256": fmt.Sprintf("%x  s-ui-node-linux-arm64\n", arm64Sum),
		"s-ui/agents/manifest.json":                fmt.Sprintf(`{"sing_box_version":"v9.9.9","agents":{"amd64":"%x","arm64":"%x"}}`, amd64Sum, arm64Sum),
		"s-ui/agents/unexpected":                   "ignored",
	})
	stage := filepath.Join(dir, "stage")
	requireNodeTestSuccess(t, os.Mkdir(stage, 0700))
	requireNodeTestSuccess(t, os.Mkdir(filepath.Join(dir, "agents"), 0755))
	requireNodeTestSuccess(t, extractRelease(archive, stage))
	oldFile := filepath.Join(dir, "agents", "s-ui-node-linux-amd64")
	requireNodeTestSuccess(t, os.WriteFile(oldFile, []byte("previous agent"), 0755))

	finish, err := nodes.InstallReleaseAgents(stage, dir)
	requireNodeTestSuccess(t, err)
	verifyFile := func(path, expected string) {
		t.Helper()
		body, err := os.ReadFile(path)
		requireNodeTestSuccess(t, err)
		if string(body) != expected {
			t.Fatalf("%s has %q, want %q", path, body, expected)
		}
	}
	verifyFile(oldFile, amd64)
	if _, err := os.Stat(filepath.Join(dir, "agents", "unexpected")); !os.IsNotExist(err) {
		t.Fatal("unexpected archive member was extracted")
	}
	requireNodeTestSuccess(t, finish(false))
	verifyFile(oldFile, "previous agent")
	requireNodeTestSuccess(t, os.WriteFile(filepath.Join(stage, "agents", "s-ui-node-linux-amd64"), []byte("corrupt"), 0755))
	if _, err := nodes.InstallReleaseAgents(stage, dir); err == nil {
		t.Fatal("corrupt bundle was installed")
	}
	verifyFile(oldFile, "previous agent")
	requireNodeTestSuccess(t, os.WriteFile(filepath.Join(stage, "agents", "s-ui-node-linux-amd64"), []byte(amd64), 0755))
	finish, err = nodes.InstallReleaseAgents(stage, dir)
	requireNodeTestSuccess(t, err)
	requireNodeTestSuccess(t, finish(true))
	verifyFile(oldFile, amd64)
}

func writeTestRelease(t *testing.T, archive string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(archive)
	requireNodeTestSuccess(t, err)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		requireNodeTestSuccess(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		requireNodeTestSuccess(t, err)
	}
	requireNodeTestSuccess(t, tw.Close())
	requireNodeTestSuccess(t, gz.Close())
	requireNodeTestSuccess(t, f.Close())
}

func TestReleaseTagPattern(t *testing.T) {
	for tag, want := range map[string]bool{"v1.6.7": true, "1.7.0": true, "v1.7.0-beta": false, "v1;rm": false, "": false} {
		if got := releaseTagPattern.MatchString(tag); got != want {
			t.Errorf("tag %q: got %v, want %v", tag, got, want)
		}
	}
}
