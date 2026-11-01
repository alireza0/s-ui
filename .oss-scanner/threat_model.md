# Threat model: s-ui

## What this project does and where untrusted input enters
s-ui is a self-hosted web panel that manages an embedded sing-box proxy core (inbounds, outbounds, endpoints, TLS, clients, traffic limits). One Go binary (`sui`) serves three things:

- **Admin panel** (`web/`, `api/apiHandler.go`): a Vue SPA (`frontend/`, a submodule) plus a session-cookie JSON API under `<base>/api`. Login, password change, config save, DB import/export, token management, self-update. Reachable from the Internet in most deployments; anyone can reach the login form.
- **Token API** (`api/apiV2Handler.go`, `<base>/apiv2`): the same actions, authenticated by an API token header instead of a session.
- **Subscription server** (`sub/`): a separate listener that serves `/<subPath>/<subid>` (plain links, `?format=json`, `?format=clash`) to every end user. It is unauthenticated apart from knowing the `subid`, and it faces all clients of the panel, not just the operator.

Untrusted input:
- Everything an unauthenticated Internet client sends to the panel port and the subscription port (paths, headers, cookies, `X-Forwarded-*`, Host, query strings, the login body).
- Subscription IDs and query parameters on the subscription server.
- Remote content the panel fetches: external subscription URLs (`util/subToJson.go`), link/sub conversion (`linkConvert`, `subConvert`), GitHub release metadata and downloads for self-update (`service/update.go`, `service/upgrade.go`), Cloudflare WARP API responses (`service/warp.go`).
- An uploaded database file (`importdb`, `database/backup.go`) is operator-supplied but should still never lead to code execution or file writes outside the DB folder.

Trusted: the operator with a valid admin session or API token is trusted to configure the proxy (they can already point sing-box anywhere). Local shell access to the host, and the CLI (`cmd/`, `s-ui.sh`, `install.sh`), are out of the attacker's reach and out of scope.

## Components that matter most / least
Most important:
- Authentication and session handling: `api/session.go`, login and brute-force limiting (`service/login_limit.go`), token checks in `api/apiV2Handler.go`, CSRF / same-origin checks (`middleware/csrf.go`), domain validation (`middleware/domainValidator.go`), trusted-proxy handling in `web/web.go`.
- Subscription server (`sub/`): anything that lets one client read another client's config or credentials, enumerate `subid`s, or reach panel functionality.
- Anything that turns panel input into commands, files or paths: `service/upgrade.go` (`exec`, `systemd-run`), DB import/backup, TLS certificate and key file paths (`service/tls.go`, `util/cert.go`).
- Server-side requests to attacker-influenced URLs (SSRF from `linkConvert` / `subConvert` / external subscriptions).
- Stored XSS in the panel from data that non-admins control (client remarks via subscriptions, external subscription content, sing-box log lines shown in `Logs.vue`).

Less important but in scope:
- Generation of sing-box / Clash / JSON configs (`util/`, `sub/*Service.go`) where malformed input should not corrupt other clients' entries.
- Stats and cron jobs (`cronjob/`).

Out of scope:
- sing-box itself and other third-party Go modules; report only how s-ui misuses them.
- npm/Go dependency CVEs without a reachable path in s-ui.
- Issues that require an already-authenticated admin to attack themselves (e.g. an admin putting a bad outbound in their own config).

## How to exercise it
- The image has the binary at `/src/sui`. Run it with a scratch DB: `SUI_DB_FOLDER=/tmp/sui-db /src/sui` (first run creates the DB and default admin; `/src/sui admin -h` and `/src/sui setting -h` show how to set credentials, ports and paths).
- The panel defaults to port 2095 and the subscription server to 2096; both are plain HTTP unless TLS is configured.
- Tests: `. ./build-tags.sh && go test -tags "$(tags_for test)" ./...` from `/src`. End-to-end API tests live in `api/login_e2e_test.go` and `api/apiv2_e2e_test.go`; subscription hardening tests in `sub/hardening_test.go` and `sub/anti_probing_test.go` are good templates for reproducers.
- Frontend: `cd /src/frontend && npm test` (vitest).

## How you rate severity
- **Critical:** unauthenticated remote code execution; unauthenticated authentication bypass to the panel or token API; unauthenticated read of the database, admin credentials or API tokens.
- **High:** RCE, arbitrary file read/write or command injection that needs an admin session or token; reading another client's subscription/credentials without knowing their `subid`; SSRF from an unauthenticated position; stored XSS in the admin panel triggered by data a non-admin controls; CSRF that performs a state-changing admin action.
- **Medium:** SSRF that requires admin; login rate-limit bypass; session fixation or sessions that survive logout/password change; information disclosure of server internals to unauthenticated users; DoS that one unauthenticated request can trigger (crash, unbounded memory).
- **Low:** DoS that needs sustained traffic or admin access; missing hardening headers; issues only exploitable with a misconfigured reverse proxy.

A finding without a working reproducer against the built binary or a test should be rated one level lower.

## Reports and patches
- Please include a Go test (in the style of the existing `*_test.go` files) or a `curl` sequence against a freshly started `sui` as the reproducer.
- Keep patches minimal and `gofmt`-clean; build tags belong in `build-tags.sh`, not inline.

## Anything to leave alone
- Default credentials on a brand-new install are documented and changed during setup; do not report them.
- The panel and subscription server listening on plain HTTP by default is intended (TLS is optional and often terminated by a reverse proxy).
- The container starting as root by default is intentional (TUN needs `CAP_NET_ADMIN`); see `entrypoint.sh`.
