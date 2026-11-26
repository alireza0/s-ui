package nodeagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrafficLedgerSurvivesAgentRestartAndDoesNotExposeMutableMaps(t *testing.T) {
	directory := t.TempDir()
	r := NewCoreRuntimeWithState(directory)
	r.traffic.Users = map[string]UserTraffic{"1:100": {Up: 10, Down: 20, At: 100}}
	r.trafficDirty = true
	sample := r.Traffic()
	if sample == nil {
		t.Fatal("could not persist sample")
	}
	sample.Users["1:100"] = UserTraffic{Up: 9999}
	r.Close()
	restarted := NewCoreRuntimeWithState(directory)
	defer restarted.Close()
	stored := restarted.Traffic()
	if stored == nil || stored.Process != sample.Process || stored.Users["1:100"].Up != 10 {
		t.Fatal("restart reset or corrupted usage", stored)
	}
	info, err := os.Stat(filepath.Join(directory, "traffic.json"))
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("traffic ledger is not private", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "traffic.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := NewCoreRuntimeWithState(directory)
	defer broken.Close()
	if broken.Traffic() != nil || broken.Validate([]byte(`{}`)) == nil {
		t.Fatal("corrupted ledger silently reset accounting")
	}
}

func TestProxyAndNamedTrafficMapToSameStableClientAcrossAccessChanges(t *testing.T) {
	a := AccessSnapshot{Clients: []Grant{{Name: "client/1/g1/n1", TrafficID: "1:100", Credentials: map[string]json.RawMessage{"http": json.RawMessage(`{"username":"example-user","password":"fixture"}`)}}}}
	aliases := trafficAliases(a)
	if aliases["example-user"] != "1:100" || aliases["client/1/g1/n1"] != "1:100" {
		t.Fatal("proxy and UUID users do not share quota identity", aliases)
	}
	a.Clients[0].Name = "client/1/g2/n2"
	if trafficAliases(a)[a.Clients[0].Name] != "1:100" {
		t.Fatal("access generation reset client counters")
	}
}
