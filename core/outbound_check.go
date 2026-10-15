package core

import (
	"context"
	"time"

	urltest "github.com/sagernet/sing-box/common/urltest"
)

const checkTimeout = 15 * time.Second

type CheckOutboundResult struct {
	OK    bool
	Delay uint16
	Error string
}

// CheckOutbound measures an outbound against a test URL.
//
// The box reference is taken under the read lock and released before the test
// runs: URLTest dials the network and can take the full checkTimeout, and
// holding the core lock for fifteen seconds would freeze every status poll.
func (c *Core) CheckOutbound(tag string, link string) (result CheckOutboundResult) {
	box, err := c.running()
	if err != nil {
		result.Error = "core not running"
		return result
	}

	ob, ok := box.outbound.Outbound(tag)
	if !ok {
		result.Error = "outbound not found"
		return result
	}

	ctx, cancel := context.WithTimeout(box.scope.Context(), checkTimeout)
	defer cancel()

	delay, err := urltest.URLTest(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.Delay = delay
	return result
}
