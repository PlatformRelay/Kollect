// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"os"
	"strconv"
	"testing"
)

// ---------------------------------------------------------------------------
// Extractor hot-path budget.
//
// WHAT THIS EXERCISES: collect.Extractor.Extract over a bounded pool of varied
// Deployment-shaped objects with two JSONPath attributes and one CEL
// attribute, single-threaded, in-process.
//
// WHAT THIS DOES *NOT* EXERCISE: no API server, no informers, no cluster, no
// sinks, no controller, no concurrency, no export path. It is a micro-benchmark
// budget, NOT a load test and NOT cluster-scale evidence. In-cluster scale
// lives in the opt-in envtest scale test (TestEngine_ScaleEnvtestOptIn) and in
// docs/operator-manual/load-test-runbook.md.
//
// WHAT IS ACTUALLY ENFORCED — read this before quoting a number from it:
//
//   - B/op and allocs/op ARE a >25% regression gate. They are
//     hardware-INDEPENDENT for a fixed Go toolchain and word size: measured
//     identical on darwin/arm64 and amd64 (32217 vs 32212 B/op), unchanged
//     under -cover, and only marginally higher under -race (32980 B/op / 459
//     allocs). They also held unchanged across the go1.26.5 -> go1.26.6 bump.
//     A regression in either fails anywhere this test runs.
//
//   - ns/op is NOT a >25% gate. It is a deliberately coarse
//     catastrophic-regression net and nothing more. Wall-clock is not portable
//     across CI hardware, so any ceiling loose enough to be safe on a shared
//     runner is far too loose to catch 25%. Concretely: an injected CPU-only
//     regression that made Extract 3.5x slower (39144 -> 11078 ops/s) at
//     UNCHANGED B/op and allocs/op passed both the task-pinned ceiling and the
//     in-code default. If you need the real latency gate, measure your own
//     hardware and set KOLECT_EXTRACT_MAX_NS_PER_OP to that × 1.25.
//
// This asymmetry is deliberate and is on the record: PERF-FIX-04's acceptance
// criterion ("throughput regresses >25% => FAILS") is met for the ALLOCATION
// dimensions only. It was not silently dropped — a flaky wall-clock gate on
// shared CI runners was judged worse than no wall-clock gate.
//
// RECORDED BASELINE — regenerate with `task bench` after changing the workload
// (worst of `-count=5`), then update the const block below in the same commit:
//
//	S-LOCAL, darwin/arm64 Apple M5 Max, go1.26.6, extractPoolSize=128:
//	  25551 ns/op · 32218 B/op · 452 allocs/op  (~39 000 ops/s)
//
// For reference, the pre-variation single-object baseline recorded in
// agent-context/PERFTEST-RESULTS-2026-08-16.md was 27679 ns/op · 32219 B/op ·
// 452 allocs/op — varying the object pool did not move the allocation profile.
//
// All three ceilings are env-overridable via
// KOLECT_EXTRACT_MAX_{NS,BYTES,ALLOCS}_PER_OP, so a calibrated machine can pin
// a real latency floor without the shared unit gate false-redding.
// ---------------------------------------------------------------------------

const (
	// Recorded S-LOCAL baseline (darwin/arm64 Apple M5 Max, go1.26.5), varied pool.
	baselineNsPerOp     = 25551
	baselineBytesPerOp  = 32218
	baselineAllocsPerOp = 452

	// Regression headroom. This is a genuine >25% gate for B/op and allocs/op.
	// It is ALSO applied to ns/op, but there it is only the starting point for
	// nsPerOpHardwareSlack below — the shipped ns/op ceiling is not a 25% gate.
	budgetHeadroomNumerator   = 125
	budgetHeadroomDenominator = 100

	// Hardware slack applied to the ns/op ceiling ONLY (see CALIBRATION above).
	// This default has to survive the WORST case: the required PR gate runs
	// `go test $(go list ./...) -coverprofile` while ~40 sibling packages —
	// several of them spinning envtest kube-apiserver/etcd — compete for a
	// 4-core runner. Coverage instrumentation itself is in the noise here
	// (measured at or slightly below the uninstrumented number), so contention,
	// not -cover, is what this absorbs: two concurrent commands alone cost 2.8x
	// on an 18-core laptop. 50x is deliberately absurd. It catches only an
	// order-of-magnitude regression, and a ceiling that false-reds on every PR
	// is worse than one that never fires.
	//
	// `task extract-budget` runs this test ALONE and pins a tighter
	// KOLECT_EXTRACT_MAX_NS_PER_OP — still a coarse net, not a 25% gate.
	nsPerOpHardwareSlack = 50
)

// envBudget reads an int64 budget override, failing loudly on garbage rather
// than silently falling back to the default.
func envBudget(t *testing.T, key string, def int64) int64 {
	t.Helper()

	raw := os.Getenv(key)
	if raw == "" {
		return def
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		t.Fatalf("%s must be a positive integer, got %q", key, raw)
	}

	return v
}

func withHeadroom(base int64) int64 {
	return base * budgetHeadroomNumerator / budgetHeadroomDenominator
}

// TestExtractHotPathBudget asserts the extractor hot path stays inside its
// recorded allocation and latency budget. Unlike a bare benchmark, this FAILS.
func TestExtractHotPathBudget(t *testing.T) {
	res := testing.Benchmark(extractWorkload)

	// Guard against a vacuous pass: a failed or zero-iteration benchmark
	// reports zeroes, which would satisfy every ceiling below.
	if res.N <= 0 {
		t.Fatalf("benchmark did not run (N=%d) — extractWorkload failed", res.N)
	}
	if res.MemAllocs == 0 || res.MemBytes == 0 {
		t.Fatalf("benchmark reported no allocation data (allocs=%d bytes=%d); "+
			"the memory ceiling would pass vacuously", res.MemAllocs, res.MemBytes)
	}

	nsPerOp := res.NsPerOp()
	bytesPerOp := res.AllocedBytesPerOp()
	allocsPerOp := res.AllocsPerOp()

	maxNsPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_NS_PER_OP",
		withHeadroom(baselineNsPerOp)*nsPerOpHardwareSlack)
	maxBytesPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_BYTES_PER_OP",
		withHeadroom(baselineBytesPerOp))
	maxAllocsPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_ALLOCS_PER_OP",
		withHeadroom(baselineAllocsPerOp))

	t.Logf("extract hot path: N=%d %d ns/op (%.0f ops/s) %d B/op %d allocs/op "+
		"[ceilings: %d ns/op, %d B/op, %d allocs/op]",
		res.N, nsPerOp, 1e9/float64(nsPerOp), bytesPerOp, allocsPerOp,
		maxNsPerOp, maxBytesPerOp, maxAllocsPerOp)

	// Memory ceiling — hardware-independent, so this IS the >25% regression gate.
	if bytesPerOp > maxBytesPerOp {
		t.Errorf("memory ceiling exceeded: %d B/op > %d B/op (baseline %d B/op +25%%); "+
			"re-profile the extractor or re-record the baseline with evidence",
			bytesPerOp, maxBytesPerOp, baselineBytesPerOp)
	}
	if allocsPerOp > maxAllocsPerOp {
		t.Errorf("allocation ceiling exceeded: %d allocs/op > %d allocs/op "+
			"(baseline %d allocs/op +25%%)",
			allocsPerOp, maxAllocsPerOp, baselineAllocsPerOp)
	}

	// Latency ceiling — a catastrophic-regression net, NOT a >25% gate. Set
	// KOLECT_EXTRACT_MAX_NS_PER_OP on calibrated hardware for a real floor.
	if nsPerOp > maxNsPerOp {
		t.Errorf("latency ceiling exceeded: %d ns/op > %d ns/op "+
			"(%.0f ops/s < %.0f ops/s); S-LOCAL baseline is %d ns/op. This is a "+
			"catastrophic-regression net, so breaching it means something is very "+
			"wrong (or the runner is heavily loaded) — not merely a 25%% regression",
			nsPerOp, maxNsPerOp, 1e9/float64(nsPerOp), 1e9/float64(maxNsPerOp),
			baselineNsPerOp)
	}
}
