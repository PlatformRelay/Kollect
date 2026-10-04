# Design

## Context

See proposal.md. Credentials change, so the workflow asks for a design, adversarial cases and a
negative control.

## Goals / Non-Goals

**Goals:** the CLI and the operator turn the same snapshot sink and Secrets into the same backend
input; a reference that cannot be resolved stops the run.

**Non-Goals:** sharing one implementation between the two paths in this change; database, Kafka
and NATS sinks in the CLI (unreachable from a config directory); the operator's own behaviour.

## Decisions

- **Mirror the operator's order in the CLI instead of running the operator's resolver on an
  in-memory client.** `BuildContextFromSpec` takes a `client.Reader` and defaults the namespace
  of an unqualified reference to the sink's namespace. The CLI matches a Secret by name, and by
  namespace only when the reference sets one. An adapter would either change which Secrets CLI
  users' configs match, or need a reader that guesses whether a namespace was defaulted. Neither
  is acceptable in a fix.
- **Parity is a test, not a shared code path.** The test feeds both resolvers the same spec and
  Secrets (for the operator, through a fake client with the Secrets in the sink's namespace) and
  requires equal results. If either side changes its order, the test fails. This is the
  independent model: the operator's resolver is the oracle, not a copy of the expected values.
- **The CA is resolved in `cliBuildContext`**, after collection and before the backend is built.
  Credentials stay in `ResolveSinkSecretData`, which runs before collection, so a missing
  credential still fails before any cluster call, as today.

## Risks / Trade-offs

- [Two implementations of one order can drift] → the parity test covers every combination of the
  four references, so drift fails CI.
- [Configs that relied on the silent drop now fail] → intended: they were pushing without
  credentials or verifying TLS against the wrong roots. The error names the missing Secret, and
  the fix is called out in the changelog.
- [Secret matching by name only] → unchanged from today; out of scope.
