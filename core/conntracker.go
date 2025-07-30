package core

import (
	"context"
	"net"
	"s-ui/database/model"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/network"
)

type Counter struct {
	read  *atomic.Int64
	write *atomic.Int64
}

type ConnectionInfo struct {
	ID         string
	Conn       net.Conn
	PacketConn network.PacketConn
	Inbound    string
	User       string
	CreatedAt  time.Time
	Type       string // "tcp" or "udp"
}

type ConnTracker struct {
	access      sync.Mutex
	createdAt   time.Time
	inbounds    map[string]Counter
	outbounds   map[string]Counter
	users       map[string]Counter
	connections map[string]*ConnectionInfo
}

func NewConnTracker() *ConnTracker {
	return &ConnTracker{
		createdAt:   time.Now(),
		inbounds:    make(map[string]Counter),
		outbounds:   make(map[string]Counter),
		users:       make(map[string]Counter),
		connections: make(map[string]*ConnectionInfo),
	}
}

func (c *ConnTracker) getReadCounters(inbound string, outbound string, user string) ([]*atomic.Int64, []*atomic.Int64) {
	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	c.access.Lock()
	if inbound != "" {
		readCounter = append(readCounter, c.loadOrCreateCounter(&c.inbounds, inbound).read)
		writeCounter = append(writeCounter, c.inbounds[inbound].write)
	}
	if outbound != "" {
		readCounter = append(readCounter, c.loadOrCreateCounter(&c.outbounds, outbound).read)
		writeCounter = append(writeCounter, c.outbounds[outbound].write)
	}
	if user != "" {
		readCounter = append(readCounter, c.loadOrCreateCounter(&c.users, user).read)
		writeCounter = append(writeCounter, c.users[user].write)
	}
	c.access.Unlock()
	return readCounter, writeCounter
}

func (c *ConnTracker) loadOrCreateCounter(obj *map[string]Counter, name string) Counter {
	counter, loaded := (*obj)[name]
	if loaded {
		return counter
	}
	counter = Counter{read: &atomic.Int64{}, write: &atomic.Int64{}}
	(*obj)[name] = counter
	return counter
}

func (c *ConnTracker) generateConnectionID() string {
	return uuid.Must(uuid.NewV4()).String()
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

func (c *ConnTracker) createWrappedConn(conn net.Conn, connID string) net.Conn {
	return &wrappedConn{
		Conn:    conn,
		tracker: c,
		connID:  connID,
	}
}

func (c *ConnTracker) createWrappedPacketConn(conn network.PacketConn, connID string) network.PacketConn {
	return &wrappedPacketConn{
		PacketConn: conn,
		tracker:    c,
		connID:     connID,
	}
}

func (c *ConnTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	readCounter, writeCounter := c.getReadCounters(metadata.Inbound, matchOutbound.Tag(), metadata.User)

	connID := c.generateConnectionID()
	connInfo := &ConnectionInfo{
		ID:        connID,
		Conn:      conn,
		Inbound:   metadata.Inbound,
		User:      metadata.User,
		CreatedAt: time.Now(),
		Type:      "tcp",
	}

	c.trackConnection(connID, connInfo)

	wrappedConn := c.createWrappedConn(conn, connID)
	return bufio.NewInt64CounterConn(wrappedConn, readCounter, writeCounter)
}

func (c *ConnTracker) RoutedPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) network.PacketConn {
	readCounter, writeCounter := c.getReadCounters(metadata.Inbound, matchOutbound.Tag(), metadata.User)

	connID := c.generateConnectionID()
	connInfo := &ConnectionInfo{
		ID:         connID,
		PacketConn: conn,
		Inbound:    metadata.Inbound,
		User:       metadata.User,
		CreatedAt:  time.Now(),
		Type:       "udp",
	}

	c.trackConnection(connID, connInfo)

	wrappedConn := c.createWrappedPacketConn(conn, connID)
	return bufio.NewInt64CounterPacketConn(wrappedConn, readCounter, writeCounter)
}

func (c *ConnTracker) ForceCloseConn(inbound, user string) int {
	c.access.Lock()
	defer c.access.Unlock()

	closedCount := 0
	for connID, connInfo := range c.connections {
		if connInfo.Inbound == inbound && connInfo.User == user {
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

func (c *ConnTracker) GetStats() *[]model.Stats {
	c.access.Lock()
	defer c.access.Unlock()

	dt := time.Now().Unix()

	s := []model.Stats{}
	for inbound, counter := range c.inbounds {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s, model.Stats{
				DateTime:  dt,
				Resource:  "inbound",
				Tag:       inbound,
				Direction: false,
				Traffic:   down,
			}, model.Stats{
				DateTime:  dt,
				Resource:  "inbound",
				Tag:       inbound,
				Direction: true,
				Traffic:   up,
			})
		}
	}

	for outbound, counter := range c.outbounds {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s, model.Stats{
				DateTime:  dt,
				Resource:  "outbound",
				Tag:       outbound,
				Direction: false,
				Traffic:   down,
			}, model.Stats{
				DateTime:  dt,
				Resource:  "outbound",
				Tag:       outbound,
				Direction: true,
				Traffic:   up,
			})
		}
	}

	for user, counter := range c.users {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s, model.Stats{
				DateTime:  dt,
				Resource:  "user",
				Tag:       user,
				Direction: false,
				Traffic:   down,
			}, model.Stats{
				DateTime:  dt,
				Resource:  "user",
				Tag:       user,
				Direction: true,
				Traffic:   up,
			})
		}
	}
	return &s
}
