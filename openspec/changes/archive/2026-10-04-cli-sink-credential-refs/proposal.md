# Proposal

## Why

`kollect-pipeline` loads the same `KollectSnapshotSink` resource as the operator, but resolves less
of it. `ResolveSinkSecretData` (`internal/pipeline/wire.go:80`) reads only `spec.secretRef`, and
`cliBuildContext` (`wire.go:187`) never sets the CA. So in the CLI:

- a git sink whose credentials are in `spec.git.auth.secretRef` pushes with no credentials. With
  `spec.secretRef` also set, the CLI uses `spec.secretRef`, while the operator uses the git one;
- `spec.tls.caSecretRef` is dropped without a message. The backend falls back to the system roots
  and fails TLS against a private CA, which looks like a server problem.

The operator resolves both (`internal/sink/build_context.go:28`). The same manifest therefore
behaves differently in the two modes, and no test spans config directory → resolved credentials →
constructed backend. Existing tests stop at the resolver helper or use a factory that ignores
what it is given.

## What Changes

- The CLI resolves sink credentials the way the operator does: `spec.secretRef`, then the
  type-specific override (`git.auth.secretRef` for git), and the CA from `tls.caBundle` or
  `tls.caSecretRef` (keys `tls.crt`, `ca.crt`, `ca.pem`), all from Secret manifests in the config
  directory, with the existing `${env:VAR}` substitution.
- A referenced Secret that is not in the config directory fails the run before the backend is
  built and names the reference. Nothing is silently dropped.
- Tests cover the CLI boundary from a config directory to the backend, and the CLI is checked
  against the operator's own resolver on the same spec and Secrets.

## Capabilities

### New Capabilities

- `pipeline-sink-credentials`: how `kollect-pipeline` turns the credential and CA references of
  its sink into what the sink backend receives, and when it must refuse.

### Modified Capabilities

None.

## Impact

- Entry point: `kollect-pipeline collect --config <dir>` (`cmd/cli/collect.go:109-126`), through
  `pipeline.ResolveSinkSecretData` and `cliBuildContext` to `Registry.NewBackend`.
- Behaviour change for users: a config whose `git.auth.secretRef` or `tls.caSecretRef` names a
  Secret that is not in the config directory used to run, without credentials or CA. It now fails
  with a clear error. A config with both `secretRef` and `git.auth.secretRef` now pushes with the
  git one, which is what the operator does.
- Changelog-worthy fix (`fix(pipeline)`), no API or CRD change.

## Non-goals

- Database, Kafka and NATS sinks in the CLI. The CLI loads only `KollectSnapshotSink` (git,
  gitlab, s3, gcs), so their override paths cannot be reached from a config directory. Tracked
  separately.
- Changing the operator's resolution, the backend pool or the admission webhooks.
- A CLI Secret matching rule that differs from today's (name, plus namespace when the reference
  sets one).

## Assumptions

- The operator's order is: resolve `spec.secretRef` if set (missing is an error), then replace the
  data with the type-specific reference when set (`build_context.go:46-58`). The CA comes from
  `tls.caBundle` first, else `tls.caSecretRef` with keys `tls.crt`, `ca.crt`, `ca.pem`
  (`build_context.go:215-245`). Read from source at a7bd368e2.
- The git backend exposes the CA it was built with through `Backend.Config().CABundle`
  (`internal/sink/git/backend.go:49`, `config.go:75-82`); an inline `caBundle` already reaches it
  without the resolver.
