package service

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
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
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{
		"s-ui/sui":          "bin",
		"s-ui/s-ui.sh":      "sh",
		"../sui":            "evil",
		"s-ui/../../x":      "evil",
		"other/s-ui.sh":     "evil",
		"s-ui/s-ui.service": "unit",
	} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	f.Close()

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

func TestReleaseTagPattern(t *testing.T) {
	for tag, want := range map[string]bool{"v1.6.7": true, "1.7.0": true, "v1.7.0-beta": false, "v1;rm": false, "": false} {
		if got := releaseTagPattern.MatchString(tag); got != want {
			t.Errorf("tag %q: got %v, want %v", tag, got, want)
		}
	}
}
