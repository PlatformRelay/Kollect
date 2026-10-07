# ADR-0803: The git snapshot sink converges to one export engine, go-git

> One export engine for the git snapshot sink: `engine: cli` is removed from the API surface; the
> git CLI machinery survives only where it is genuinely shared (file:// remotes and the
> ls-remote connection probe).

**Theme:** 04 · Export & sinks (numbered in the 08 range: 0802 is reserved by the pipeline-CLI
contract, see Notes) · **Status:** Accepted (2026-10-06)

## Context

`spec.git.engine` offered `go-git` (default) and `cli`. The CLI engine existed for an assumed
class of SSH/KEX edge cases, but a verification pass of the product decisions behind it found no
auth mode that genuinely needs it:

1. The pipeline image ships no git binary, so the CLI engine cannot serve pipeline mode — one
   engine means go-git.
2. The go-git SSH key-exchange offer is an in-repo pin, and the pinned x/crypto already implements
   `mlkem768x25519-sha256` and `diffie-hellman-group16-sha512`; the CRD doc's "cli required for
   SSH/KEX edge cases" reduces to a stale pin plus a transitional-algorithm residue
   (sntrup761-only servers) with no in-repo evidence of users.
3. `file://` remotes and `git ls-remote` connection probes call the CLI machinery for both engines
   today, so `exec_git.go`/`cli_env.go`/`export_file.go` stay regardless — removing them was never
   on the table.
4. [ADR-0104](0104-security-model.md) documents the CLI SSH path as the weaker host-key story (no
   fail-closed guard), so making the CLI the only engine would regress the documented security
   model.

## Decision

The git snapshot sink exports and deletes through go-git only:

- the CRD field marker becomes `+kubebuilder:validation:Enum=go-git`, and the exported
  `GitEngineCLI` API constant is removed;
- admission validation rejects any `spec.git.engine` other than `go-git` (including `cli`),
  naming `go-git` as the engine;
- backend construction rejects it independently (defence in depth): a persisted object still
  carrying `engine: cli` fails at its next export build with an error naming `go-git`; the
  operator never deletes or rewrites the object;
- the internal `GitEngine` type, `Config.Engine` field and the engine branch points
  (`isFileRemote(...) || engine == cli` in the export and delete paths) are removed: the delivery
  split is `file://` remote vs everything else;
- the go-git SSH key-exchange offer is extended with the two algorithms x/crypto already
  implements, appended so the existing eight keep their relative order;
- every in-repo site asserting `engine: cli` works is truthed up in the same change (CRD
  reference, chart README template and regenerated README, operator manual engine table,
  coding-standards container-build MUST line, security architecture engine sections, operator
  Dockerfile comments, [ADR-0415](0415-git-sink-commit-ergonomics.md) commit-ergonomics sentence);
  `docs/operator-manual/upgrading.md` names the persisted-sink behaviour.

### Consequences

- The CLI delivery machinery's persistent warm-mirror handling becomes unreachable in production
  (file:// remotes get a fresh workdir per operation). The per-engine probe/delivery function
  pairs inside the shared machinery are hoisted/trimmed in a later sweep, not here — the branch
  points that made them drift are gone.
- The integration-tier regression locks that pin the (CLI machinery × persistent mirror) surface
  were removed with that surface; the go-git twins keep pinning the surviving persistent-mirror
  machinery.
- Operators with a persisted `engine: cli` sink must remove the field before that sink exports
  again; see the upgrade note. SSH sinks converting to go-git need `known_hosts` in the sink's
  credential secret (the go-git path fails closed where the CLI path left host-key policy to the
  ambient ssh configuration).

## Alternatives considered

- **Keep `cli` as an opt-in**: rejected — no verified auth mode needs it, it doubles the
  export/delete/probe surfaces to keep green, and the pipeline image cannot serve it.
- **Make `cli` the only engine**: rejected — it would regress the fail-closed SSH host-key model
  of [ADR-0104](0104-security-model.md) and keep the git binary as a hard runtime dependency of
  the operator image.
- **Deprecate without removing**: rejected — pre-v0.x API, no deprecation window to buy;
  soft-deprecation would keep both engines' behaviours "green" while only one is served.

## Notes

- ADR number 0802 stays reserved: eight pipeline code comments already cite "ADR-0802" as the
  future pipeline-CLI contract, so this ADR takes 0803 to avoid squatting on it.
- GTE-1..GTE-3 in
  `openspec/changes/2026-10-06-product-decision-convergence/specs/git-engine/spec.md` are the
  requirement form of this decision.

## References

- [ADR-0407](0407-git-object-store-layout.md) — git export layout and workflow
- [ADR-0415](0415-git-sink-commit-ergonomics.md) — commit ergonomics
- [ADR-0104](0104-security-model.md) — security model (SSH host-key fail-closed guard)
- [ADR-0801](0801-pipeline-cli-mode.md) — pipeline CLI mode
