## Unified verdict: CLEAN   (legs ok: 2/2)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | LOW | "Identical class" comment overstates `ClassOf`/`ClassifyAPI` equivalence — true only for unclassified acquire errors (old code let the API-status class override an explicit class); `ResolveSecret` also rewrites API NotFound to a plain error | internal/sink/export.go:174-179, cleanup.go:188-192 | DeepSeek-V4.1-Flash, Qwen3.8-Flash-Next | 2 | 100 |
| 2 | LOW | T11.md verification matrix rows 5, 9-19 read "pending" while the Sensors table records the same commands green; row 18 (`gitleaks`, "Secrets clean at commit") has no recorded run at all | evidence/T11.md:72-118 | DeepSeek-V4.1-Flash, Qwen3.8-Flash-Next | 2 | 100 |

Dropped per rule: 4 single-leg NOTEs — K-14 insecure-sinks refusal wrapped Terminal (DeepSeek), cleanup any-terminal-wins asymmetry (DeepSeek), T11.md:159 "terminal for free" scope overstatement (Qwen), `timeNow()` read outside mutex judged immaterial (Qwen).

## Disagreements
- None; both legs returned CLEAN and the two code-chain readings (e.g. apimachinery `errors.As` semantics, no third demotion site) matched.
- Finding #1 was angled differently — DeepSeek: `ResolveSecret` NotFound rewrite makes missing-secret transient under both old and new code; Qwen: explicit class now wins over API class for already-classified errors — complementary framings, same fix (qualify the comment).

## Nobody could check
- `task test-integration` — no Docker in either leg; CI-owned.
- Neither leg re-ran the full gate suite (`go test/build/lint`, spec:validate, `-race -count=2` full packages); Qwen reran partial `-race` on the new/neighbour tests and ERA-2 checks only.
- `gitleaks` was never executed (nothing staged); both read the diff manually and saw no secret values.
- `reviews/B-branch/round-1/register.md` and the other legs' transcripts were out of bounds for both, so the five register findings were checked against live code, not the register's own wording.
