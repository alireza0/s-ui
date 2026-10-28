package database

import (
	"bytes"
	"log"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util"
)

// Regression: a fresh database used to get a first admin account with the
// password "admin", so anyone who reached a new panel before its owner
// did owned it. The initial password must now be random, stored hashed,
// and the password printed to the log must be the one that was stored --
// it is the only copy.
func TestInitUserCreatesRandomAdminPassword(t *testing.T) {
	oldDb := db
	oldOut := log.Writer()
	defer func() {
		db = oldDb
		log.SetOutput(oldOut)
	}()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	if err := InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("first user: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("username = %q, want %q", user.Username, "admin")
	}
	if !util.IsHashedPassword(user.Password) {
		t.Fatal("initial password is not stored hashed")
	}
	if util.CheckPassword("admin", user.Password) {
		t.Fatal("initial password is still the old default")
	}

	m := regexp.MustCompile(`Password: (\S+)`).FindStringSubmatch(buf.String())
	if m == nil {
		t.Fatalf("initial password was not printed to the log; log was: %q", buf.String())
	}
	if !util.CheckPassword(m[1], user.Password) {
		t.Fatal("password printed to the log does not match the stored hash")
	}

	// Running it again must not reset or duplicate the account.
	if err := initUser(); err != nil {
		t.Fatalf("second initUser: %v", err)
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count = %d, want 1", count)
	}
}
