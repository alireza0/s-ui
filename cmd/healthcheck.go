package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/service"
)

// healthCheckTimeout is per dial. Two dials must fit in the container's
// five-second health check timeout.
const healthCheckTimeout = 2 * time.Second

// healthCheck reports whether the panel is accepting connections on the port it
// is actually configured with.
//
// It reads the port from the database rather than taking one on the command
// line, because the operator can change it from the panel at any time. A health
// check with the port baked into the Dockerfile goes red the moment they do,
// and an orchestrator then kills a container that was working perfectly.
//
// It dials rather than speaking HTTP: the panel serves either HTTP or HTTPS
// depending on whether a certificate is configured, and a check that assumed
// one of them would fail on the other.
func healthCheck() {
	// OpenDB, not InitDB: this runs every 30 seconds beside the live panel, and
	// InitDB migrates and writes, so it waited on the panel's write lock for up
	// to the busy timeout and the container went unhealthy (#1274). Under WAL a
	// plain read does not wait on a writer.
	if err := database.OpenDB(config.GetDBPath()); err != nil {
		fmt.Println("healthcheck: unable to open the database:", err)
		os.Exit(1)
	}

	settingService := service.SettingService{}
	port, err := settingService.GetPort()
	if err != nil {
		fmt.Println("healthcheck: unable to read the panel port:", err)
		os.Exit(1)
	}
	listen, err := settingService.GetListen()
	if err != nil {
		fmt.Println("healthcheck: unable to read the panel listen address:", err)
		os.Exit(1)
	}

	// A panel bound to one interface is not reachable on loopback. For an
	// empty or unspecified address, try IPv4 loopback, then IPv6 for a panel
	// on an IPv6-only "::".
	hosts := []string{"127.0.0.1", "::1"}
	if ip := net.ParseIP(listen); listen != "" && (ip == nil || !ip.IsUnspecified()) {
		hosts = []string{listen}
	}

	for _, host := range hosts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, dialErr := net.DialTimeout("tcp", addr, healthCheckTimeout)
		if dialErr != nil {
			err = dialErr
			continue
		}
		conn.Close()
		fmt.Printf("healthcheck: panel is listening on %s\n", addr)
		return
	}
	fmt.Printf("healthcheck: nothing listening on port %d: %v\n", port, err)
	os.Exit(1)
}
