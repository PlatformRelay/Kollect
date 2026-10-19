## Verdict: CONCERNS

## Findings
- [WARNING] The mutation proof still can't be reproduced as written, even though round 1's WARNING is marked "fixed: script inlined" — `openspec/changes/test-depth-signals/evidence/G3.md:121`, `:158-184`, `:189`
  Failure: the text says "27 of 27 killed (20 before review, 7 on the fix lines after it)". The inlined `M` list has 25 entries, and the table has 22 rows that also cover 25 mutants. The script also calls `tempfile.mkdtemp(dir="/tmp/claude/g3")`. That directory is never created, so on a clean machine the first mutant fails with FileNotFoundError and nothing runs. The row is tagged "machine", but nobody can re-derive it.
  Fix: either change the count to 25, or inline the two missing mutants. Use `tempfile.mkdtemp()` with no `dir=`, or add `os.makedirs(..., exist_ok=True)`.
  Confidence: 85
- [WARNING] tasks.md marks 3.1 and 3.2 `[x] … closed 2026-10-18`, but the task's own records say review isn't finished — `openspec/changes/test-depth-signals/tasks.md:226-228`, `evidence/G3.md:7`, `:284` ("Round 2: pending"), `loop.md:46` ("pending review")
  Failure: someone reading tasks.md, or the loop resuming, sees G3 as closed before round 2 has run. Two matrix rows are still `independent-review | pending`. The checkboxes and the evidence disagree.
  Fix: leave the boxes unticked, or mark them "pending review", until round 2 has a disposition.
  Confidence: 70
- [NOTE] The guard only recognises a fence written as three backticks at column 0. An indented fence (inside a list item) or a `~~~sh` fence is skipped without any error — `hack/test/readme_quickstart_test.sh:105`
  Failure: say a later README edit numbers the steps and puts `task nosuch` inside a list-item fence, while a column-0 `sh` fence stays elsewhere in the section. The guard passes. R8 only catches the case where no column-0 fence is left at all. Today's README (lines 111-127) has only column-0 fences, so nothing is wrong yet.
  Fix: either fail `extract` on any line in the section that matches `^[[:space:]]+(```|~~~)`, or add this to the "Known limits" list in G3.md.
  Confidence: 60
- [NOTE] The Iterations list runs 1, 2, 3, 5, 4. The round-1 review (5) comes before the 3.2 wiring RED (4), although it happened after it — `evidence/G3.md:104-115`
  Fix: renumber or reorder the entries.
  Confidence: 90

Checked and OK:
- The parser reads the real README (`README.md:106-127`), and every command there resolves: `demo-up`, `dev-up` (in the comment), `kinv`, `ktgt`, `ksnap`, `config/samples/advanced/`.
- The `&&`, `||`, `;`, `|`, `&` and bracket split now runs in awk. Each segment checks only the first `task` or `kubectl` word, and the comment segment is checked like code, as TDS-3 asks (`spec.md:157-166`).
- Every other rule I read either fails closed or is spec-conformant: a flag before the kubectl verb, value flags, `-k=`, the `/name` suffix, comma lists, quoted or `aliases:`/`flatten:` task names, and include paths written as variables or missing files. TDS-3 only asks for `get` and `apply -k`, so verbs like `describe` are left unchecked.
- `crd_names` matches controller-gen's layout. The `categories:` list resets the shortNames state, so its entries are not counted as names.
- Self-test hygiene is sound. `replace_once` refuses an anchor that is missing or not unique, `reject` asserts the clause name and the token rather than only the exit code, and the EXIT trap cleans up.
- I traced the "missing Taskfile accepted" hand mutant: after the `;` the `die` runs unconditionally, so R7 kills it, which matches the table.
- CI wiring: two separate `lint` steps (`ci.yaml:603-606`), and `lint` has no `needs:`. The `cfc_guards` entry and its yq mutant delete both steps and assert the CFC-6 message (`ci_workflow_security_test.sh:609-611`, `:1142-1148`).
- No overlap with `demo_task_aliases_test.sh`: this guard checks names only, never delegation.
- No allow-list or exclusion was widened.

## Could not check
- I didn't run the guard, its self-test, the cws self-test or the hand-mutant script (read-only leg). The sensor results come from G3.md as recorded.
- The round-2 review legs (`reviews/L-tasks/G3/round-2/`, untracked) are outside the diff, and I didn't read them.
- I didn't read the hack/task Taskfiles, so the R6 targets `test:run` and `test:coverage:race` are taken on trust from the self-test.
- Whether protect-main actually lists `lint` as a required context is configured in GitHub, not in the repo. I relied on the CWS-6 definition.
