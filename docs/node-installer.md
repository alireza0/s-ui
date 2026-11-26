# Live node installer API

The authenticated cookie API and APIv2 expose `node-provision`. The installer
uses model 2, whole-node assignments and schema 1 templates.

POST form field `data` contains JSON:

```json
{
  "model_version": 2,
  "request_key": "example-operation-00000001",
  "name": "Example node",
  "ip": "192.0.2.10",
  "ssh_port": 22,
  "ssh_user": "root",
  "password": "<bootstrap password>",
  "inbound_template_ids": [1, 2],
  "domain": {
    "mode": "managed",
    "profile_id": 1,
    "zone_id": "<configured zone ID>",
    "automatic_prefix": true,
    "certificate_mode": "auto"
  }
}
```

The UI selects reusable inbound templates by `inbound_template_ids`. An empty
selection creates a node without VPN listeners. Cloning a node with only
private templates uses `protocols` to create new private presets and listener
credentials. For an existing node, `template_id` can identify its current
deployment and does not change the selection.

The request key contains 16–96 letters, digits, underscores or hyphens. The same
key and intent return the original job; a different intent conflicts. An explicit
retry of a failed job uses a new request key. Bootstrap password and uploaded
certificate secrets are absent from the public job and saved request payload.

For an existing node provide `node_id` and `expected_version` (its current
`settings_version`). Use `action: "update"` or `"recover"`. Host-key
replacement requires explicit recovery, `replace_host_key: true` and a
password. Template replacement uses the live-template API separately.

Job checkpoints precede remote mutations. Password is not persisted; an unfinished
first login may need re-entry. Later stages resume with the encrypted dedicated
key. A committed job verifies its recorded identity before continuing.

Stages: `ssh`, `preflight`, `dependencies`, `inspect`, `ssh_key`, `upload`, `retire`,
`install`, `register`, `verify`, `certificate`, `ready`.
A real key-only login and all planned listeners are checked before fencing the
old service. Only suffix-owned services, paths and key markers are changed.
An occupied port or unreachable previous target blocks cutover.

Transfers are compressed and checked against the original artifact SHA-256.
An identical installed artifact can be staged locally. Structural/access/resource
ACKs are fenced by management generation and registration. New bootstrap holds
VPN until fresh authenticated access; an ordinary later offline restart resumes
the last good configuration.

Assign a node using the client's independent `nodes` array.
`inbounds` remains local. The read-only `nodeinbounds` reports applied remote
listener IDs. Read `clients?id=<ID>` before editing a client's protocol settings.

For control-certificate renewal, HTTP-01 firewall permissions and interrupted
installation recovery, see [node maintenance](nodes-maintenance.md).

## Backup

`GET .../getdb?format=archive` returns `panel.db`, optional
`node-provision.key` and `manifest.json`. Only `stats` and `changes`
may be excluded. The coherent SQLite snapshot preserves sequence high-water values.

`POST .../importdb` accepts multipart field `db` containing SQLite or ZIP.
Archive/key validation precedes the live DB swap. A durable restore journal rolls
back an incomplete DB/key pair or validates a completed pair at startup.
Import does not compare node revisions against the destination database:
restoration deliberately installs the archived state. Supported older database
versions follow the normal S-UI migration chain, including on a fresh panel.

Each download disables HTTP caching and reads a new coherent database snapshot;
it includes the latest committed node settings, template targets,
desired/applied access, resources
and traffic cursors even while remote application is pending. It cannot include
changes made after that snapshot. Save a new full backup after changing access.

An explicit import schedules restored node access and certificate settings as
new commands above the authenticated agent's observed revisions. Structural
delivery likewise advances above the observed revision. Archived worker leases
are released. Agent epochs, keys, configuration and replay protection are not
reset. Ordinary delivery without an import still rejects older/conflicting
access or certificate commands.

For a transfer, stop the previous panel before the restored panel adopts its
network settings and controls the same nodes. Import preserves panel settings
and subscription addresses; moving to another host still requires suitable
DNS, certificate paths and network reachability. A backup made before an OS
reinstall or management-key replacement cannot restore the missing remote
identity; use explicit node recovery for that case.

Actionable errors include `node_settings_version_conflict`,
`node_listener_port_occupied`, `ssh_key_verification_failed`,
`old_server_retirement_pending`, `ssh_host_changed`,
`certificate_request_rejected`, `structural_revision_rejected`,
`installation_database_unavailable`, `access_state_rollback_detected`.
