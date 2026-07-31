# Resolved-address policy for sink connections

Kollect treats every configured sink endpoint as untrusted network input. By default, admission
rejects literal loopback, private, link-local, metadata, and `file://` targets. Outbound sink clients
repeat the address decision at connection time: they resolve the hostname for each new connection,
reject the complete answer set if any address is forbidden, and open the socket to an authorized
numeric address. This reduces DNS rebinding between admission and connection and applies again when
HTTP redirects or backend reconnects create a new socket.

## Default (secure by default)

The **default** policy denies private, loopback, link-local, metadata, and `file://` targets.
Weakening the guard through DNS aliases, `HTTP_PROXY`, or per-workload environment variables is
unsupported. The `integration` Go build tag uses a compile-time-only permissive dialer so local
testcontainers can exercise real protocols; that code is absent from production builds.

## Production opt-in (NET-01)

Operators who need **cluster-local** sinks (RFC1918 / ULA ClusterIP services such as in-cluster
Postgres, MinIO, or NATS) on a **published production image** have an explicit, secure-by-default
**OFF** opt-in:

| Control | Shape | Notes |
| --- | --- | --- |
| Manager flag | `--allow-private-sinks` | Bool; **default `false`** |
| Helm value | `allowPrivateSinks` | Renders the flag only when `true` (cluster-admin install surface) |

**Not** a CRD / tenant field — sink authors cannot widen the deny-list. When enabled,
admission (literal ClusterIP) and dial-time netguard (literal + DNS→RFC1918/ULA) apply the
**same** decision. Metadata hostnames, link-local, loopback, and `file://` remain denied even when the
opt-in is on (narrower than the `integration` dialer’s full private skip).

**Residual SSRF risk:** a principal who can create or edit sink CRs can then aim connection-tests and
exports at RFC1918 / VPC peers reachable from the manager pod. Enable only in environments that accept
that trade-off (typically private clusters with trusted CR authors). Shipped per **NET-01**
(DR-FIND-05 Option A, operator decision 2026-07-31).

With `allowPrivateSinks` unset (the default), production images keep deny-by-default with **no**
runtime hatch.

Git transports follow the same rule. The pure-Go HTTP transport uses the guarded dialer, Git CLI
HTTP operations pin libcurl with `http.curloptResolve`, and SSH operations pin the checked numeric
address while retaining the original hostname for host-key verification.

Pure-Go HTTP disables inherited proxy use. Git CLI processes can inherit proxy variables from the
manager environment, so proxy-based weakening is unsupported but not centrally prevented. Do not
inject proxy variables into the manager; enforce approved destinations with an egress
`NetworkPolicy`. See [Security architecture and controls](security-architecture.md#residual-risk).
