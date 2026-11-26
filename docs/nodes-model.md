# Whole-node assignment and versioned templates

## Client assignments

`inbounds` still contains local inbound IDs. `nodes` contains logical Node IDs:

```json
{"inbounds": [21], "nodes": [1]}
```

- Omitted `nodes` preserves assignments; `[]` removes them.
- Invalid, duplicate, unknown IDs or nodes outside active whole-node mode reject
  the transaction.
- A whole-node assignment includes all enabled connections of that node.
- New connections are published only from confirmed applied state.
- Credentials already stored for the client are preserved; missing credentials
  for the enabled agent capabilities are prepared when assigning a whole node.
- Removing/re-adding a node retains and increments its grant generation.
- Changes enqueue durable access intent. The v2 access worker runs independently
  of structural compilation and provisioning jobs.

`nodeinbounds` reports applied remote listener IDs. A matching unchanged echo
is accepted for reads followed by edits. Node selection is written through
`nodes`; the panel's `inbounds` still identifies local listeners.

## Agent protocol v2

Control uses the existing mTLS transport, optionally over the managed SSH
connection. The following endpoints are agent endpoints, not public panel API:

- `GET /v2/status`: identity, registration epoch, structural revision/digest,
  durable access revision/digest, applied access revision/digest, running/error
  and certificate observations; no client credentials.
- `PUT /v2/access`: a complete client access snapshot, independent of a template
  or structural config. The agent persists intent before runtime application.
- `PUT /v2/spec`: a structural config without `users`. The agent combines it
  with its latest durable access, including during structural recovery.

Revisions are independent and monotonic inside a pinned epoch. A retry with the
same revision and digest is idempotent. The agent rejects stale revisions,
different payloads at the same revision and unexpected epochs. After an explicit
database import, the panel compares saved access and certificate intent with
authenticated agent state and issues a new revision when needed. See
[backup and restore](nodes-maintenance.md#backup-and-restore).

Access updates can interrupt a blocked structural startup. A failed hot update
rebuilds last-good structure with current permissions. If that cannot start,
the agent stops its own core and retains the durable access intent. Reboot
restoration applies current access and known absolute expiry before serving.
Loss of panel connectivity has no three-minute shutdown timer.

Subscriptions compose confirmed applied structure with confirmed applied access
and current client permissions. Pending grants and credentials are withheld.
A confirmed stopped core is hidden until running is confirmed again; a control
timeout does not undo the last known stop.

## Template versions

`api/save` with object `nodetemplates` expects `kind: "inbound"`, `name`, and
a `spec` with one listener. An edit requires `id` and `expected_version`.
Each saved configuration has an immutable version. A spec contains:

- `schema_version: 1`, `config: {}` and an `entries` array;
- each entry's persistent `connection_key`, `name`, `protocol`, `enable`,
  `options` and optional `sources`;
- a numeric `listen_port`, or a source bound to `/listen_port`.

Supported client proxy inbounds are `vless`, `tuic`, `hysteria2`, `vmess`,
`trojan`, `anytls`, `shadowsocks`, `snell`, `shadowtls`, `hysteria`, `naive`,
`http`, `socks` and `mixed`, subject to the installed agent capabilities.
Protocol-specific transport and TLS options follow the pinned sing-box core.
REALITY is available for VLESS, not QUIC protocols. A key's protocol cannot
change, including after the entry was removed in an intervening version.
Server keys, certificates and client users are not inline template literals.

Source kinds are `literal`, `node_parameter`, `node_generated`,
`node_certificate` and `profile`. Node parameters include `primary_domain`,
`node_address`, `node_name` and `node_id`. Generated material is keyed by the
connection and purpose, not generated during resolution. Explicit overrides
replace inherited sources; arrays are atomic. Disabled entries do not demand
runtime resources. Unknown sources and overlapping overrides are errors.

- `GET node-template-history?id=<template-id>` returns version metadata.
- `GET node-template-version?id=<template-id>&version=<version>` returns the
  schema 1 spec. Omit `version` for the latest version.

## Live template delivery

Schema 1 templates are shared by default. A private template has `scope:
"private"` and an `owner_node_id`; other nodes cannot bind it. Template copies
retain lineage and connection keys, while runtime material belongs to each node.
An origin template cannot be deleted while copies reference it.

Use object `nodes`, action `selectinbounds`, and data containing `id`,
`expected_version` (the node settings version) and `inbound_template_ids`.
An empty list removes every listener after the agent acknowledges the change.

Saving a schema 1 version transactionally updates each linked node's target and
queues durable structural work. Binding a node reads the latest version inside
its transaction. Per-node `template_target_version`,
`materialized_template_version` and `applied_template_version` distinguish saved,
resolved and acknowledged state. Runtime revisions remain independent from
template versions. The [installer](node-installer.md) uses the same template
catalog.

Materialization resolves node parameters and explicit overrides, allocates
stable REALITY material and optional ports, and persists it before delivery.
Generated values use the external panel state key and are bound to node/key/
purpose. TCP and UDP reservations are separate and use the installation IP, or
management hostname for manually managed nodes. Aliases of the same host and
ports occupied by unrelated services still require installer-side inspection.
Missing certificate/profile resources keep last-good. Independent
[resource workers](node-resources.md) prepare and confirm the requested
material; resource names are never treated as certificate contents.

Renaming/reordering a connection preserves its NodeInbound ID and server tag.
Removed entries become retired records; their overrides are archived, not
silently applied again when restoring history. Retired entries do not detach
into working manual listeners. Old public connections remain until a confirming
ACK; newly added ones appear only after confirmed structure/access/certificate
readiness. Restoring history keeps surviving keys/material and advances the
template version monotonically.

Structural tasks use expiring leases, bounded retries with jitter and generation
coalescing. A rejected runtime spec waits for a new target or `nodes/retry`.
Access delivery continues independently. Immutable NodeDeployment snapshots
recover a lost ACK from agent status; a late ACK records the actual applied
version without replacing the desired version or a newer observation.

Additional `api/save` actions (same cookie/token authorization as ordinary save):

- Object `nodes`, action `setoverrides`: `id`, `expected_version` equal to the
  current node `settings_version`, and an `overrides` array. This replaces the
  active override set; `[]` resets it. Each item uses `connection_key`, `path`,
  `operation: "set" | "remove"`, and `source` for set. Explicit equal values
  remain overrides. Arrays are replaced as a whole. Removed-key overrides are
  archived; reactivating one requires an explicit new override write.
- Object `nodetemplates`, action `restoreversion`: `id`, historical `version`
  and current template `expected_version`. Creates a new current version from
  schema 1 history and automatically queues linked nodes.

Use template selection and overrides to change a node deployment. The ordinary
node edit can change its name or enabled state. Use the installer to replace its
address or restore SSH access.

## Persistence

The shared database backup registry includes whole-node grants, template
versions, explicit overrides, encrypted generated-material storage, durable
tasks, immutable deployments, port reservations, and access snapshots. Access payloads contain client
credentials and are excluded from ordinary bundle JSON. Installation secrets
still require the external panel state key; a database download alone is not a
full recovery. Deleting a disabled, confirmed-revoked node removes its grants
and control state; an unfinished installation or old-host retirement blocks
deletion.
