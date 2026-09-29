package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/service"

	_ "github.com/mattn/go-sqlite3"
)

const (
	// healthCheckDBTimeout limits the read-only lookup of the panel listen
	// settings. The check does not run migrations or update application data.
	healthCheckDBTimeout = 1 * time.Second

	// healthCheckDialTimeout is the upper bound for a single TCP connect attempt.
	// Candidate addresses share the remaining check budget, so each fallback
	// receives an equal slice of whatever time is left, capped at this value.
	healthCheckDialTimeout = 3 * time.Second

	// healthCheckTimeout covers database access and all dial attempts, leaving
	// room for the process to exit before the container's five-second timeout.
	healthCheckTimeout = healthCheckDBTimeout + healthCheckDialTimeout
)

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
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	// Opening a database connection may not honor context cancellation, so
	// enforce the overall deadline at the command level as well.
	type result struct {
		addr string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		addr, err := checkPanel(ctx)
		done <- result{addr, err}
	}()

	var r result
	select {
	case r = <-done:
		if ctx.Err() != nil {
			r.err = ctx.Err()
		}
	case <-ctx.Done():
		r.err = ctx.Err()
	}
	if r.err != nil {
		fmt.Printf("healthcheck: failed after %s: %v\n", time.Since(started).Round(time.Millisecond), r.err)
		os.Exit(1)
	}
	fmt.Printf("healthcheck: panel is listening on %s (elapsed %s)\n", r.addr, time.Since(started).Round(time.Millisecond))
}

func checkPanel(ctx context.Context) (string, error) {
	dbPath, err := filepath.Abs(config.GetDBPath())
	if err != nil {
		return "", fmt.Errorf("unable to resolve the database path: %w", err)
	}
	// Open the existing database read-only.
	dsn := url.URL{Scheme: "file", Path: filepath.ToSlash(dbPath)}
	params := url.Values{"mode": {"ro"}, "_busy_timeout": {"500"}, "_query_only": {"true"}}
	dsn.RawQuery = params.Encode()
	db, err := sql.Open("sqlite3", dsn.String())
	if err != nil {
		return "", fmt.Errorf("unable to open the database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Match SettingService: use defaults for missing settings.
	// The sub-context limits the query to healthCheckDBTimeout; the command-level
	// deadline in healthCheck is the final backstop if the driver ignores cancellation.
	dbCtx, dbCancel := context.WithTimeout(ctx, healthCheckDBTimeout)
	defer dbCancel()
	var listen, portValue string
	err = db.QueryRowContext(dbCtx, `SELECT
		COALESCE((SELECT value FROM settings WHERE key = 'webListen' ORDER BY id LIMIT 1), ?),
		COALESCE((SELECT value FROM settings WHERE key = 'webPort' ORDER BY id LIMIT 1), ?)`,
		service.DefaultWebListen, service.DefaultWebPort).Scan(&listen, &portValue)
	if err != nil {
		return "", fmt.Errorf("unable to read the panel listen settings: %w", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid panel port: %q", portValue)
	}

	hosts := []string{listen}
	switch ip := net.ParseIP(listen); {
	case listen == "":
		hosts = []string{"127.0.0.1", "::1"}
	case ip != nil && ip.IsUnspecified():
		if ip.To4() != nil {
			hosts = []string{"127.0.0.1"}
		} else {
			hosts = []string{"::1", "127.0.0.1"}
		}
	}

	var failures []error
	for i, host := range hosts {
		if err := ctx.Err(); err != nil {
			return "", errors.Join(append(failures, err)...)
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		// Reserve a share of the remaining time for each fallback address.
		dialTimeout := healthCheckDialTimeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return "", errors.Join(append(failures, context.DeadlineExceeded)...)
			}
			dialTimeout = min(dialTimeout, remaining/time.Duration(len(hosts)-i))
		}
		dialer := net.Dialer{Timeout: dialTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return addr, nil
		}
		failures = append(failures, fmt.Errorf("unable to connect to %s: %w", addr, err))
		if ctx.Err() != nil {
			break
		}
	}
	return "", errors.Join(failures...)
}
