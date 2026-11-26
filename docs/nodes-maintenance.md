# Node maintenance

## Backup and restore

Download a new full backup after changing node settings or client access.
Every download uses a fresh coherent database snapshot, including pending node
settings, template versions, access, certificate resources and traffic cursors.
The ZIP also carries the node recovery key; do not publish it.

Restoring a valid backup replaces the panel's saved state with the archived
state. Higher node revisions in the destination database do not block import.
An older supported database is migrated to the current panel version; the full
ZIP carries the key needed on a fresh panel. See
[installer backup](node-installer.md#backup) for integrity and key checks.

The import marks restored node intent for one-time reconciliation. After an
authenticated status check, the panel sends that intent at a new revision above
the agent checkpoint, then consumes the mark transactionally. The agent keeps
rejecting replayed old commands. Do not edit revision numbers or agent state
files. Keep only one panel controlling a transferred node set at a time.

## Control certificates

For nodes managed by the SSH installer (model 2), the panel checks the control
CA, controller certificate and agent certificate every minute. It starts renewal
when any identity has less than 30 days remaining or fails validation. Failed
attempts are retried at most once per hour. Unfinished jobs resume after a panel
restart using their encrypted installation plan.

Renewal uses the saved dedicated SSH key and pinned server host key. It follows
the normal agent update procedure, including a brief service restart. Node and
inbound identities, protocol keys, client credentials and subscription URLs are
preserved. The agent waits for fresh authenticated client access before starting
the saved VPN configuration after installation.

Renewal does not replace a changed SSH host key or recover a reinstalled OS.
Use explicit node recovery with new SSH credentials for that case. Installation
errors appear in the node's existing job history. Manual control deployments
require separate certificate maintenance.

## HTTP-01 firewall permissions

HTTP-01 requires incoming TCP port 80. The panel opens this permission through
its privileged SSH connection when UFW or firewalld is active, before requesting
a certificate. The agent does not receive firewall administration privileges.
Provider firewalls must allow the challenge traffic separately.

The HTTP-01 permission is tracked independently from VPN listener ports and
remains available for autonomous certificate renewal. Changing VPN protocols
does not remove it. The panel releases its permission when the node is disabled,
deleted or retired, or after a replacement certificate method is confirmed.
Existing administrator rules and permissions still used by other managed
listeners are retained.

## Interrupted installation

A new installation operation requires a fresh authenticated access snapshot.
While that bootstrap is pending, structural delivery waits and retries instead
of permanently rejecting the configuration. An expired worker lease can delay
this retry; the installer continues verification without recreating clients or
their credentials.

## Runtime updates

Linux releases and container images include the amd64 and arm64 agent bundle.
The built-in Linux updater verifies the new bundle before switching the panel
binary. After restart the panel uses the new bundle to update managed nodes.
An explicit `SUI_NODE_AGENT_DIR` remains an administrator-managed bundle path.

Unchanged user lists do not invoke the protocol's hot-update interface. When an
inbound cannot update users in place, the agent rebuilds the runtime with the
current access list. Supported inbounds continue to use in-place updates.

A rejected certificate activation retains the active certificate. A successful
retry clears that activation error without hiding a newer resource error.

## Removing a reinstalled server

Disable the old node record first. If the OS was reinstalled or the agent was
stopped manually, its shutdown acknowledgement cannot arrive. The delete dialog
offers a separate confirmation for that case. Confirm only after stopping the
old server: removing a record cannot stop a disconnected agent.

This confirmation does not bypass active installation/resource jobs, unfinished
retirement of another server or DNS ownership checks. Deletion removes the
saved SSH identity, grants and control state. Adding the same address later
creates a fresh identity and does not inherit the deleted node's assignments.

## Client Last online

Node agents sample authenticated user traffic every second, including while
the panel is unreachable. The panel merges that activity into the existing
client Last online timestamp when communication resumes. Older observations
cannot move the timestamp backwards. User activity is recorded independently
of traffic-chart settings. Remote user traffic also contributes to the client's
existing upload/download counters and quota.

## Shared client quota

Each agent retains cumulative per-client byte counters in a private, atomically
written ledger. The ledger survives ordinary agent restarts. The panel commits
each node's cursor and the corresponding client usage in the same transaction.
Repeated polls, delayed smaller samples and a panel restart do not charge the
same bytes twice. Different nodes add independent deltas, including concurrent
connections. Quota resets preserve cursors so old samples are not charged again.
The normal S-UI deplete job disables the client locally and on all assigned nodes.

Accounting starts when the upgraded agent receives a metered access snapshot;
previous unmetered remote traffic is not reconstructed. Graph collection is
independent of quota collection. Existing agents upgrade through the panel's
normal agent update path.

Enforcement follows the panel's polling/depletion interval. An offline node
continues serving the last saved configuration; it reports accumulated usage
after reconnection. A shared hard byte limit during a panel outage is not
provided, so simultaneous sessions and reporting delays can exceed the quota.
A forced power loss can lose traffic since the last one-second checkpoint.
Bytes received after a quota reset are charged to the current period, even if
they were accumulated offline before the reset.
