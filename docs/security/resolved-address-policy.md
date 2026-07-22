# Resolved-address policy for sink connections

Kollect treats every configured sink endpoint as untrusted network input. Admission rejects literal
loopback, private, link-local, metadata, and `file://` targets. Outbound sink clients repeat that
decision at connection time: they resolve the hostname for each new connection, reject the complete
answer set if any address is forbidden, and open the socket to an authorized numeric address. This
prevents DNS rebinding between admission and connection and applies again when HTTP redirects or
backend reconnects create a new socket.

The default policy does not provide an `allowPrivate` escape hatch. Private-address sinks therefore
require an explicit future operator policy decision; weakening the guard through DNS aliases,
`HTTP_PROXY`, or per-workload environment variables is unsupported. The `integration` Go build tag
uses a compile-time-only permissive dialer so local testcontainers can exercise real protocols; that
code is absent from production builds.

Git transports follow the same rule. The pure-Go HTTP transport uses the guarded dialer, Git CLI
HTTP operations pin libcurl with `http.curloptResolve`, and SSH operations pin the checked numeric
address while retaining the original hostname for host-key verification.
