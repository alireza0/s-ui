package nodes

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os/exec"
	"reflect"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func TestListenerPortsSkipBusySocketsAndReserveManualFirst(t *testing.T) {
	db, node, _ := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	const host = "192.0.2.20"
	if err := db.Create(&model.NodePortReservation{NodeId: 999, ConnectionKey: "other", Host: host, Network: "tcp", Port: 8443}).Error; err != nil {
		t.Fatal(err)
	}
	requests := []listenerRequest{
		{Key: "first-tcp", listenerSocket: listenerSocket{Network: "tcp"}},
		{Key: "first-udp", listenerSocket: listenerSocket{Network: "udp"}},
		{Key: "second-udp", listenerSocket: listenerSocket{Network: "udp"}},
		{Key: "manual", listenerSocket: listenerSocket{Network: "udp", Port: 8443}},
		{Key: "custom", listenerSocket: listenerSocket{Network: "tcp", Port: 16123}},
	}
	occupied := map[listenerSocket]bool{{Network: "tcp", Port: 443}: true}
	var result []provisionListener
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = reserveListenerPlan(tx, node.Id, host, requests, occupied)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var ports []int
	for _, listener := range result {
		ports = append(ports, listener.Port)
	}
	if !reflect.DeepEqual(ports, []int{9443, 443, 9443, 8443, 16123}) {
		t.Fatalf("unexpected TCP/UDP assignments: %v", ports)
	}
	// Once stored, the selected automatic port behaves as a fixed listener
	// across repeated applies or a move to another server.
	value, err := loadGeneratedValue(db, node.Id, "first-tcp", "listen_port")
	if err != nil || string(value) != "9443" {
		t.Fatalf("automatic port was not persisted: %s %v", value, err)
	}
	requests[0].Port = ports[0]
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := reserveListenerPlan(tx, node.Id, "192.0.2.21", requests[:1], map[listenerSocket]bool{{Network: "tcp", Port: 9443}: true})
		return err
	}); !errors.Is(err, errListenerOccupied) {
		t.Fatalf("existing port changed during move: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := reserveListenerPlan(tx, node.Id, host, requests[4:], map[listenerSocket]bool{requests[4].listenerSocket: true})
		return err
	}); !errors.Is(err, errListenerOccupied) {
		t.Fatalf("occupied manual port was not rejected: %v", err)
	}
}

func TestListenerInspectionDetectsRealSocketsPerNetwork(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("socket inspection needs python3 on Linux")
	}
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	port := tcp.Addr().(*net.TCPAddr).Port
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	udp.Close()
	probes := []listenerSocket{{Network: "tcp", Port: port}, {Network: "udp", Port: port}}
	body, _ := json.Marshal(map[string]interface{}{"probes": probes, "fixed_tcp_ports": []int{16500}})
	// Run the actual socket checks without requiring a systemd fixture or root.
	prefix := "import json,os,platform,re,socket,subprocess,sys\nfrom pathlib import Path\np=json.load(sys.stdin)\nunit=Path('/dev/null/fixture')\nname='fixture'\n"
	cmd := exec.Command(python, "-c", prefix+liveInspectionScript[len(liveHostHeader):])
	cmd.Stdin = bytes.NewReader(body)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("port inspection failed: %s %v", out, err)
	}
	var inspection listenerInspection
	if err := json.Unmarshal(out, &inspection); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspection.Occupied, probes[:1]) {
		t.Fatalf("TCP listener also blocked UDP or escaped detection: %v", inspection.Occupied)
	}
	if inspection.ControlPort == 0 || inspection.ControlPort == 16500 {
		t.Fatalf("control listener conflicts with an explicitly requested TCP port: %d", inspection.ControlPort)
	}
	// An actively closed TCP connection leaves TIME_WAIT after the listener
	// stops. It must not prevent restoring an inbound on the same port.
	connection, err := net.Dial("tcp4", tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := tcp.Accept()
	if err != nil {
		connection.Close()
		t.Fatal(err)
	}
	accepted.Close()
	buffer := make([]byte, 1)
	connection.Read(buffer)
	connection.Close()
	tcp.Close()
	cmd = exec.Command(python, "-c", prefix+liveInspectionScript[len(liveHostHeader):])
	cmd.Stdin = bytes.NewReader(body)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("closed listener inspection failed: %s %v", out, err)
	}
	if err = json.Unmarshal(out, &inspection); err != nil || len(inspection.Occupied) != 0 {
		t.Fatalf("TIME_WAIT was reported as an active listener: %s %v", out, err)
	}
}
