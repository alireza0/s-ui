package core

import (
	"net"

	"github.com/sagernet/sing/common/network"
)

type wrappedConn struct {
	net.Conn
	tracker *ConnTracker
	connID  string
}

func (w *wrappedConn) Close() error {
	w.tracker.untrackConnection(w.connID)
	return w.Conn.Close()
}

type wrappedPacketConn struct {
	network.PacketConn
	tracker *ConnTracker
	connID  string
}

func (w *wrappedPacketConn) Close() error {
	w.tracker.untrackConnection(w.connID)
	return w.PacketConn.Close()
}
