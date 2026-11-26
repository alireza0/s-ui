# Node interface

The Nodes page uses the versioned whole-node model. Use **Add node** to connect
a VPS through the [SSH installer](nodes-quickstart.md). Open the node from its
card to manage its server access, recovery, and configuration.

1. In **Inbound templates**, create one of the supported client proxy inbounds
   listed in [nodes-model.md](nodes-model.md), using its protocol, Listen,
   Transport, Multiplex and TLS editors where applicable. Select a stable
   per-node port or an explicit port, and the node domain or a manual SNI.
   Certificates and REALITY keys belong to each node, not to the template.
2. Open a node. Set its domain and certificate under **Server**, then select
   and apply one or more inbound templates under **Inbounds**.
   Managed DNS uses a Cloudflare profile and zone; manual DNS can still use
   automatic HTTP-01. Uploaded PEM is write-only, and file paths refer to the
   node. Port 80 must be free for HTTP-01; existing web services are not stopped.
3. Configure node-specific fields with **Settings for this node**. Explicit
   overrides remain separate from the template. **Use template values** removes
   those overrides. Name, enabled state and client TLS are edited in the
   template. Changing the selection preserves the existing connection keys
   and individual values for entries that remain selected.
4. Assign **Nodes** in the client editor or bulk editor. Local inbound selection
   is separate. All enabled confirmed connections of each assigned node appear
   automatically; changing a template does not require reassigning clients.
5. Open the client's QR dialog and use **Copy Clash URL** or **Open Clash
   subscription**. This is the generated subscription, not a static link list.

Saved, resolved, installed and actively used states are distinct. The node page
shows template/configuration/access versions, resource errors and certificate
observations. New endpoints are withheld until confirmed. A stale editor save
is rejected and retains the draft; reopen it to load the latest version.

For API consumers, `GET node-workspace?id=<node-id>` is a read-only projection
of the target spec, explicit overrides, redacted prepared connections and task
states. It does not materialize or deploy anything. `POST node-dns-zones` accepts
a form `data` JSON with `id`, `auth_type`, optional `email`, and new `token` or
`api_key`; omitted credentials reuse the same stored profile identity. It lists
available zones without persisting credentials. Both cookie and token APIs
authenticate these endpoints. Resource writes return the updated redacted
resource bundle; see [domain/certificate API](node-resources.md).

Schema-1 entries accept optional `client_tls` for public client settings such as
uTLS fingerprint. Private keys, client file paths and inline secret material are
rejected. Server-derived TLS identity remains authoritative; REALITY public keys
are derived separately for each node.
