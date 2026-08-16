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
// CALIBRATION (why the two dimensions have different strictness):
//
//   - Bytes/op and allocs/op are hardware-INDEPENDENT for a fixed Go toolchain
//     and word size — an M-series laptop and ubuntu-latest agree. These carry
//     the regression-detection weight and are set at the recorded baseline
//     +25%. The 25% headroom is also what absorbs a Go patch-release bump.
//
//   - Ns/op is hardware-DEPENDENT. A ceiling calibrated on the M-series
//     baseline would false-red on slower CI runners, and the default unit gate
//     runs packages in parallel, which adds scheduling noise. So the default
//     ns/op ceiling is a deliberately coarse net for catastrophic regressions
//     only. Tighten it per-machine with KOLECT_EXTRACT_MAX_NS_PER_OP once you
//     have a baseline for that hardware (recommended: measured × 1.25).
//
// RECORDED BASELINE — regenerate with `task bench` after changing the workload
// (worst of `-count=5`), then update the const block below in the same commit:
//
//	S-LOCAL, darwin/arm64 Apple M5 Max, go1.26.5, extractPoolSize=128:
//	  25551 ns/op · 32218 B/op · 452 allocs/op  (~39 000 ops/s)
//
// For reference, the pre-variation single-object baseline recorded in
// agent-context/PERFTEST-RESULTS-2026-08-16.md was 27679 ns/op · 32219 B/op ·
// 452 allocs/op — varying the object pool did not move the allocation profile.
//
// All three ceilings are env-overridable so a calibrated runner can assert the
// real >25% regression floor without the unit gate false-redding.
// ---------------------------------------------------------------------------

const (
	// Recorded S-LOCAL baseline (darwin/arm64 Apple M5 Max, go1.26.5), varied pool.
	baselineNsPerOp     = 25551
	baselineBytesPerOp  = 32218
	baselineAllocsPerOp = 452

	// Regression headroom: the acceptance criterion is "fails if throughput
	// regresses by more than 25%".
	budgetHeadroomNumerator   = 125
	budgetHeadroomDenominator = 100

	// Hardware slack applied to the ns/op ceiling ONLY (see CALIBRATION above).
	// This default has to survive the WORST case: the required PR gate runs
	// `go test $(go list ./...) -coverprofile`, so this test executes with
	// coverage instrumentation (measured +9% locally) while ~40 sibling
	// packages — several of them spinning envtest kube-apiserver/etcd — compete
	// for a 4-core runner. Two concurrent commands alone cost 2.8x on an
	// 18-core laptop. 50x is deliberately absurd: it catches only an
	// order-of-magnitude regression, and a ceiling that false-reds on every PR
	// is worse than one that never fires.
	//
	// `task extract-budget` runs this test ALONE and pins a much tighter
	// KOLECT_EXTRACT_MAX_NS_PER_OP, which is where the real latency gate lives.
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

	// Memory ceiling — hardware-independent, this is the real regression gate.
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

	// Throughput floor, expressed as a latency ceiling. Coarse by default; set
	// KOLECT_EXTRACT_MAX_NS_PER_OP on calibrated hardware for the real +25% gate.
	if nsPerOp > maxNsPerOp {
		t.Errorf("throughput floor breached: %d ns/op > %d ns/op ceiling "+
			"(%.0f ops/s < %.0f ops/s); S-LOCAL baseline is %d ns/op",
			nsPerOp, maxNsPerOp, 1e9/float64(nsPerOp), 1e9/float64(maxNsPerOp),
			baselineNsPerOp)
	}
}
