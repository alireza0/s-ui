package nodes

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func TestClientQuotaSumsConcurrentNodesAndLocalTrafficWithoutReplay(t *testing.T) {
	db, first, client := fixture(t)
	first.AssignmentMode = model.NodeAssignmentWhole
	if err := db.Save(&first).Error; err != nil {
		t.Fatal(err)
	}
	second := first
	second.Id = 2
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	for _, n := range []model.Node{first, second} {
		if err := db.Create(&model.ClientNode{NodeId: n.Id, ClientId: client.Id, Active: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	client.Up, client.Down = 10, 20 // Traffic already charged by local S-UI.
	if err := db.Save(&client).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	key := clientTrafficID(client)
	a := &nodeagent.TrafficSample{Process: "11111111-1111-4111-8111-111111111111", Users: map[string]nodeagent.UserTraffic{key: {Up: 100, Down: 200, At: now.Unix()}}}
	b := &nodeagent.TrafficSample{Process: "22222222-2222-4222-8222-222222222222", Users: map[string]nodeagent.UserTraffic{key: {Up: 300, Down: 400, At: now.Unix()}}}
	var work sync.WaitGroup
	for range 8 {
		for i, sample := range []*nodeagent.TrafficSample{a, b} {
			work.Add(1)
			go func() {
				defer work.Done()
				n := first
				if i == 1 {
					n = second
				}
				if err := recordClientTraffic(db, n, sample, now); err != nil {
					t.Error(err)
				}
			}()
		}
	}
	work.Wait()
	if err := db.First(&client, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	if client.Up != 410 || client.Down != 620 || client.OnlineAt != now.Unix() {
		t.Fatalf("wrong combined usage: %+v", client)
	}
	// A late, smaller sample cannot roll the cursor back and be charged twice.
	a.Users[key] = nodeagent.UserTraffic{Up: 90, Down: 190}
	if err := recordClientTraffic(db, first, a, now); err != nil {
		t.Fatal(err)
	}
	a.Users[key] = nodeagent.UserTraffic{Up: 100, Down: 200}
	if err := recordClientTraffic(db, first, a, now); err != nil {
		t.Fatal(err)
	}
	// Resetting the quota preserves both node cursors, including after reconnect.
	if err := db.Model(&client).Updates(map[string]any{"up": 0, "down": 0}).Error; err != nil {
		t.Fatal(err)
	}
	if err := recordClientTraffic(db, first, a, now); err != nil {
		t.Fatal(err)
	}
	a.Users[key] = nodeagent.UserTraffic{Up: 110, Down: 230}
	if err := recordClientTraffic(db, first, a, now); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.Up != 10 || client.Down != 30 {
		t.Fatal("reset replayed previously charged bytes", client.Up, client.Down)
	}
}

func TestClientTrafficRejectsForeignAndStaleNodeIdentities(t *testing.T) {
	db, n, client := fixture(t)
	n.AssignmentMode = model.NodeAssignmentWhole
	n.ManagementGeneration = 2
	db.Save(&n)
	db.Create(&model.ClientNode{NodeId: n.Id, ClientId: client.Id, Active: false})
	other := model.Client{Name: "unassigned"}
	db.Create(&other)
	s := &nodeagent.TrafficSample{Process: "11111111-1111-4111-8111-111111111111", Users: map[string]nodeagent.UserTraffic{
		clientTrafficID(client): {Up: 100, Down: 200}, clientTrafficID(other): {Up: 1000, Down: 2000},
	}}
	stale := n
	stale.ManagementGeneration--
	if err := recordClientTraffic(db, stale, s, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.Up != 0 {
		t.Fatal("stale node charged usage")
	}
	if err := recordClientTraffic(db, n, s, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	db.First(&other, other.Id)
	if client.Up != 100 || client.Down != 200 || other.Up != 0 {
		t.Fatal("wrong final revoked or foreign usage")
	}
	s.Process = "not-a-ledger"
	s.Users[clientTrafficID(client)] = nodeagent.UserTraffic{Up: 500, Down: 500}
	if err := recordClientTraffic(db, n, s, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.Up != 100 {
		t.Fatal("invalid ledger charged usage")
	}
}

func TestClientTrafficRollbackDoesNotConsumeTheCursor(t *testing.T) {
	db, n, client := fixture(t)
	addFixtureAssignment(t, db, n.Id)
	sample := &nodeagent.TrafficSample{Process: "11111111-1111-4111-8111-111111111111", Users: map[string]nodeagent.UserTraffic{clientTrafficID(client): {Up: 100, Down: 200}}}
	callback := "fixture-reject-client-usage"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "clients" {
			tx.AddError(errors.New("fixture database failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := recordClientTraffic(db, n, sample, time.Now())
	db.Callback().Update().Remove(callback)
	if err == nil {
		t.Fatal("injected database error was ignored")
	}
	var count int64
	db.Model(&model.NodeClientTraffic{}).Count(&count)
	if count != 0 {
		t.Fatal("failed client update consumed the traffic cursor")
	}
	if err := recordClientTraffic(db, n, sample, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.First(&client, client.Id)
	if client.Up != 100 || client.Down != 200 {
		t.Fatal("retry did not charge exactly once", client.Up, client.Down)
	}
}
