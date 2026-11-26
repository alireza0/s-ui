package database

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func nodeBackupStateFixture(t *testing.T) (model.Node, []byte) {
	t.Helper()
	_, key := nodeBackupFixture(t)
	node := model.Node{Id: 1, Name: "Fixture node", AgentProtocol: 2, ControlEpoch: "fixture-epoch", ControlCA: "fixture-ca", ControlCertificate: "fixture-controller", SettingsVersion: 1}
	requireNodeTestSuccess(t, db.Create(&model.Setting{Key: "version", Value: config.GetVersion()}).Error)
	requireNodeTestSuccess(t, db.Create(&node).Error)
	requireNodeTestSuccess(t, db.Create(&model.NodeAccessState{NodeId: node.Id, DesiredRevision: 2, AppliedRevision: 2, DesiredSnapshot: json.RawMessage(`{"revision":2,"clients":[]}`)}).Error)
	requireNodeTestSuccess(t, db.Create(&model.NodeCertificateBinding{NodeId: node.Id, DesiredRevision: 1, AppliedRevision: 1, DesiredFingerprint: "fixture-request"}).Error)
	return node, key
}

func openNodeArchive(t *testing.T, archive []byte) *gorm.DB {
	t.Helper()
	body, _, err := readFullBackup(bytes.NewReader(archive), int64(len(archive)))
	requireNodeTestSuccess(t, err)
	return openBackup(t, body)
}

func TestSuccessiveBackupsIncludeLatestPendingNodeChanges(t *testing.T) {
	node, _ := nodeBackupStateFixture(t)
	first, err := GetFullBackup("stats,changes")
	requireNodeTestSuccess(t, err)
	requireNodeTestSuccess(t, db.Model(&node).Updates(map[string]any{"settings_version": 2, "template_target_version": 4, "desired_revision": 2, "applied_revision": 1, "desired_snapshot": json.RawMessage(`{"revision":2,"inbounds":[]}`)}).Error)
	requireNodeTestSuccess(t, db.Model(&model.NodeAccessState{}).Where("node_id = ?", node.Id).Updates(map[string]any{"desired_revision": 3, "desired_snapshot": json.RawMessage(`{"revision":3,"clients":[]}`)}).Error)
	requireNodeTestSuccess(t, db.Create(&model.NodeTemplate{Id: 1, Name: "Updated fixture", Version: 4}).Error)
	requireNodeTestSuccess(t, db.Create(&model.NodeDomainBinding{NodeId: node.Id, Mode: "manual", FQDN: "updated.example.com", Generation: 2}).Error)
	requireNodeTestSuccess(t, db.Create(&model.NodeClientTraffic{NodeId: node.Id, ClientId: 1, Ledger: "fixture-ledger", Up: 123, Down: 456}).Error)
	requireNodeTestSuccess(t, db.Model(&model.NodeCertificateBinding{}).Where("node_id = ?", node.Id).Update("desired_revision", 2).Error)
	latest, err := GetFullBackup("stats,changes")
	requireNodeTestSuccess(t, err)
	old := openNodeArchive(t, first)
	var original model.Node
	requireNodeTestSuccess(t, old.First(&original, node.Id).Error)
	if original.SettingsVersion != 1 {
		t.Fatal("changing live state modified an earlier backup")
	}
	checkLatestNodeBackup(t, openNodeArchive(t, latest), node.Id)
	plain, err := GetDb("stats,changes")
	requireNodeTestSuccess(t, err)
	checkLatestNodeBackup(t, openBackup(t, plain), node.Id)
}

func checkLatestNodeBackup(t *testing.T, candidate *gorm.DB, nodeID uint) {
	t.Helper()
	var node model.Node
	var access model.NodeAccessState
	var template model.NodeTemplate
	var domain model.NodeDomainBinding
	var traffic model.NodeClientTraffic
	var certificate model.NodeCertificateBinding
	var version model.Setting
	requireNodeTestSuccess(t, candidate.First(&node, nodeID).Error)
	requireNodeTestSuccess(t, candidate.First(&access, "node_id = ?", nodeID).Error)
	requireNodeTestSuccess(t, candidate.First(&template, 1).Error)
	requireNodeTestSuccess(t, candidate.First(&domain, "node_id = ?", nodeID).Error)
	requireNodeTestSuccess(t, candidate.First(&traffic, "node_id = ?", nodeID).Error)
	requireNodeTestSuccess(t, candidate.First(&certificate, "node_id = ?", nodeID).Error)
	requireNodeTestSuccess(t, candidate.First(&version, "key = ?", "version").Error)
	if version.Value != config.GetVersion() {
		t.Fatal("new backup retained an outdated database version")
	}
	if node.SettingsVersion != 2 || node.TemplateTargetVersion != 4 || template.Version != 4 || access.DesiredRevision != 3 || access.AppliedRevision != 2 || node.DesiredRevision != 2 || node.AppliedRevision != 1 {
		t.Fatal("new backup omitted pending settings or access changes")
	}
	if !bytes.Equal(access.DesiredSnapshot, []byte(`{"revision":3,"clients":[]}`)) {
		t.Fatal("new backup retained an outdated access payload")
	}
	if domain.FQDN != "updated.example.com" || domain.Generation != 2 || traffic.Up != 123 || traffic.Down != 456 {
		t.Fatal("new backup omitted current resources or accounting cursors")
	}
	if certificate.DesiredRevision != 2 || certificate.AppliedRevision != 1 {
		t.Fatal("new backup omitted the pending certificate revision")
	}
}

func TestRestoreAcceptsArchivedNodeStateAfterLaterChanges(t *testing.T) {
	for _, format := range []string{"sqlite", "zip"} {
		t.Run(format, func(t *testing.T) {
			checkArchivedNodeStateRestore(t, format)
		})
	}
}

func checkArchivedNodeStateRestore(t *testing.T, format string) {
	t.Helper()
	node, _ := nodeBackupStateFixture(t)
	task := model.NodeTask{NodeId: node.Id, Kind: "access", Generation: 3, LeaseToken: "archived-lease", LeaseUntil: time.Now().Unix() + 600, State: "working"}
	requireNodeTestSuccess(t, db.Create(&task).Error)
	export := GetDb
	if format == "zip" {
		export = GetFullBackup
	}
	archive, err := export("")
	requireNodeTestSuccess(t, err)
	requireNodeTestSuccess(t, db.Model(&node).Updates(map[string]any{"settings_version": 9, "desired_revision": 9, "control_epoch": "later-fixture-epoch"}).Error)
	requireNodeTestSuccess(t, db.Model(&model.NodeAccessState{}).Where("node_id = ?", node.Id).Update("desired_revision", 9).Error)
	requireNodeTestSuccess(t, restoreDB(backupMemoryFile{bytes.NewReader(archive)}))
	checkRestoredNodeBackup(t, node)
	requireNodeTestSuccess(t, db.First(&task, task.Id).Error)
	if task.LeaseUntil != 0 || task.LeaseToken != "" || task.Generation != 4 || task.State != "queued" {
		t.Fatal("restore retained an orphaned worker lease")
	}
}

func checkRestoredNodeBackup(t *testing.T, expected model.Node) {
	t.Helper()
	var restored model.Node
	var access model.NodeAccessState
	requireNodeTestSuccess(t, db.First(&restored, expected.Id).Error)
	requireNodeTestSuccess(t, db.First(&access, "node_id = ?", expected.Id).Error)
	if restored.SettingsVersion != 1 || restored.ControlEpoch != "fixture-epoch" || access.DesiredRevision != 2 {
		t.Fatal("restore did not preserve the archived node state")
	}
	if !restored.RestoreAccessPending || !restored.RestoreCertificatePending {
		t.Fatal("native restore did not schedule authenticated node reconciliation")
	}
}

func TestRestoreFullNodeBackupOnFreshPanel(t *testing.T) {
	for _, version := range []string{"1.6.7", config.GetVersion()} {
		t.Run(version, func(t *testing.T) {
			checkFullNodeBackupOnFreshPanel(t, version)
		})
	}
}

func checkFullNodeBackupOnFreshPanel(t *testing.T, version string) {
	t.Helper()
	node, key := nodeBackupStateFixture(t)
	requireNodeTestSuccess(t, db.Model(&model.Setting{}).Where("key = ?", "version").Update("value", version).Error)
	archive, err := GetFullBackup("")
	requireNodeTestSuccess(t, err)
	pool, err := db.DB()
	requireNodeTestSuccess(t, err)
	requireNodeTestSuccess(t, pool.Close())
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	requireNodeTestSuccess(t, InitDB(config.GetDBPath()))
	requireNodeTestSuccess(t, restoreDB(backupMemoryFile{bytes.NewReader(archive)}))
	checkRestoredNodeBackup(t, node)
	actual, err := os.ReadFile(filepath.Join(config.GetDBFolderPath(), backupKeyName))
	requireNodeTestSuccess(t, err)
	if !bytes.Equal(actual, key) {
		t.Fatal("fresh panel did not restore the archived node key")
	}
	var saved model.Setting
	requireNodeTestSuccess(t, db.First(&saved, "key = ?", "version").Error)
	if saved.Value != config.GetVersion() {
		t.Fatal("older backup did not migrate to the current panel version")
	}
}
