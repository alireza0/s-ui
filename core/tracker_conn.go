package core

import (
	"context"
	"net"
	"sync"

	"github.com/gofrs/uuid/v5"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/network"
)

type ConnectionInfo struct {
	ID         string
	Conn       net.Conn
	PacketConn network.PacketConn
	Inbound    string
	Type       string // "tcp" or "udp"
}

type ConnTracker struct {
	access      sync.Mutex
	connections map[string]*ConnectionInfo
}

func NewConnTracker() *ConnTracker {
	return &ConnTracker{
		connections: make(map[string]*ConnectionInfo),
	}
}

func (c *ConnTracker) generateConnectionID() string {
	return uuid.Must(uuid.NewV4()).String()
}

func (c *ConnTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	connID := c.generateConnectionID()
	connInfo := &ConnectionInfo{
		ID:      connID,
		Conn:    conn,
		Inbound: metadata.Inbound,
		Type:    "tcp",
	}

	c.trackConnection(connID, connInfo)

	return c.createWrappedConn(conn, connID)
}

func (c *ConnTracker) RoutedPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) network.PacketConn {
	connID := c.generateConnectionID()
	connInfo := &ConnectionInfo{
		ID:         connID,
		PacketConn: conn,
		Inbound:    metadata.Inbound,
		Type:       "udp",
	}

	c.trackConnection(connID, connInfo)

	return c.createWrappedPacketConn(conn, connID)
}

func (c *ConnTracker) CloseConnByInbound(inbound string) int {
	c.access.Lock()
	defer c.access.Unlock()

	closedCount := 0
	for connID, connInfo := range c.connections {
		if connInfo.Inbound == inbound {
			if connInfo.Conn != nil {
				connInfo.Conn.Close()
			}
			if connInfo.PacketConn != nil {
				connInfo.PacketConn.Close()
			}
			delete(c.connections, connID)
			closedCount++
		}
	}
	return closedCount
}

func (c *ConnTracker) trackConnection(connID string, connInfo *ConnectionInfo) {
	c.access.Lock()
	defer c.access.Unlock()
	c.connections[connID] = connInfo
}

func (c *ConnTracker) untrackConnection(connID string) {
	c.access.Lock()
	defer c.access.Unlock()
	delete(c.connections, connID)
}

func (c *ConnTracker) createWrappedConn(conn net.Conn, connID string) *wrappedConn {
	return &wrappedConn{
		Conn:   conn,
		connID: connID,
	}
}

func (c *ConnTracker) createWrappedPacketConn(conn network.PacketConn, connID string) *wrappedPacketConn {
	return &wrappedPacketConn{
		PacketConn: conn,
		connID:     connID,
	}
}

type wrappedConn struct {
	net.Conn
	connID string
}

func (w *wrappedConn) Close() error {
	connTracker.untrackConnection(w.connID)
	return w.Conn.Close()
}

func (w *wrappedConn) Upstream() any {
	return w.Conn
}

type wrappedPacketConn struct {
	network.PacketConn
	connID string
}

func (w *wrappedPacketConn) Close() error {
	connTracker.untrackConnection(w.connID)
	return w.PacketConn.Close()
}

func (w *wrappedPacketConn) Upstream() any {
	return w.PacketConn
}
