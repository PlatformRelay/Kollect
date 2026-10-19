# PR description — sink-ca-trust

Draft; the hand-off stage completes the rest of this file.

## Release note (v0.22.0)

S3, GCS, Postgres and MongoDB sinks now trust `spec.tls.caBundle`/`caSecretRef` in addition to the
system roots. On those types, a CA that cannot take effect now fails the sink instead of being
ignored: an `http` endpoint, a Postgres `sslmode` set explicitly to `disable`, `allow` or `prefer`,
another root certificate source (`sslrootcert` in the DSN, a service file, `PGSSLROOTCERT` or
`~/.postgresql/root.crt`), a MongoDB `tlsCAFile`, a plaintext URI, or `tlsInsecure`/`sslInsecure`.
On every sink type, Kafka and NATS included, a `caSecretRef` Secret without a CA key (`tls.crt`,
`ca.crt`, `ca.pem`) or a spec that sets both `caBundle` and `caSecretRef` now fails the sink.
CA-configured Postgres sinks without an `sslmode` now use `sslmode=verify-full`, verifying the
server including the hostname: a DSN host that does not match the certificate SAN now fails; set
`sslmode=verify-ca` explicitly to verify the chain only. These errors are terminal: after fixing a
Secret, touch the sink (change any annotation, for example
`kubectl annotate <kind> <name> ca-fixed-at="$(date +%s)" --overwrite`) so it is retried. See the upgrade
guide, *Sink CA trust (after v0.21.0)*.
