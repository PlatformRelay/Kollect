// Package shuffle is an order-dependent fixture for hack/test/test_shuffle_test.sh (TDS-2,
// openspec change test-depth-signals). TestDependent passes only after TestSetsState has run in
// the same test binary, so source order (-shuffle=off) is green and a seed that runs
// TestDependent first is red. It lives under testdata/, so `go list ./...` never selects it.
//
// Source: written for task 2.2. Checked with go1.27.2 linux/amd64 (2026-10-18):
// `go test -shuffle=off -count=3 ./hack/test/testdata/shuffle` is ok, and `-shuffle=2` fails on
// TestDependent at -count=3 and at -count=1.
package shuffle

import "testing"

var stateSet bool

func TestSetsState(t *testing.T) {
	stateSet = true
}

func TestDependent(t *testing.T) {
	if !stateSet {
		t.Fatal("TestDependent ran before TestSetsState: order-dependent fixture")
	}
}
