# Post-merge evidence — mutation-testing-signal

Measured against `main` @ `cf448c4a6` (2026-10-25). All runs are scheduled runs of
`.github/workflows/mutation.yaml` on `main`.

| Row | Check | Result |
|---|---|---|
| MUT-1 | First scheduled run and its reporter | Run 39379440869 (2026-10-20T06:16Z) `success`; artifact `mutation-reports` (1952 bytes) holds `aggregate`, `pathvalidate`, `redact` and `fixture` reports, each with `mutants_total`, `_killed`, `_lived`, `_not_viable`, `_not_covered`; reporter run 39394745241 `success` |
| MUT-3 | Fixture control on the first scheduled run | `mutation: fixture control control.go:9:11 ARITHMETIC_BASE is LIVED, as it must be` |
| MUT-4 | Forced failure, then recovery | `MUTATION_FORCE_FAILURE=true` set 2026-10-20T06:19Z; run 39554408074 (2026-10-21T06:16Z) `failure`; reporter 39554536556 `success` opened exactly one issue, #488 `CI failure: Mutation testing`; variable deleted 2026-10-21T06:27Z; run 39733252056 (2026-10-22T06:19Z) `success`, reporter 39733515765 closed #488 (`event=recovery`) |

Later scheduled runs 39910190768, 40069039553 and 40169228768 (2026-10-23 to 2026-10-25) are `success`.

Commands:

```sh
gh api repos/PlatformRelay/Kollect/actions/runs/39379440869/artifacts --jq '.artifacts[].name'
gh run view 39379440869 --repo PlatformRelay/Kollect --log | grep 'fixture control'
gh run list --repo PlatformRelay/Kollect --workflow mutation.yaml --limit 6
gh issue view 488 --repo PlatformRelay/Kollect --json state,closedAt,comments
```
