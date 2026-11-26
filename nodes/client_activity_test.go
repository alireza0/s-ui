package nodes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

func TestRemoteActivityUpdatesOnlyAssignedClientAndNeverQuota(t *testing.T) {
	db, node, client := fixture(t)
	node.AssignmentMode = model.NodeAssignmentWhole
	node.AgentProtocol = 2
	node.ControlEpoch = "fixture-epoch"
	node.ManagementGeneration = 1
	if err := db.Save(&node).Error; err != nil {
		t.Fatal(err)
	}
	client.Config = json.RawMessage(`{"http":{"username":"auth-first","password":"fixture"}}`)
	client.Up, client.Down = 10, 20
	if err := db.Save(&client).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientNode{ClientId: client.Id, NodeId: node.Id, Active: true, Generation: 1}).Error; err != nil {
		t.Fatal(err)
	}
	other := model.Client{Name: "unassigned", Enable: true}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(2 * time.Minute)
	stamp := now.Unix() - 10
	sample := &nodeagent.TrafficSample{Up: 1000, Down: 2000, Activity: map[string]int64{effectiveUserName(db, node, client): stamp, "auth-first": stamp - 2, userName(other): stamp}}
	if err := recordClientActivity(db, node, sample, now); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&client, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	db.First(&other, other.Id)
	if client.OnlineAt != stamp || other.OnlineAt != 0 || client.Up != 10 || client.Down != 20 {
		t.Fatalf("wrong activity or quota: %d/%d/%d/%d", client.OnlineAt, other.OnlineAt, client.Up, client.Down)
	}
	sample.Activity = map[string]int64{"auth-first": stamp - 5}
	if err := recordClientActivity(db, node, sample, now); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.OnlineAt != stamp {
		t.Fatal("late node sample moved Last online backwards")
	}
	stale := node
	stale.ManagementGeneration--
	sample.Activity = map[string]int64{"auth-first": stamp + 1}
	if err := recordClientActivity(db, stale, sample, now); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.OnlineAt != stamp {
		t.Fatal("stale registration changed activity")
	}
	sample.Activity = map[string]int64{"auth-first": now.Add(time.Hour).Unix()}
	if err := recordClientActivity(db, node, sample, now); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.OnlineAt != stamp {
		t.Fatal("future activity timestamp accepted")
	}
	if err := db.Model(&model.ClientNode{}).Where("client_id = ? AND node_id = ?", client.Id, node.Id).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	sample.Activity = map[string]int64{"auth-first": stamp + 2}
	if err := recordClientActivity(db, node, sample, now); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.OnlineAt != stamp {
		t.Fatal("removed assignment still reported activity")
	}
}
