# Node domains and certificates

Domain and certificate resources have their own revisions. SSH onboarding and recovery use the same resource settings as template application.

## API

Both authenticated API handlers expose `GET node-resources`, returning `dnsprofiles`, `domains`, `certificates` and `dnsrecords`. Secrets, account keys, PEM material and encrypted delivery payloads are omitted. These are administrator endpoints, not subscription endpoints.

`POST node-resource` uses form fields `resource`, `action`, and a JSON string in `data`. Resources are `dnsprofiles`, `domains`, `certificates`. Saving requires the current `expected_version` (zero on first creation); the returned projection contains the new version or generation. An unchanged domain save is a no-op. A successful save confirms intent, not remote application.

- DNS profile: `action=save`, `id`, `name`, `auth_type=token|global_key`, `zones=[{"id":"<zone-id>","name":"example.com"}]`. Supply `token`, or `api_key` plus `email`. Omitted credentials preserve the existing secret only when authentication identity is unchanged. `action=del` refuses referenced profiles; removing a referenced zone is also refused.
- Domain: `action=save`, `node_id`, `mode=manual|managed`, `ip`, `certificate_mode=auto|provided|remote_paths`. Manual mode takes `fqdn`; it never edits provider DNS. Managed mode takes `profile_id`, `zone_id`, and `prefix`, or `automatic_prefix=true`. Random prefixes persist across retries. Omitted IP preserves the configured address, then defaults to the physical node host on creation.
- Certificate: `action=save`, `node_id`, `mode=auto|dns01|http01|provided|remote_paths`. Auto selects panel DNS-01 when a profile is available, otherwise node HTTP-01. Explicit DNS-01 takes `dns_profile_id` and `zone_id`. ACME accepts `ca=letsencrypt|staging` and optional `email`. Provided mode takes `certificate` and `key` PEM. Paths mode takes absolute `certificate_path` and `key_path` on the node. Public trust is still required; staging certificates are not treated as trusted production endpoints.
- Certificate `action=renew` takes `node_id` and `expected_version`; it queues a new issuer generation for DNS-01/HTTP-01. This does not claim that the new certificate is already serving.

`POST node-resource-retry` takes `data={"node_id":1}`. `POST node-dns-cleanup` additionally accepts `record_id`; omitted ID selects all owned records for that node. Cleanup refuses the current managed address of an enabled node. Node deletion cleans up owned DNS records after its other checks pass, or accepts explicit confirmation that they were already removed.

## Resolution and readiness

Shared templates refer to the per-node certificate without storing its PEM, domain or provider token. A `node_certificate` source named `primary` can supply `/tls`, certificate/key paths, or the virtual `/tls/certificate_binding`. The virtual binding preserves other TLS options and an explicitly supplied SNI, and supplies the node domain when SNI is absent. `node_parameter/primary_domain` supplies the node domain explicitly. REALITY handshakes remain independent of node leaf certificates.

DNS-01 runs only on the panel. The panel encrypts account keys, provided material, certificate versions and pending delivery payloads with the external state key. HTTP-01 runs on the node independently of the VPN application lock and can renew from persisted configuration while the panel is unavailable. HTTP-01 requires public TCP 80; a busy listener is reported as `http01_port_occupied` and is never stopped by the worker.

Every resolved A/AAAA address must equal the configured IP. The current binding stores one expected address; simultaneous distinct IPv4 and IPv6 addresses need a later address-set model. Existing foreign records, proxy-enabled records and ownership mismatches are refused. An owned intent is persisted before provider mutation, allowing recovery after a lost response without creating duplicates. Old managed names remain visible for explicit owned-record cleanup; IP/OS handover belongs to the installer workflow.

Certificate delivery uses its own immutable revision and digest. The agent validates name, key pair, time and the system trust chain, then installs private versioned files atomically. An installed artifact is separate from a certificate actually used by the core. New endpoints are published only after spec/access confirmation and a ready, unexpired core TLS report. Resource state `installed` alone does not publish an endpoint.

Node-issued and node-file certificates use a logical `managed:primary` reference. A successful renewal is tried with the latest client access; a failed activation keeps the prior active artifact and cannot restore revoked clients. Failed issuance retains the old valid material and exposes an error. Expired certificates are excluded from publication. Remote file mode watches supplied files; renewing those external files remains the administrator's responsibility.

Cloudflare is the supported managed DNS provider. Retry DNS cleanup if the provider is unreachable. Back up the agent state and the panel's full archive for recovery. The agent resolves the state directory selected at startup and rejects symlinks in certificate artifact paths beneath it.
