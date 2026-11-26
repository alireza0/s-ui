package database

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alireza0/s-ui/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var backupImportMu sync.Mutex

const (
	backupDatabaseName = "panel.db"
	backupManifestName = "manifest.json"
	backupLimit        = 256 << 20
	backupKeyName      = "node-provision.key"
)

type nodeBackupManifest struct {
	Version        int    `json:"version"`
	DatabaseSHA256 string `json:"database_sha256"`
	KeySHA256      string `json:"key_sha256,omitempty"`
}

func backupHash(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }

func readOptionalKey(root *os.Root) ([]byte, error) {
	body, err := root.ReadFile(backupKeyName)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(body) != 32 {
		return nil, errors.New("invalid_node_backup_key")
	}
	return body, nil
}

// The list is private cipher state, not arbitrary fields supplied by an archive.
var nodeCipherColumns = map[string][]string{
	"node_installations":       {"private_key", "server_certificate", "server_key"},
	"node_generated_materials": {"encrypted_value"},
	"node_dns_profiles":        {"secret"}, "node_certificate_bindings": {"material", "account_key", "desired_request"},
	"node_certificate_versions": {"bundle"}, "node_jobs": {"plan"},
}

func validateNodeBackupKey(path string, key []byte) error {
	candidate, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(path)+"?mode=ro"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := candidate.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	aead, err := nodeBackupCipher(key)
	if err != nil {
		return err
	}
	for table, columns := range nodeCipherColumns {
		if !candidate.Migrator().HasTable(table) {
			continue
		}
		for _, column := range columns {
			if !candidate.Migrator().HasColumn(table, column) {
				continue
			}
			if err := validateBackupCipherColumn(candidate, table, column, aead); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBackupBytes(body, key []byte) error {
	files, err := openRestoreFiles(config.GetDBPath())
	if err != nil {
		return err
	}
	defer files.root.Close()
	name := ".backup-key-check-" + rand.Text() + ".db"
	f, err := files.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer files.root.Remove(name)
	defer files.removeSidecars(name)
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	fullPath := files.path(name)
	if err = validateImport(fullPath); err != nil {
		return err
	}
	return validateNodeBackupKey(fullPath, key)
}

func GetFullBackup(exclude string) ([]byte, error) {
	backupImportMu.Lock()
	defer backupImportMu.Unlock()
	for _, name := range strings.Split(exclude, ",") {
		name = strings.TrimSpace(name)
		if name != "" && name != "stats" && name != "changes" {
			return nil, errors.New("incomplete_node_backup_exclusions")
		}
	}
	root, err := os.OpenRoot(config.GetDBFolderPath())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	key, err := readOptionalKey(root)
	if err != nil {
		return nil, err
	}
	body, err := GetDb(exclude)
	if err != nil {
		return nil, err
	}
	after, err := readOptionalKey(root)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(key, after) {
		return nil, errors.New("node_backup_key_changed")
	}
	if err = validateBackupBytes(body, key); err != nil {
		return nil, err
	}
	manifest := nodeBackupManifest{Version: 1, DatabaseSHA256: backupHash(body)}
	if len(key) > 0 {
		manifest.KeySHA256 = backupHash(key)
	}
	encoded, _ := json.Marshal(manifest)
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	files := map[string][]byte{backupDatabaseName: body, backupManifestName: encoded}
	if len(key) > 0 {
		files[backupKeyName] = key
	}
	for _, name := range []string{backupManifestName, backupDatabaseName, backupKeyName} {
		data, exists := files[name]
		if !exists {
			continue
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		file, e := archive.CreateHeader(h)
		if e != nil {
			return nil, e
		}
		if _, e = file.Write(data); e != nil {
			return nil, e
		}
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func readFullBackup(reader io.ReaderAt, size int64) ([]byte, []byte, error) {
	if size <= 0 || size > backupLimit {
		return nil, nil, errors.New("invalid_node_backup_archive")
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, nil, errors.New("invalid_node_backup_archive")
	}
	files, err := readBackupArchiveFiles(archive)
	if err != nil {
		return nil, nil, err
	}
	var manifest nodeBackupManifest
	if json.Unmarshal(files[backupManifestName], &manifest) != nil || manifest.Version != 1 || manifest.DatabaseSHA256 != backupHash(files[backupDatabaseName]) {
		return nil, nil, errors.New("invalid_node_backup_manifest")
	}
	key := files[backupKeyName]
	if len(key) > 0 && (len(key) != 32 || manifest.KeySHA256 != backupHash(key)) {
		return nil, nil, errors.New("node_backup_key_mismatch")
	}
	if manifest.KeySHA256 != "" && len(key) == 0 {
		return nil, nil, errors.New("node_backup_key_required")
	}
	if err = validateBackupBytes(files[backupDatabaseName], key); err != nil {
		return nil, nil, err
	}
	return files[backupDatabaseName], key, nil
}

// A prepared pair is rolled back after a crash; a ready pair has both new
// files durably installed. Startup never opens a half-restored DB/key pair.
type backupPairJournal struct {
	Version  int    `json:"version"`
	Fallback string `json:"fallback"`
	OldKey   bool   `json:"old_key"`
	Ready    bool   `json:"ready"`
}

type restoreFiles struct {
	root     *os.Root
	database string
}

func openRestoreFiles(dbPath string) (restoreFiles, error) {
	root, err := os.OpenRoot(filepath.Dir(dbPath))
	return restoreFiles{root: root, database: filepath.Base(dbPath)}, err
}

// SQLite's driver takes a filename. Its callers supply names allocated by the
// restore code in the configured database directory.
func (files restoreFiles) path(name string) string {
	return filepath.Join(files.root.Name(), name)
}

func (files restoreFiles) journalName() string { return files.database + ".restore-pair.json" }

func (files restoreFiles) writeJournal(j backupPairJournal) error {
	body, _ := json.Marshal(j)
	return files.writePrivate(files.journalName(), body)
}

func (files restoreFiles) writePrivate(name string, body []byte) error {
	temp := ".restore-write-" + rand.Text()
	f, err := files.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer files.root.Remove(temp)
	_, err = f.Write(body)
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
	if err = files.root.Rename(temp, name); err != nil {
		return err
	}
	folder, err := files.root.Open(".")
	if err != nil {
		return err
	}
	defer folder.Close()
	return folder.Sync()
}

func recoverBackupPair(dbPath string) error {
	files, err := openRestoreFiles(dbPath)
	if err != nil {
		return err
	}
	defer files.root.Close()
	return files.recoverPair()
}

func (files restoreFiles) recoverPair() error {
	journalName := files.journalName()
	body, err := files.root.ReadFile(journalName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal backupPairJournal
	if json.Unmarshal(body, &journal) != nil || journal.Version != 1 || !filepath.IsLocal(journal.Fallback) || filepath.Base(journal.Fallback) != journal.Fallback || !strings.HasPrefix(journal.Fallback, files.database+".backup-") {
		return errors.New("invalid_restore_pair_journal")
	}
	if journal.Ready {
		key, err := readOptionalKey(files.root)
		if err != nil {
			return err
		}
		if err = validateNodeBackupKey(files.path(files.database), key); err != nil {
			return err
		}
	} else if err := files.restorePair(journal); err != nil {
		return err
	}
	return files.root.Remove(journalName)
}

func (files restoreFiles) restorePair(journal backupPairJournal) error {
	_, err := files.root.Lstat(journal.Fallback)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Restore the key first, leaving the previous DB available until both
	// files can be restored. A failed key read or write remains retryable.
	if journal.OldKey {
		key, err := files.root.ReadFile(journal.Fallback + ".key")
		if err != nil {
			return err
		}
		if err = files.writePrivate(backupKeyName, key); err != nil {
			return err
		}
	} else if err = files.root.Remove(backupKeyName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files.removeSidecars(files.database)
	return files.root.Rename(journal.Fallback, files.database)
}

type backupMemoryFile struct{ *bytes.Reader }

func (backupMemoryFile) Close() error { return nil }

func backupReadSource() (*gorm.DB, func(), error) {
	var files []struct {
		Name string
		File string
	}
	if err := db.Raw("PRAGMA database_list").Scan(&files).Error; err != nil {
		return nil, nil, err
	}
	for _, file := range files {
		if file.Name == "main" && file.File != "" {
			source, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(file.File)+"?mode=ro&_busy_timeout=10000&_txlock=deferred"), &gorm.Config{Logger: gormlogger.Discard})
			if err != nil {
				return nil, nil, err
			}
			pool, err := source.DB()
			if err != nil {
				return nil, nil, err
			}
			return source, func() { pool.Close() }, nil
		}
	}
	return db, func() { /* The existing database is borrowed; its owner closes it. */ }, nil
}

func copyBackupSequences(source, target *gorm.DB) error {
	if !source.Migrator().HasTable("sqlite_sequence") {
		return nil
	}
	var rows []struct {
		Name string
		Seq  int64
	}
	if err := source.Raw("SELECT name,seq FROM sqlite_sequence").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if !target.Migrator().HasTable(row.Name) {
			continue
		}
		result := target.Exec("UPDATE sqlite_sequence SET seq=MAX(seq,?) WHERE name=?", row.Seq, row.Name)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := target.Exec("INSERT INTO sqlite_sequence(name,seq) VALUES(?,?)", row.Name, row.Seq).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func nodeBackupCipher(key []byte) (cipher.AEAD, error) {
	if len(key) == 0 {
		return nil, nil
	}
	if len(key) != 32 {
		return nil, errors.New("invalid_node_backup_key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func validateBackupCipherColumn(candidate *gorm.DB, table, column string, aead cipher.AEAD) error {
	rows, err := candidate.Raw(fmt.Sprintf(`SELECT "%s" FROM "%s" WHERE "%s" <> ''`, column, table, column)).Rows()
	if err != nil {
		return err
	}
	for rows.Next() {
		var sealed string
		if err = rows.Scan(&sealed); err != nil {
			rows.Close()
			return err
		}
		if aead == nil {
			rows.Close()
			return errors.New("node_backup_key_required")
		}
		body, e := base64.StdEncoding.DecodeString(sealed)
		if e != nil || len(body) < aead.NonceSize() {
			rows.Close()
			return errors.New("node_backup_key_mismatch")
		}
		if _, e = aead.Open(nil, body[:aead.NonceSize()], body[aead.NonceSize():], []byte("s-ui-node-provision-v1")); e != nil {
			rows.Close()
			return errors.New("node_backup_key_mismatch")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	return nil
}

func readBackupArchiveFiles(archive *zip.Reader) (map[string][]byte, error) {
	files := map[string][]byte{}
	var total uint64
	for _, file := range archive.File {
		if err := readBackupArchiveFile(file, files, &total); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func readBackupArchiveFile(file *zip.File, files map[string][]byte, total *uint64) error {
	if file.Name != backupDatabaseName && file.Name != backupKeyName && file.Name != backupManifestName {
		return errors.New("invalid_node_backup_archive")
	}
	if _, exists := files[file.Name]; exists {
		return errors.New("invalid_node_backup_archive")
	}
	limit := uint64(backupLimit)
	if file.Name == backupKeyName {
		limit = 32
	}
	if file.Name == backupManifestName {
		limit = 4096
	}
	*total += file.UncompressedSize64
	if file.UncompressedSize64 > limit || *total > backupLimit {
		return errors.New("invalid_node_backup_archive")
	}
	stream, e := file.Open()
	if e != nil {
		return e
	}
	body, e := io.ReadAll(io.LimitReader(stream, int64(limit)+1))
	stream.Close()
	if e != nil || uint64(len(body)) > limit {
		return errors.New("invalid_node_backup_archive")
	}
	files[file.Name] = body
	return nil
}
