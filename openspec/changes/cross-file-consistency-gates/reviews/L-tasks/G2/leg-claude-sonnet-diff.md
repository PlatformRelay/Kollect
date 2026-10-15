## Verdict: CONCERNS

I read the full diff (`hack/test/consistency_tool_pins_test.sh`, `evidence/G2.md`) against CFC-2/CFC-6 in the spec, the Makefile `go-install-tool` and `golangci-lint` targets, and a repo-wide grep for pins. I did not run anything; the recorded sensor results in G2.md are the only run evidence.

## Findings
- [WARNING] The temp-dir cleanup never removes anything, so every run leaks its dirs — `hack/test/consistency_tool_pins_test.sh:97-102, 216, 298, 252, 399, 402`
  Failure: `new_tmp` appends to `TMP_DIRS`, but every caller runs it inside `$(...)`: `work="$(new_tmp)"`, `log="$(new_tmp)/smoke.log"`, `copy="$(new_copy)"`, `KOLLECT_TOOL_PINS_CACHE=$(new_tmp)`. The append happens in a subshell and is lost, so the EXIT trap's `TMP_DIRS` stays empty. A `--self-test` run leaves about 11 full tree copies plus the built tool caches (a custom golangci-lint binary and others) in `/tmp`. On a dev machine that's hundreds of MB per run. G2.md maintainability item 8 ("Writes only to temp dirs") hides this.
  Fix: have `new_tmp` take a variable name and set it in the caller's shell (`new_tmp VAR`). Or create one `WORKROOT="$(mktemp -d)"` at startup, register it once, and make `new_tmp` return `mktemp -d -p "${WORKROOT}"`.
  Confidence: 90

- [WARNING] `RUN_CACHE` never persists, so there is no per-run tool cache in plain mode — `:216`
  Failure: `RUN_CACHE="$(new_tmp)"` is assigned inside `tool_cache`, which is itself called via `$(...)`, so the assignment is lost. Every `build_tool` call gets a fresh directory. Today each tool is built once per run, so nothing visibly breaks. Any future second smoke of the same tool silently rebuilds (golangci takes about 10 s warm and 60+ s cold), and the comment at lines 40-42 describes caching that doesn't happen.
  Fix: same as above. Initialise `RUN_CACHE` once at top level.
  Confidence: 85

- [NOTE] The smoke run for golangci-lint is keyed to the Makefile's version, not the version it reports — `:224-236, 350-355`
  Failure: `build_tool` always builds via `make golangci-lint`. If another pin of golangci-lint at a different version appeared (for example a `go run …golangci-lint@vX` in `.github`), the `seen` key `golangci-lint@vX` would trigger a smoke run that actually exercises the Makefile's version. The new pin would pass untested while the output claims otherwise.
  Fix: after building, assert that the binary's `--version` output contains `${ver}`. Or skip the Makefile route when `src` isn't `Makefile`.
  Confidence: 55

- [NOTE] If the custom build fails, a dangling symlink is cached — `:232-234` with `Makefile:212-216`
  Failure: the `|| true` in the Makefile leaves `golangci-lint` as an absolute symlink into `${dir}.tmp/…-vX`. The `-x` check passes before the `mv`. After the `mv` the symlink dangles and the cache entry is bad. The guard still fails closed with a less direct message, and the next run rebuilds because `! -x` is true. The evidence file already notes the Makefile quirk.
  Fix: also verify `[[ ! -L … ]]`, or `readlink -f` the binary and require it to be a regular file.
  Confidence: 60

- [NOTE] Check mode on an arbitrary `<tree-root>` runs that tree's Makefile and builds — `:106-110, 232`
  Failure: `make -C <root>` evaluates `$(shell …)` and runs the tree's recipes. `hack/test/consistency_tool_pins_test.sh <untrusted-tree>` therefore executes code from that tree. This is acceptable for the self-test's own copies, but the usage header advertises it for any tree.
  Fix: document it as trusted-trees-only.
  Confidence: 50

- [NOTE] A failed self-test leaves the fixture module in the real module cache — `:456-464`
  Failure: the `kollect.invalid` cache cleanup runs only after the newer-Go mutant passes. If `fail` fires in between, `$(go env GOMODCACHE)/kollect.invalid` stays behind, with write bits set thanks to `-modcacherw`.
  Fix: do the removal in `cleanup` (EXIT trap), or use a private `GOMODCACHE` for that mutant.
  Confidence: 70

- [NOTE] Spec deviation: `GOTOOLCHAIN=go<ver>` instead of `GOTOOLCHAIN=local` — `:297`
  Failure: CFC-2 says `local`. The guard pins an exact toolchain, which is stricter on hosts with a different local Go. This is documented and defensible. A host with no network and no cached toolchain fails with "cannot run go X", which is closed but noisy.
  Fix: none needed. Make sure the spec text is amended to match.
  Confidence: 80

## Could not check
- I ran no tests. Every sensor result (rows 2–12, the 27 s / 88 s / 178 s timings, mutant kills) comes from G2.md. I did not verify the stated `ENVTEST_VERSION=release-0.24` or the real-tree pin list.
- `/tmp/g2-hand-mutants/run.sh` (the hand mutants h1–h6) is outside the repo.
- `tasks.md` is not in this diff, so I could not check the "tick" the evidence file lists as in scope.
- CI wiring and `task lint:shell` glob coverage belong to task 6.1 and are out of scope.
- I did not read `.go-arch-lint.yml` or `hack/tooling/.custom-gcl.yml`, so the smoke targets' validity under a clean checkout is unverified.
