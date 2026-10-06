# Spec Delta

## ADDED Requirements

### Requirement: GTE-1 The git sink has one export engine, go-git

The git snapshot sink SHALL export and delete through the go-git engine only. `spec.git.engine`
SHALL accept `go-git` (the default) and SHALL reject `cli` — at the CRD schema, at admission
validation, and at backend construction — with an error naming `go-git` as the engine. The
`file://` remote handling and the `git ls-remote` connection probe SHALL keep using the CLI
machinery they already use for both engines.

#### Scenario: engine=cli is rejected at admission

- **WHEN** a `KollectSnapshotSink` with `spec.git.engine: cli` is created or updated
- **THEN** admission fails naming `go-git` as the engine
- **AND** backend construction independently rejects the same value (defence in depth)

#### Scenario: go-git and default are accepted

- **WHEN** a sink sets `spec.git.engine: go-git` or omits the field
- **THEN** export and delete behave as the go-git engine does today

#### Scenario: file:// remotes are unchanged

- **WHEN** a git sink targets a `file://` remote (any engine value it carried)
- **THEN** export, delete and connection probing use the CLI machinery, as today

### Requirement: GTE-2 The go-git SSH client offers the key exchanges x/crypto implements

The go-git SSH key-exchange offer SHALL include, at minimum, the current list plus
`mlkem768x25519-sha256` and `diffie-hellman-group16-sha512`, in an order that keeps the existing
algorithms first. This closes the documented "SSH/KEX edge cases" gap: the algorithms are already
implemented by the pinned x/crypto; the offer, not the library, was the limiter.

#### Scenario: KEX list carries the modern algorithms

- **WHEN** the go-git SSH auth is built
- **THEN** the client's key-exchange offer contains `curve25519-sha256`, `mlkem768x25519-sha256` and `diffie-hellman-group16-sha512` (non-exhaustive)

#### Scenario: Existing preferences are not demoted

- **WHEN** the list is compared to today's
- **THEN** the eight existing algorithm names keep their relative order

### Requirement: GTE-3 The documented engine contract matches the code

`docs/crds/kollectsnapshotsink.md` and the API field description SHALL describe `go-git` as the
engine, SHALL NOT claim `cli` is required for SSH/KEX edge cases, and SHALL note the convergence
(ADR-0803). The pipeline image's documented engine support statement stays true.

#### Scenario: CRD reference does not advertise cli

- **WHEN** a user reads the snapshot sink CRD reference for `spec.git.engine`
- **THEN** the only admitted value documented is `go-git`, with the convergence rationale linked
