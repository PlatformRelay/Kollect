# fuzz-retry fixtures

Replayed by `hack/test/fuzz_retry_test.sh` through a fake `go` on `PATH`. Each file is the output
of one `go test -run='^$' -fuzz=<target> -fuzztime=30s <pkg>` attempt.

| File | Source |
| --- | --- |
| `deadline-FuzzValidateAttributePath.txt` | run 37911142180, job `fuzz (FuzzValidateAttributePath, ./internal/collect/...)`, attempt 1 |
| `pass-FuzzValidateAttributePath.txt` | same job, attempt 2 (the passing retry) |
| `deadline-FuzzContentHash.txt` | run 37288739312, job `fuzz (FuzzContentHash, ./internal/aggregate/...)`, attempt 1 |
| `pass-FuzzContentHash.txt` | same job, attempt 2 (the passing retry) |
| `zero-target.txt` | local: `go test -run='^$' -fuzz=FuzzNoSuch -fuzztime=1s ./internal/aggregate`, go1.27.2 linux/amd64, exit 0 |
| `crash-local.txt` | local: a throwaway module outside the repository whose `FuzzCrash` panics on inputs longer than 3 bytes, `go test -run='^$' -fuzz=FuzzCrash -fuzztime=30s .`, go1.27.2 linux/amd64, exit 1 |

The CI files come from `gh run view --log --job <id>` (2026-10-18): the fuzz step's lines, with the
job and step columns and the timestamps cut, the echoed script and the runner's own `::warning::`
line dropped, and the attempts split at that warning. `crash-local.txt` is locally produced, with
trailing whitespace stripped; nothing else is edited.
