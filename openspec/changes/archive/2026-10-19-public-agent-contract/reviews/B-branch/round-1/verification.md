# Stage B round 1: verification

Reviewed revision: `73e92c8c4` (the whole branch against `origin/main` @ `e9a9779c5`). Legs:
`leg-astra-adversarial.md` (brief `brief-astra.md`), verdict BLOCK; `leg-claude-opus-spec.md`
(`fanout.sh --legs claude/opus:spec --no-unify`), verdict CONCERNS. Neither leg wrote
`AGENTS.md` or any part of the branch. Holistic run: `holistic.md` (green).

Every finding below was checked by the orchestrator of stage B against the tree, not taken from
the leg.

| Id | Finding | Leg sev | Verified? | Sev after check | Disposition |
|---|---|---|---|---|---|
| A1 / O1 | This change's records publish machine-local and harness detail: the absolute worktree path and harness script paths in `task-prompt.md:3,7,15` and `loop.md:3`, local harness file counts in `evidence/G1.md:33`, reviewer and model names in records and in two commit messages (`ec288c216`, `3cb6d45df`) | astra P0, opus WARNING | yes: the paths and names are there; the same pattern is on `main` in six earlier change folders, so nothing here is newly public; `tasks.md:133` is a test literal (`/home/someone/x`), not a leak | WARNING | fix in part: the absolute home and harness paths in `task-prompt.md` and `loop.md` become placeholders (tasks.md 5.6). Not fixed, for the operator: whether loop and review records that name reviewers and models belong in the public tree at all (a repository-wide question), and the two commit messages that name reviewer models, which only a history rewrite changes. A rewrite was tried and reverted: it rewrites reviewed commits and needs the operator's go-ahead |
| A2 | Data loss: a checkout with an ignored private root `AGENTS.md` loses it on pull, and nothing tells a contributor before pulling | astra P0 | yes: in a scratch repository, merging a commit that tracks an ignored `AGENTS.md` replaced the local file without a warning (exit 0). This host is covered by 0.2 and the operator's other machines by the INBOX notice (present, checked); other contributors are not | WARNING | fix: `docs/development/setup.md` says to rename such a file to `AGENTS.local.md` before pulling (tasks.md 5.5) |
| A3 | `paths = [""]` in `.github/gitleaks.toml` exempts every file from gitleaks, and the guard skips empty regexes (`agent_contract_test.sh:478`) | astra P1 | yes: the empty line is skipped; an empty pattern matches every path (`grep -P ''`), and gitleaks compiles every path entry | WARNING (fail-open) | fix with a mutant (tasks.md 5.1) |
| A4 | An SSH `Host` line with four or more aliases passes PAC-6: the pattern allows at most three names (`agent_contract_test.sh:87`) | astra P0 | yes: `Host a b c` matches, `Host a b c d` does not | WARNING (fail-open; the spec says "a line starting with an SSH Host directive") | fix with a mutant (tasks.md 5.2); the prose control that started with "host" becomes a mid-line "host" (a test change) |
| A5 / O2 | The ripgrep step pins the noble revision `14.1.0-1` while the `lint` job runs on `ubuntu-latest`; when the image moves on, the required `lint` job fails on every PR | astra P2, opus WARNING | yes: the pin exists only in noble; the failure is loud, not silent | WARNING (availability) | fix: install the archive's ripgrep without a version pin, as the same job installs PyYAML (tasks.md 5.4) |
| O3 | PAC-4 home paths are only partly refused: `/var/home/<user>`, `/root/` and `C:\Users\<user>` pass (`agent_contract_test.sh:265`) | opus WARNING | yes for those three; `$HOME/x` is not machine-specific and stays allowed | WARNING (fail-open) | fix with mutants (tasks.md 5.3) |
| O4 | The running-host check skips itself when the name already occurs in tracked Markdown, so an earlier leak disables it | opus NOTE | yes; the spec allows this skip (PAC-6), and the independent review covers other machines | NOTE | reject: spec behaviour |
| O5 | The scrub self-test plants each pattern line as literal text; a future pattern with regex syntax would make its own mutant survive | opus NOTE | yes, and it fails closed (the self-test reds on that scrub-list edit) | NOTE | reject: fails closed and names the line |
| O6 | The self-test adds about 80 s to the required `lint` job and to `task check` | opus NOTE | yes (`evidence/G2.md`) | NOTE | recorded, no change |

## AGENTS.md truth (task 3.1)

Both legs and the orchestrator checked the committed `AGENTS.md` statement by statement on this
tree, as a stranger would. All were true. Checked by the orchestrator: every `task` name
(`task --list-all`); the 17 gates and their order (`hack/check.sh:42-63`); the guard sweep, the
Docker-header skip and the five exclusions (`hack/check.sh:88-124`); the 15 required contexts
(`hack/release/verify-eligibility.sh:19-22`) and what each job runs (`ci.yaml`, `preflight.yaml`,
`e2e-smoke.yaml`); every checklist path, glob and symbol (`registerTo`, `Catalog`,
`registeredMetricNames`, `NewRegistry`, `ToKollectSinkSpec`, `KollectProfileSpec`,
`KollectEventSinkSpec`); the named tests (`TestRegisterMatchesRegisteredMetricNames`,
`TestCRDSpecOpenAPIFragments`); the commands `make generate manifests`,
`bash hack/helm-sync-crds.sh`, `python3 hack/gen-glossary.py`; `hack/verify.sh:68` running the
schema test; and each gotcha against its cited file. The astra leg reported 60 statements checked
true, the claude leg every claim. No private content in `AGENTS.md` or
`.github/copilot-instructions.md`: no home or harness path, no internal host, no hostname, no
scrub-pattern match (`rg -i -f hack/scrub-patterns.txt` on both: no match).

`.gitignore`: in a scratch clone with an empty `.git/info/exclude` and
`core.excludesFile=/dev/null`, `AGENTS.local.md`, `CLAUDE.md` and `CLAUDE.local.md` are ignored
(`.gitignore:11-13`) and `git add -A --dry-run` with all three present adds none; the only
negation (`!hack/coverage.sh`) does not touch them.

## CRITICAL

The astra leg marked A1, A2 and A4 P0. After checking, none is a CRITICAL in the sense of the
brief: A1 publishes nothing that `main` does not already carry, A2 is covered on every known
checkout of the maintainer and gets a contributor warning, A4 is a narrow gap in a supplementary
pattern. Round 2 runs anyway, because a leg reported a P0.
