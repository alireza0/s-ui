# Independent node agent

The agent reuses S-UI's `core` in a separate process, without the panel database,
web admin or global `corePtr`. The management protocol is v2.

Build from this repository, using its pinned Go version:

```sh
. ./build-tags.sh
CGO_ENABLED=0 go build -tags "$(tags_for agent)" -ldflags "$(ldflags_for agent)" -o s-ui-node ./cmd/s-ui-node
go test -tags "$(tags_for agent)" ./nodeagent ./cmd/s-ui-node
```

Run with a private persistent directory, an explicit node ID and a dedicated
management identity. The certificate must have a SAN matching the address used
by the controller; the controller certificate needs client-auth usage. Keep the
CA private key on the issuing/controller side.

```sh
./s-ui-node --node-id 1 --state /var/lib/s-ui-node \
  --listen 127.0.0.1:16443 \
  --tls-cert /run/credentials/server.pem \
  --tls-key /run/credentials/server-key.pem \
  --client-ca /run/credentials/controller-ca.pem
```

There is no insecure HTTP mode. Use TLS 1.3 with a verified controller client
certificate. Listener exposure, firewall and certificate provisioning belong to
deployment; the default control listener is loopback. The tested systemd profile
uses DynamicUser, StateDirectory, LoadCredential, NoNewPrivileges and a read-only
system filesystem. S-UI's Linux core also needs AF_NETLINK to discover interfaces.

## Management protocol v2

- `GET /v2/status`: node ID, registration epoch, independent spec/access
  revisions and digests, durable and applied access, running/error and public
  certificate metadata. This response contains no client credentials.
- `PUT /v2/spec`: structural sing-box configuration without inbound `users`.
- `PUT /v2/access`: complete client grants with absolute expiry, credentials and
  whole-node scope (`all: true`).
- Both payloads contain `node_id`, `epoch` and a positive `revision`. Revisions
  advance independently. A foreign identity/epoch, stale revision or changed
  payload at the same revision returns 409. Exact retries are idempotent.
- Invalid JSON returns 400; an unapplied state returns 422 with a sanitized
  message. A successful response confirms the applied revision and digest.

Example shape only; placeholders below are not working credentials:

```json
{
  "node_id": 1,
  "epoch": "<PINNED_AGENT_EPOCH>",
  "revision": 1,
  "config": {
    "inbounds": [{
      "type": "vless",
      "tag": "node-vless",
      "listen": "::",
      "listen_port": 14443
    }],
    "outbounds": [{"type": "direct", "tag": "direct"}]
  }
}
```

`config` is server configuration; client users arrive separately through access.
Panel clients use local `inbounds: [21]` and whole-node `nodes: [1]` independently.
`nodeinbounds` is a read-only list of applied remote listener IDs. Subscriptions
contain client outbounds and connection URIs.

## Runtime and recovery

Access intent is saved before runtime application. Users-only changes use hot
updates; removed users, replaced credentials and changed access generations close
old sessions. A structural change rebuilds the core. Recovery combines last-good
structure with the latest access, never its obsolete user list. If safe recovery
fails, the agent stops its own core and retains the access intent.

Each rebuilt instance has a fresh cancellable context. Stopping it cancels its
asynchronous certificate-provider jobs, so a retired ACME job cannot continue
using a closed DNS/HTTP manager. Hot user changes keep the provider context.

Certificate resources are separate from management TLS. HTTP-01 issuance and
remote file validation run on the agent; managed DNS-01 issuance runs on the
panel so the DNS credential stays there. See [node-resources.md](node-resources.md)
for resource methods, path rules and subscription readiness.

`state-v2.json` contains last-good structure, latest durable access and applied
access, with a checksum. Updates use a temporary 0600 file in a 0700 directory,
file fsync, atomic rename and directory fsync. Application is acknowledged only
after the applied checkpoint succeeds. Failure never restores an older ACL.

Back up the whole state directory and upgrade the panel before the agent. Managed agents
follow the panel's pinned sing-box version and update through the saved SSH
identity. OS-reinstall recovery uses the explicit [installer](node-installer.md)
workflow; automatic downgrade remains unsupported.

There is no heartbeat deadline that shuts down the VPN. Known absolute user
expiries are checked locally once per second. A new central revocation while the
controller is unreachable can only arrive after reconnection. Per-client traffic
is counted towards the shared panel quota after the controller receives it.

The automated checks cover access/session revocation, restart recovery,
structural/runtime/checkpoint failures and mTLS integration. Graceful restarts
and forced power loss have different guarantees; the traffic ledger can lose
bytes since its latest durable checkpoint. See [maintenance](nodes-maintenance.md)
for shared-quota and offline behavior.
