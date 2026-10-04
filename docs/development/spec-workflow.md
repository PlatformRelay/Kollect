# Spec workflow

New behavioural work in Kollect goes through [OpenSpec](https://github.com/Fission-AI/OpenSpec),
pinned to one version in `hack/tooling/openspec/package.json`. This page is the contract. The
OpenSpec configuration in `openspec/config.yaml` repeats the rules an author needs while writing a
change.

| Location | Holds |
| --- | --- |
| `openspec/specs/<capability>/spec.md` | Current requirements and scenarios, one file per durable capability |
| `openspec/changes/<change>/` | One change in flight: proposal, requirement deltas, design (when needed), tasks with verification |
| `openspec/changes/archive/` | Completed changes, moved there by `openspec archive` |
| `specs/001-*` | Historical bundles, read-only. See [`specs/README.md`](https://github.com/PlatformRelay/Kollect/blob/main/specs/README.md) |
| `docs/adr/` | Durable architecture decisions. Changes link to them rather than copy them |

Small fixes do not need a change: a typo, a dependency bump, a CI tweak or a documentation
correction goes straight to a PR. Open a change when observable behaviour changes.

## Lifecycle

1. **Frame.** In `proposal.md`, write the outcome, the production entry point, non-goals and the
   assumptions the change depends on. Back each load-bearing assumption about a tool or API with a
   source or a probe.
2. **Specify.** Write requirements with stable IDs, plus scenarios for both what must happen and
   what must not happen. Write the verification table in `tasks.md` before touching production
   code.
3. **Red.** Write the behavioural test and watch it fail on the assertion you care about. A
   compile error, a missing Docker daemon or a skipped test is not a red.
4. **Green.** Make the smallest coherent change and run the relevant gates.
5. **Review.** An independent reviewer (not the author) checks the exact revision against the
   requirements and the verification table.
6. **Land.** Merge only with green required CI (rebase merge). Then sync the living spec and
   archive the change.

These are separate states: implemented, tested, reviewed, merged, archived. Archiving is history,
not proof. If a change is archived before its PR merges, the work stays open until the merge is
verified.

## Verification scales with risk

The [gates](testing.md) apply to every PR. On top of them:

| Change | Additional verification |
| --- | --- |
| Prose only | Content, source and link checks. No invented behavioural test |
| Bounded local behaviour | Positive and negative test, with a red→green record |
| Stateful export or configuration | A test through the production path, the relevant state transitions, and the race detector on the changed packages (`go test -race`, or `task coverage:race`) |
| Deletion, tenancy, credentials, migrations | An isolation or ownership invariant, adversarial cases, and a mutation or negative control that shows the test catches the defect |

Identity, persisted metadata, compatibility and migration changes need a `design.md`.

## Verification table

`tasks.md` ends with a table that maps each requirement to its evidence:

```markdown
## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEE-1 | `TestRequireAll_missingTest` | run fails, names the missing test | pass | rev `abc1234`, `go test ./hack/...`, CI run link |
| IEE-4 | mutation: drop the exit-status check | the test above fails | pass | rev `abc1234`, local, Go 1.26.6 |
| IEE-5 | real Docker integration run | expected = executed, 0 skipped | not-run | needs CI |
```

Status is one of `pass`, `fail`, `blocked`, `not-run` or `N/A` (with a reason). Evidence names the
revision, the command, what ran and what was skipped, the tool versions and the known limitations.
Link CI runs instead of pasting logs. When a test found a failing input (a seed or a fuzz corpus
entry), keep that input as a regression fixture.

## Commands

```bash
task spec:list                  # changes in flight; add -- --specs for capabilities
task spec:validate              # openspec validate --all --strict, then check OpenSpec loaded config.yaml
task spec:cli -- new change <name>
task spec:cli -- archive <name>
```

The tasks install the pinned CLI with `npm ci` and set `DO_NOT_TRACK=1` and
`OPENSPEC_NO_UPDATE_CHECK=1`, so they make no telemetry or update-check calls.
