## Unified verdict: CONCERNS  (legs ok: 2/2)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | WARNING | KEX order guard enforces only an in-order subsequence of the eight existing algorithms, not the spec's stronger "keeps the existing algorithms first" — a T09 that prepends stays green | internal/sink/git/kex_convergence_test.go:80-89 vs specs/git-engine/spec.md:43 | 1 (DeepSeek) | DeepSeek | 85 |
| 2 | MINOR | Probe test's skip guard re-implements skipWithoutGit and resolves git on ambient PATH instead of the pinned dirs — drift risk plus spurious failure when git lives only outside /usr/bin:/bin:/usr/local/bin | internal/sink/git/file_remote_convergence_test.go:64-67 | 2 (DeepSeek, Qwen) | both | 100 |

## Disagreements
- DeepSeek CONCERNS vs Qwen CLEAN hinges on spec.md:43: DeepSeek reads "keeps the existing algorithms first" as a leading-positions requirement; Qwen read the same guard as relative-order-only and judged it compliant, so never flagged it.
- Engine-less fixtures: Qwen verified no test hardcodes an engine (string literals only); DeepSeek counters that `.withDefaults()` still assigns GitEngineGoGit, so "engine-less" describes literals, not effective config — dropped as a one-leg NOTE.

## Nobody could check
- Full 47-package suite, `-race`, `task lint`/`task verify`, integration/helm gates: never executed; Qwen ran no tests at all (read-only reasoning), DeepSeek ran only the five targeted test commands.
- T09 interaction: guard behaviour under the future API change is reasoned, not executed.
- charts/kollect/crds enum block verified by grep-count only; CI behaviour of the branch carrying the four sanctioned reds rests on loop.md's word.
