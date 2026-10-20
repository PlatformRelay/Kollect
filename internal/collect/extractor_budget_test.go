// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
//   - Latency is NOT a >25% gate either, and it is not a raw ns/op ceiling.
//     Wall-clock is not portable across CI hardware or load: the same Extract
//     costs ~37k ns/op on an idle amd64 laptop, ~380k under -race and >3M
//     under -race on an oversubscribed host, so any absolute ceiling loose
//     enough for every runner catches nothing, and one tight enough to catch
//     something false-reds under load. The default latency check instead
//     measures Extract against a calibration workload (deep-copy and JSON
//     encoding of the same pool objects) in interleaved short slices, in the
//     same process, and bounds the RATIO of the two best per-op times. Load,
//     -race and -cover slow both sides alike, so the ratio stays put (~7-8 idle,
//     ~10 under -race on amd64) while the absolute numbers move by 100x. The
//     ceiling is the recorded ratio x calibratedRatioSlack: a coarse
//     catastrophic-regression net (an injected CPU-only 6.8x slowdown fails it,
//     a 5x one passes), not a
//     25% gate. For a real latency floor on calibrated hardware, measure your
//     own ns/op and set KOLECT_EXTRACT_MAX_NS_PER_OP to that x 1.25; the
//     absolute check runs only when that variable is set.
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
// The calibrated ratio (baselineCalibratedRatio) is the logged "ratio" of an
// idle run; re-record it from the test's own log line when the workload or the
// calibration changes.
//
// The B/op and allocs/op ceilings are env-overridable via
// KOLECT_EXTRACT_MAX_{BYTES,ALLOCS}_PER_OP; KOLECT_EXTRACT_MAX_NS_PER_OP adds an
// absolute latency ceiling for a calibrated machine.
// ---------------------------------------------------------------------------

const (
	// Recorded S-LOCAL baseline (darwin/arm64 Apple M5 Max, go1.26.5), varied pool.
	// baselineNsPerOp is reference only: no default check compares against it.
	baselineNsPerOp     = 25551
	baselineBytesPerOp  = 32218
	baselineAllocsPerOp = 452

	// Regression headroom: a genuine >25% gate for B/op and allocs/op.
	budgetHeadroomNumerator   = 125
	budgetHeadroomDenominator = 100

	// baselineCalibratedRatio is Extract's best ns/op divided by the
	// calibration workload's best ns/op, measured interleaved (linux/amd64,
	// go1.26, idle: 7.2-7.6 over repeated runs; ~9.9 under -race). Stored x10
	// to stay an integer.
	baselineCalibratedRatioX10 = 75

	// calibratedRatioSlack multiplies the recorded ratio into the ceiling. It
	// absorbs the -race shift (~1.25x), differences between CPU
	// micro-architectures in how Extract and the calibration scale, and
	// measurement noise; load itself mostly cancels out of the ratio. It is a coarse
	// net, not a 25% gate.
	calibratedRatioSlack = 5

	// calibrationRounds interleaved slices of each workload; the best slice
	// per workload wins, so a burst of contention during one slice does not
	// count. calibrationSlice is the target duration of one slice.
	calibrationRounds = 40
	calibrationSlice  = 2 * time.Millisecond
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

// calibrationWorkload is the reference the default latency ceiling is scaled
// to: deep-copy one pool object and JSON-encode the copy. It is generic,
// allocation-heavy map work that slows down with the runner (CPU contention,
// -race, -cover) the way Extract does, and it calls nothing in this package, so
// an extractor regression cannot hide in it.
func calibrationWorkload(obj *unstructured.Unstructured) error {
	_, err := json.Marshal(runtime.DeepCopyJSON(obj.Object))

	return err
}

// interleavedBestNsPerOp runs extract and calibrate in alternating slices of
// roughly calibrationSlice each, flipping which goes first every round, and
// returns the lowest ns/op either workload reached in any slice. Alternating
// short slices means both see the same machine state; taking the best slice
// discards the slices a contention burst hit.
func interleavedBestNsPerOp(t *testing.T, extract, calibrate func(i int) error) (extractNs, calibNs float64) {
	t.Helper()

	slice := func(fn func(int) error, iters int) float64 {
		start := time.Now()
		for i := range iters {
			if err := fn(i); err != nil {
				t.Fatalf("workload: %v", err)
			}
		}

		return float64(time.Since(start).Nanoseconds()) / float64(iters)
	}
	itersFor := func(fn func(int) error) int {
		perOp := slice(fn, 64)
		if perOp <= 0 {
			return 64
		}

		return max(1, int(float64(calibrationSlice.Nanoseconds())/perOp))
	}

	extractIters, calibIters := itersFor(extract), itersFor(calibrate)
	extractNs, calibNs = math.Inf(1), math.Inf(1)
	for round := range calibrationRounds {
		if round%2 == 0 {
			extractNs = math.Min(extractNs, slice(extract, extractIters))
			calibNs = math.Min(calibNs, slice(calibrate, calibIters))
		} else {
			calibNs = math.Min(calibNs, slice(calibrate, calibIters))
			extractNs = math.Min(extractNs, slice(extract, extractIters))
		}
	}

	return extractNs, calibNs
}

// TestExtractHotPathBudget asserts the extractor hot path stays inside its
// recorded allocation budget and its calibrated latency ratio. Unlike a bare
// benchmark, this FAILS.
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

	maxBytesPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_BYTES_PER_OP",
		withHeadroom(baselineBytesPerOp))
	maxAllocsPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_ALLOCS_PER_OP",
		withHeadroom(baselineAllocsPerOp))
	// 0 = no absolute latency ceiling unless a calibrated machine pins one.
	maxNsPerOp := envBudget(t, "KOLECT_EXTRACT_MAX_NS_PER_OP", 0)

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}
	pool := deploymentPool(extractPoolSize)
	attrs := extractAttrs()
	extractNs, calibNs := interleavedBestNsPerOp(t,
		func(i int) error {
			_, extractErr := extractor.Extract(pool[i%len(pool)], attrs)

			return extractErr
		},
		func(i int) error { return calibrationWorkload(pool[i%len(pool)]) },
	)
	if calibNs <= 0 || math.IsInf(calibNs, 0) || math.IsInf(extractNs, 0) {
		t.Fatalf("calibration produced no timing (extract=%v ns/op, calibration=%v ns/op); "+
			"the latency ratio would pass vacuously", extractNs, calibNs)
	}
	ratio := extractNs / calibNs
	maxRatio := float64(baselineCalibratedRatioX10) / 10 * calibratedRatioSlack

	t.Logf("extract hot path: N=%d %d ns/op (%.0f ops/s) %d B/op %d allocs/op; "+
		"interleaved best: extract %.0f ns/op, calibration %.0f ns/op, ratio %.2f "+
		"[ceilings: ratio %.1f, %d B/op, %d allocs/op, absolute ns/op %d (0 = off)]",
		res.N, nsPerOp, 1e9/float64(nsPerOp), bytesPerOp, allocsPerOp,
		extractNs, calibNs, ratio, maxRatio, maxBytesPerOp, maxAllocsPerOp, maxNsPerOp)

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

	// Calibrated latency ceiling — load-independent, a catastrophic-regression
	// net, NOT a >25% gate.
	if ratio > maxRatio {
		t.Errorf("calibrated latency ceiling exceeded: Extract costs %.1fx the calibration "+
			"workload, ceiling %.1fx (recorded %.1fx x slack %d). Load mostly cancels out of this "+
			"ratio, so this means Extract itself got several times slower",
			ratio, maxRatio, float64(baselineCalibratedRatioX10)/10, calibratedRatioSlack)
	}

	// Absolute latency ceiling — only when a calibrated machine pins one.
	if maxNsPerOp > 0 && nsPerOp > maxNsPerOp {
		t.Errorf("latency ceiling exceeded: %d ns/op > %d ns/op (KOLECT_EXTRACT_MAX_NS_PER_OP; "+
			"S-LOCAL baseline is %d ns/op)", nsPerOp, maxNsPerOp, baselineNsPerOp)
	}
}
