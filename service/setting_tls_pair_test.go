package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// The web and sub servers refuse to start when only one of a cert/key pair
// is set, but Save checked each path on its own, so a half pair went into
// the database without a word and only failed at the next restart -- with
// the panel down and no way back in from the panel. Save must reject it
// while the operator is still there to fix it.
func TestSaveRejectsHalfTlsPair(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	for _, f := range []string{certFile, keyFile} {
		if err := os.WriteFile(f, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	save := func(payload map[string]string) error {
		data, _ := json.Marshal(payload)
		return db.Transaction(func(tx *gorm.DB) error {
			return s.Save(tx, data)
		})
	}

	for _, pair := range [][2]string{
		{"webCertFile", "webKeyFile"},
		{"subCertFile", "subKeyFile"},
	} {
		for _, tc := range []struct {
			name    string
			payload map[string]string
			missing string
		}{
			{"cert only", map[string]string{pair[0]: certFile}, pair[1]},
			{"key only", map[string]string{pair[1]: keyFile}, pair[0]},
		} {
			t.Run(pair[0]+"/"+tc.name, func(t *testing.T) {
				err := save(tc.payload)
				if err == nil {
					t.Fatalf("a half pair was saved: %v", tc.payload)
				}
				if !strings.Contains(err.Error(), tc.missing) {
					t.Errorf("error does not name the missing setting %s: %v", tc.missing, err)
				}
				for _, key := range pair {
					if got, _ := s.getString(key); got != "" {
						t.Errorf("%s = %q after a rejected save, want empty", key, got)
					}
				}
			})
		}

		// Both halves together are the legitimate save, and clearing both
		// is how the operator turns HTTPS off again.
		if err := save(map[string]string{pair[0]: certFile, pair[1]: keyFile}); err != nil {
			t.Fatalf("saving a full pair: %v", err)
		}
		if err := save(map[string]string{pair[0]: "", pair[1]: ""}); err != nil {
			t.Fatalf("clearing a pair: %v", err)
		}
	}

	// A save that touches neither key of a pair leaves it alone, even a
	// half pair that predates this check: unrelated saves must not be held
	// hostage to it.
	if err := save(map[string]string{"subCertFile": certFile}); err == nil {
		t.Fatal("seeding a half pair should have been rejected")
	}
	if err := db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "subCertFile", certFile).Error; err != nil {
		t.Fatalf("seeding a pre-existing half pair: %v", err)
	}
	if err := save(map[string]string{"subUpdates": "24"}); err != nil {
		t.Fatalf("an unrelated save was rejected over an untouched pair: %v", err)
	}
}
