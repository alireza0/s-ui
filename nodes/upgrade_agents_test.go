package nodes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alireza0/s-ui/nodeagent"
)

func TestAgentUpgradeWaitsForActiveReconciliation(t *testing.T) {
	db, node, _ := fixture(t)
	node.AgentProtocol, node.ControlEpoch, node.TemplateTargetVersion = 2, "fixture-epoch", 1
	requireNodeTestSuccess(t, db.Save(&node).Error)
	value, _ := nodeLocks.LoadOrStore(node.Id, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	held := true
	t.Cleanup(func() {
		if held {
			lock.Unlock()
		}
	})
	done := make(chan struct{})
	go func() {
		upgradeManagedAgent(context.Background(), node.Id, agentBundle{})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("upgrade was silently dropped while reconciliation held the node")
	case <-time.After(30 * time.Millisecond):
	}
	lock.Unlock()
	held = false
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("upgrade did not resume after reconciliation released the node")
	}
	requireNodeTestSuccess(t, db.First(&node, node.Id).Error)
	if node.AgentUpgradeState != "waiting" {
		t.Fatal("upgrade did not reach the transport check after acquiring the node")
	}
}

func TestAgentUpgradeRequiresRunningIdentityAndMatchingAccess(t *testing.T) {
	before := nodeagent.StatusV2{NodeID: 1, Epoch: "epoch", SpecRevision: 2, SpecDigest: "spec", AccessRevision: 3, AccessDigest: "access"}
	after := before
	after.SingBoxVersion, after.AgentDigest, after.Running = "v1.14.4", "new-artifact", true
	after.AppliedAccessRevision, after.AppliedAccessDigest = 3, "access"
	if !matchesAgentUpgrade(before, after, "v1.14.4", "new-artifact") {
		t.Fatal("compatible update rejected")
	}
	for _, change := range []func(*nodeagent.StatusV2){
		func(s *nodeagent.StatusV2) { s.Running = false },
		func(s *nodeagent.StatusV2) { s.Epoch = "replaced" },
		func(s *nodeagent.StatusV2) { s.SpecDigest = "lost" },
		func(s *nodeagent.StatusV2) { s.AppliedAccessRevision-- },
		func(s *nodeagent.StatusV2) { s.AgentDigest = "old-artifact" },
	} {
		bad := after
		change(&bad)
		if matchesAgentUpgrade(before, bad, "v1.14.4", "new-artifact") {
			t.Fatalf("accepted incompatible state: %+v", bad)
		}
	}
}
