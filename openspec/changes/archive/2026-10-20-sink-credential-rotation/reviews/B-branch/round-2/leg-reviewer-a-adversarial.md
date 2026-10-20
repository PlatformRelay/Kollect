## Verdict: CONCERNS

## Round-1 findings

- H1: closed — indirect dependency imports removed; recorded tidy check passed.
- A1: closed — full-read and CA-read failures retire and fence the correct key; holders drain safely.
- A2: not closed — early-return reconciles still allow an unchanged `0s` binding to expire.
- B1: closed — accepted residual documented and safety tested.
- B3: closed — wording now locates the fence at the pool miss.
- B4: closed — cluster terminal no-requeue test added and passed.

## Findings

- [WARNING] A2 still permits duplicate exports after prolonged sink unavailability — `internal/controller/per_sink_export.go:131`
  Counterargument: regular watchdog skips refresh activity, preventing expiry during healthy operation.
  Failure: a successfully exported `0s` inventory's sink has `ConnectionVerified=False` for over 48 hours → reconciles return at `kollectinventory_controller.go:158` without refreshing `lastSkipped`; another inventory's export prunes its state → connection recovery causes the unchanged inventory to export again, violating SCR-7.
  Fix: refresh existing bindings' activity when a live inventory reconciles, including early-return paths; add a regression covering prolonged connection failure and recovery.
  Confidence: 98

- [NOTE] The B1 test permits reuse instead of asserting the documented retirement — `internal/sink/backend_pool_reread_residual_test.go:55`
  Failure: removing the failed-re-read retirement lets `next == b6` pass, so this test alone does not pin the accepted extra-build behavior.
  Fix: assert the entry was retired and `next != b6`, then assert the held backend closes on release.
  Confidence: 100

## Could not check

- Both requested targeted test commands passed; full gates, race tests, envtest, and mutants were not rerun.
- Reviewed the fix range and relevant surrounding paths, not the entire original branch.
