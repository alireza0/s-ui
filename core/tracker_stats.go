package core

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database/model"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/network"
)

type Counter struct {
	read  *atomic.Int64
	write *atomic.Int64
}

type StatsTracker struct {
	access    sync.Mutex
	inbounds  map[string]Counter
	outbounds map[string]Counter
	users     map[string]Counter
}

func NewStatsTracker() *StatsTracker {
	return &StatsTracker{
		inbounds:  make(map[string]Counter),
		outbounds: make(map[string]Counter),
		users:     make(map[string]Counter),
	}
}

func (c *StatsTracker) Reset() {
	c.access.Lock()
	defer c.access.Unlock()
	c.inbounds = make(map[string]Counter)
	c.outbounds = make(map[string]Counter)
	c.users = make(map[string]Counter)
}

func (c *StatsTracker) getReadCounters(inbound string, outbound string, user string) ([]*atomic.Int64, []*atomic.Int64) {
	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	c.access.Lock()
	defer c.access.Unlock()

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
	return readCounter, writeCounter
}

func (c *StatsTracker) loadOrCreateCounter(obj *map[string]Counter, name string) Counter {
	counter, loaded := (*obj)[name]
	if loaded {
		return counter
	}
	counter = Counter{read: &atomic.Int64{}, write: &atomic.Int64{}}
	(*obj)[name] = counter
	return counter
}

func (c *StatsTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	readCounter, writeCounter := c.getReadCounters(metadata.Inbound, matchOutbound.Tag(), metadata.User)
	return bufio.NewInt64CounterConn(conn, readCounter, writeCounter)
}

func (c *StatsTracker) RoutedPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) network.PacketConn {
	readCounter, writeCounter := c.getReadCounters(metadata.Inbound, matchOutbound.Tag(), metadata.User)
	return bufio.NewInt64CounterPacketConn(conn, readCounter, nil, writeCounter, nil)
}

func (c *StatsTracker) GetStats() *[]model.Stats {
	c.access.Lock()
	defer c.access.Unlock()

	dt := time.Now().Unix()

	// Emit only directions that actually moved traffic; a zero-traffic row would
	// just bloat the stats table without changing any chart bucket.
	s := []model.Stats{}
	appendStat := func(resource, tag string, down, up int64) {
		if down > 0 {
			s = append(s, model.Stats{DateTime: dt, Resource: resource, Tag: tag, Direction: false, Traffic: down})
		}
		if up > 0 {
			s = append(s, model.Stats{DateTime: dt, Resource: resource, Tag: tag, Direction: true, Traffic: up})
		}
	}

	for inbound, counter := range c.inbounds {
		appendStat("inbound", inbound, counter.write.Swap(0), counter.read.Swap(0))
	}
	for outbound, counter := range c.outbounds {
		appendStat("outbound", outbound, counter.write.Swap(0), counter.read.Swap(0))
	}
	for user, counter := range c.users {
		appendStat("user", user, counter.write.Swap(0), counter.read.Swap(0))
	}
	return &s
}
