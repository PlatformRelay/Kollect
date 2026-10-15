verdict: BLOCK
findings:
  - [CRITICAL] hack/test/consistency_go_toolchain_test.sh:88 — `ARG BUILDER=golang:1.99.0` followed by `FROM ${BUILDER}` escapes scanning; other matching images keep the gate green. Confirmed with an in-memory probe.
  - [CRITICAL] hack/test/consistency_metrics_test.sh:139 — Every matching string literal counts as registration. Removing `ReconcileInFlight` from `MustRegister` leaves the gate green while the documented, queried metric disappears; confirmed in memory.
  - [CRITICAL] hack/test/consistency_tool_pins_test.sh:388 — Only the three recognized analysis tools are built. Other pins pass on module metadata and package-directory existence, even when compilation fails through incompatible code or transitive dependencies; CFC-2 is not enforced.
  - [WARNING] hack/test/consistency_go_toolchain_test.sh:328 — `new_copy` runs in command substitution, losing its `SCRATCH` updates; the EXIT trap therefore leaves every self-test copy behind.
could_not_check:
  - Full self-tests and network-dependent builds were not rerun; filesystem mutation was prohibited. Probes used in-memory substitutions.
  - Live branch protection, Renovate behavior, and the external release listing were not verified.
summary: CI wiring and RBAC exception restrictions appear sound, but three guards can pass without establishing their required invariants.
