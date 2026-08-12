# s-ui Multi-Node Plan (Living Doc)

> Update this file at the end of every implementation session.
> Goal: next sessions can resume without rediscovering architecture.

Last updated: 2026-07-15
Current milestone: **M1 Node registry complete → next M2**
Status legend: `[ ]` todo · `[~]` in progress · `[x]` done · `[-]` deferred

---

## 1. Product Goal

Run **full s-ui on every machine** (master + nodes).

- Master is source of truth for **clients / quotas / expiry / subscription**.
- Each node is a **full s-ui panel** with its own DNS, route, outbounds, endpoints, services, settings.
- Master creates users once and fans them out to selected node-hosted inbounds (multi-location VPN sub).
- End-user keeps **one master subscription URL** containing all locations.

Non-goals for v1:
- multi-hop node trees / transitive GUIDs
- pushing master DNS/route/outbounds onto nodes
- agent-only binary
- blue/green inbound live migration
- Postgres requirement

---

## 2. Database Decision

**Keep SQLite + GORM + WAL as default.**

Why:
- s-ui already uses SQLite with WAL, busy timeout, connection pool (`database/db.go`)
- multi-node traffic is N HTTPS pulls into one master writer; not a multi-writer cluster DB problem
- standard SQLite default is sufficient until very high scale
- switching DB mid-feature adds risk without unlocking M1–M6

Later trigger for Postgres (optional, not now):
- master clients in high tens of thousands **and** frequent traffic merge contention
- operators already standardize on Postgres

Not using better-sqlite3 (Node.js). Backend is Go/GORM.

Schema approach:
- use existing `AutoMigrate` for new tables/columns
- keep versioned `cmd/migration` only for data transforms when needed

---

## 3. Target Architecture

```text
Master full s-ui
  owns: clients, node registry, managed inbound placement, sub links, global traffic
  may host local inbounds (node_id = null)

Node full s-ui (location)
  owns: local DNS/route/outbounds/settings/core
  receives: managed inbounds + users from master
  reports: heartbeat + traffic snapshot
```

Control plane pattern:
1. `Runtime` Local vs Remote
2. heartbeat + dirty + reconcile
3. master-side subscription
4. per-node traffic baselines merged on master

s-ui-specific constraints:
- local apply = embedded sing-box managers (`core.AddInbound` / `UpdateInboundUsers`)
- remote apply = HTTPS to peer s-ui `apiv2` + peer endpoints
- clients are master-centric (`Client.Inbounds` JSON)

---

## 4. Data Model (planned)

### 4.1 `nodes`
Connection + health + sync flags for a full remote s-ui.

Key fields:
- identity: name, remark
- connection: scheme, address, port, base_path, api_token
- security: tls_verify_mode (`verify|skip|pin`), pinned_cert_sha256, allow_private
- location: public_host (share-link host)
- sync: inbound_sync_mode (`selected|all`), inbound_tags_json, config_dirty, config_dirty_at
- heartbeat: status, last_heartbeat, latency_ms, panel_version, core_running, cpu/mem/uptime, last_error

### 4.2 `inbounds.node_id` (nullable)
- `NULL` = local master inbound
- non-null = managed on that node

### 4.3 `node_client_traffics`
`(node_id, client_name, up, down)` baselines for merge / anti double-count.

### 4.4 clients / sub
No ownership rewrite. Master `clients` remain authority. `clients.links` regenerated multi-location.

---

## 5. Runtime / Peer API (planned)

```go
type Runtime interface {
  Name() string
  UpsertManagedInbound(ctx, inboundJSON, users) error
  DeleteManagedInbound(ctx, tag) error
  SetManagedUsers(ctx, tag, users) error
  Snapshot(ctx) (*NodeSnapshot, error)
  RestartCore(ctx) error
}
```

Peer endpoints on every full s-ui (`apiv2`, token auth):
- `nodeSnapshot`
- `nodeApplyInbound`
- `nodeApplyUsers`
- `nodeDeleteInbound`

Policy:
- default sync mode `selected` (master only manages its tags)
- master-managed users are reconciled from master (node local edits of managed users can be overwritten)
- node DNS/route/outbounds never overwritten by master

---

## 6. Milestones

### M0 Architecture / planning
- [x] Read s-ui architecture (app/web/api/service/core/sub/db)
- [x] Decide full-panel-per-node multi-location product model
- [x] Decide keep SQLite for now
- [x] Create this living plan doc

### M1 Node registry foundation
- [x] Add `model.Node` + AutoMigrate (`database/model/nodes.go`, `database/db.go`, backup migrate)
- [x] Add `service/node.go` CRUD + normalize + probe helpers
- [x] Wire API list/get/save/delete/enable/test actions for nodes (`api` + `apiv2`)
- [x] Unit tests: `service/node_test.go` (7), `api/node_api_test.go` (2)
- [x] `go build ./...` passes
- [ ] Frontend Nodes page (later with UI milestone M7)
- [ ] Heartbeat cron (M2)

### M2 Peer snapshot + heartbeat
- [ ] `nodeSnapshot` peer endpoint
- [ ] Remote probe/heartbeat job
- [ ] status fields update + tests

### M3 Managed inbound apply path
- [ ] `inbounds.node_id`
- [ ] peer `nodeApplyInbound` / `nodeDeleteInbound`
- [ ] Runtime Local/Remote skeleton
- [ ] tests for apply/delete/reconcile basic

### M4 Client fan-out (multi-location users)
- [ ] on client save, push users to all attached node inbounds
- [ ] offline dirty mark + reconcile
- [ ] tests for fan-out / offline / reconnect

### M5 Subscription multi-location links
- [ ] link builder uses node.public_host / inbound.Addrs
- [ ] raw/json/clash include all locations
- [ ] tests for link host selection

### M6 Traffic merge + global deplete
- [ ] node_client_traffics
- [ ] merge job
- [ ] deplete disables on all nodes
- [ ] tests for baseline merge and deplete push

### M7 UI
- [ ] Nodes page
- [ ] inbound node selector
- [ ] client multi-location UX polish

---

## 7. Session Log

### 2026-07-15 — M1 foundation landed
Done:
- wrote this living plan
- DB decision: **keep SQLite + GORM + WAL** (Postgres deferred; no better-sqlite3)
- `model.Node` + AutoMigrate in `database/db.go` and backup path
- `service/node.go`: Normalize, CRUD, dirty flags, BaseURL/APIv2URL, Probe via `apiv2/status`, ApplyProbeResult
- API (session + apiv2):
  - GET `nodes`, GET `node?id=`
  - POST `saveNode`, `deleteNode`, `setNodeEnable`, `testNode`
- Tests verified:
  - `go test ./service -run 'Node|DecodeCert|PrivateAddress' -count=1` → 7 passed
  - `go test ./api -run 'Node|ParseID' -count=1` → 2 passed
  - `go build ./...` → ok

Not done yet:
- no `runtime/` package
- no peer apply endpoints (`nodeSnapshot` / `nodeApply*`)
- no `inbounds.node_id`
- no heartbeat cron
- no frontend Nodes page
- no client fan-out / sub multi-location / traffic merge

**Resume next session at: M2**
1. Add peer `nodeSnapshot` endpoint on every s-ui (richer than status)
2. Heartbeat cron job over enabled nodes
3. Tests for snapshot schema + offline/online transitions
4. Then M3 managed inbound apply + `inbounds.node_id`

---

## 8. Verification Cheatsheet

```bash
# focused
go test ./service -run Node -count=1
go test ./api -run Node -count=1
go test ./database -count=1

# broader as features land
go test ./runtime ./cronjob ./sub -count=1
```

Manual M1 check:
1. run two s-ui instances
2. create API token on node
3. on master API: save node with address/token/public_host
4. list nodes
5. test/probe node (status reachable)

---

## 9. Coding Rules for This Feature

- every behavior change ships with tests
- controllers/handlers stay thin; logic in service/runtime
- do not push node DNS/route/outbounds from master
- keep single-node path identical when no nodes configured
- update this file every session before stopping
