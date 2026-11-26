package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/op/go-logging"
	"gorm.io/gorm"
)

// The normal suite skips this executable check. Isolated server checks set the
// path to their freshly built agent; no download/install or external VPS access.
func TestLiveAgentProcessesSurvivePanelReopenAndAgentCrash(t *testing.T) {
	binary := os.Getenv("SUI_TEST_AGENT_BINARY")
	if binary == "" {
		t.Skip("requires explicitly selected agent binary")
	}
	logger.InitLogger(logging.CRITICAL)
	db, node, client := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	requireNodeTestSuccess(t, db.Where("node_id = ?", node.Id).Delete(&model.NodeInbound{}).Error)
	f := liveFixture{db: db, client: client}
	restarts := []func(){}
	for i := 0; i < 2; i++ {
		n, restart := liveProcessNodeFixture(t, db, binary, client.Id, i)
		restarts = append(restarts, restart)
		f.nodes = append(f.nodes, n)
	}
	f.save(t, liveSpec())
	for i := range f.nodes {
		f.bind(t, i, f.template.Id)
	}
	ReconcileAll(context.Background())
	if f.links(t) != 2 {
		t.Fatal("process agents did not apply initial template")
	}
	spec := liveSpec()
	extra := spec.Entries[0]
	extra.Key, extra.Name = "extra", "Extra"
	spec.Entries = append(spec.Entries, extra)
	f.save(t, spec)
	ReconcileAll(context.Background())
	links, err := ClientInbounds(f.db, f.client)
	if err != nil || len(links) != 4 {
		t.Fatal("process agents did not fan out", err)
	}
	assertLiveTraffic(t, links)
	var inbounds []model.NodeInbound
	f.db.Order("id").Find(&inbounds)
	before, _ := json.Marshal(inbounds)
	var files []struct{ Name, File string }
	if err := db.Raw("PRAGMA database_list").Scan(&files).Error; err != nil || len(files) == 0 {
		t.Fatal("database path missing", err)
	}
	var path string
	for _, file := range files {
		if file.Name == "main" {
			path = file.File
		}
	}
	sqlDB, err := db.DB()
	requireNodeTestSuccess(t, err)
	requireNodeTestSuccess(t, sqlDB.Close())
	requireNodeTestSuccess(t, database.InitDB(path))
	f.db = database.GetDB()
	reopened, _ := f.db.DB()
	t.Cleanup(func() { reopened.Close() })
	restarts[0]() // SIGKILL only the child started by this test; state must recover.
	ReconcileAll(context.Background())
	links, err = ClientInbounds(f.db, f.client)
	if err != nil || len(links) != 4 {
		t.Fatal("restart lost confirmed publication", err)
	}
	assertLiveTraffic(t, links)
	var after []model.NodeInbound
	f.db.Order("id").Find(&after)
	body, _ := json.Marshal(after)
	if string(before) != string(body) {
		t.Fatal("panel reopen/agent crash changed materialized identities")
	}
}

func liveProcessNodeFixture(t *testing.T, db *gorm.DB, binary string, clientID uint, i int) (model.Node, func()) {
	t.Helper()
	placeholder, identity := controlHandlerServer(t, http.NotFoundHandler())
	address := placeholder.URL
	listen := placeholder.Listener.Addr().String()
	placeholder.Close()
	state := t.TempDir()
	for name, value := range map[string]string{"server.pem": identity.Certificate, "server-key.pem": identity.Key, "ca.pem": identity.CA} {
		requireNodeTestSuccess(t, os.WriteFile(filepath.Join(state, name), []byte(value), 0600))
	}
	n := model.Node{Id: uint(i + 1), Name: fmt.Sprintf("Process %d", i+1), Enable: true, AssignmentMode: model.NodeAssignmentWhole, AgentProtocol: 2, Address: address, LinkAddress: fmt.Sprintf("node-%d.example", i+1), ControlCA: identity.CA, ControlCertificate: identity.Certificate, ControlKey: identity.Key}
	requireNodeTestSuccess(t, db.Save(&n).Error)
	requireNodeTestSuccess(t, db.Create(&model.ClientNode{NodeId: n.Id, ClientId: clientID, Active: true, Generation: 1}).Error)
	var command *exec.Cmd
	stop := func() {
		if command != nil && command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}
	t.Cleanup(stop)
	start := func() {
		log, err := os.OpenFile(filepath.Join(state, "agent.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		requireNodeTestSuccess(t, err)
		command = exec.Command(binary, "--node-id", strconv.Itoa(int(n.Id)), "--state", state, "--listen", listen, "--tls-cert", filepath.Join(state, "server.pem"), "--tls-key", filepath.Join(state, "server-key.pem"), "--client-ca", filepath.Join(state, "ca.pem"))
		command.Stdout, command.Stderr = log, log
		if err := command.Start(); err != nil {
			log.Close()
			t.Fatal(err)
		}
		log.Close()
		var status nodeagent.StatusV2
		ready := false
		for attempt := 0; attempt < 60; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			control, closeControl, err := controlClient(ctx, n)
			if err == nil {
				status, err = getStatusV2(ctx, control, n)
				closeControl()
			}
			cancel()
			if err == nil {
				ready = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			stop()
			t.Fatal("agent process not ready; inspect private test log")
		}
		n.ControlEpoch = status.Epoch
		requireNodeTestSuccess(t, db.Model(&n).Update("control_epoch", status.Epoch).Error)
	}
	start()
	return n, func() { stop(); start() }
}
