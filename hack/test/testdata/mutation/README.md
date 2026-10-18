# Mutation fixture and recorded reports

The Go module in this directory (`go.mod`, `control.go`, `control_test.go`) is the MUT-3 control
of the nightly mutation run (openspec change mutation-testing-signal). `TestSum` executes `Sum`
without asserting on its result, so gremlins reports the `ARITHMETIC_BASE` mutant at
`control.go:9:11` as `LIVED`. Nothing calls `Unused`, so its mutants are `NOT COVERED`.

`reports/` holds gremlins JSON reports recorded on 2026-10-19 with gremlins v0.6.0
(`GOBIN=<absolute tmp>/gremlins-v0.6.0 go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`)
under go1.27.2 linux/amd64, with `GOFLAGS=-count=1` and the runner's argv:

```text
gremlins unleash --config=<absolute empty {} file> --output=<dir>/<short-pkg>.json \
  --arithmetic-base --conditionals-boundary --conditionals-negation --invert-assignments \
  --invert-bitwise --invert-bwassign --increment-decrement --invert-logical --invert-loopctrl \
  --invert-negatives --remove-self-assignments <pkg>
```

| File | `<pkg>` | Working directory | Note |
| --- | --- | --- | --- |
| `aggregate.json` | `./internal/aggregate` | repository root | KILLED 4, LIVED 1 |
| `redact.json` | `./internal/redact` | repository root | KILLED 12, LIVED 1, NOT COVERED 1 |
| `pathvalidate.json` | `./internal/pathvalidate` | repository root | KILLED 10, LIVED 2 |
| `fixture.json` | `.` | this directory | control LIVED, 2 NOT COVERED |
| `dryrun-pathvalidate.json` | `./internal/pathvalidate` | repository root | same argv plus `--dry-run`: RUNNABLE 12 |
| `warm-cache-pathvalidate.json` | `./internal/pathvalidate` | repository root | without `GOFLAGS=-count=1`, test cache warm: TIMED OUT 12 |

The files are gremlins' output byte for byte (no trailing newline). The probe that produced them
is recorded in `openspec/changes/mutation-testing-signal/evidence/G1.md`.
