// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// The breaker pair drives the live RunExportEnvelope path — production calls
// exportThroughBreaker inside RunExportEnvelope (export.go). The same trip/reset
// semantics were previously asserted through the deleted items-level runner.
func TestRunExportEnvelope_circuitBreakerTripsAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	const (
		sinkNamespace = "cb-test-ns"
		sinkName      = "cb-test-sink"
	)

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sinkNamespace},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             "stub",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	req := ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Client:        cl,
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    sinkNamespace + "/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "stub",
			Endpoint: "https://example.com/repo.git",
		},
	}

	for range circuitBreakerTripAt {
		_, err = RunExportEnvelope(req)
		if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
			t.Fatalf("RunExportEnvelope() before trip = %v (%v), want transient", err, kollecterrors.ClassOf(err))
		}
	}

	_, err = RunExportEnvelope(req)
	if err == nil {
		t.Fatal("expected circuit breaker open error")
	}
	if kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
		t.Fatalf("open breaker error class = %v, want transient", kollecterrors.ClassOf(err))
	}
}

// Not t.Parallel: ResetBreakersForTest clears every key, including the breakers of parallel tests
// mid-trip.
func TestResetBreakersForTest_clearsOpenBreaker(t *testing.T) {

	const (
		sinkNamespace = "cb-reset-ns"
		sinkName      = "cb-reset-sink"
	)

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sinkNamespace},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             "stub",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	req := ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Client:        cl,
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    sinkNamespace + "/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "stub",
			Endpoint: "https://example.com/repo.git",
		},
	}

	for range circuitBreakerTripAt {
		_, _ = RunExportEnvelope(req)
	}
	if _, err := RunExportEnvelope(req); err == nil {
		t.Fatal("expected open breaker before reset")
	}

	ResetBreakersForTest()
	stub.exportErr = nil
	if _, err := RunExportEnvelope(req); err != nil {
		t.Fatalf("RunExportEnvelope after ResetBreakersForTest: %v", err)
	}
}

// SINK-BREAKER-RACE-01: parallel tests reset the shared registry while other
// tests export through it. Run under -race, this fails if the reset replaces
// the registry instead of clearing it through its own synchronisation. Not
// t.Parallel: it races its own goroutines, and its many resets must not wipe
// the breakers of sibling parallel tests mid-trip.
func TestResetBreakersForTest_safeAgainstConcurrentExport(t *testing.T) {
	const workers = 8
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			key := fmt.Sprintf("cb-race-ns/sink-%d", i)
			for range 200 {
				if i%2 == 0 {
					ResetBreakersForTest()
					continue
				}
				if err := exportThroughBreaker(key, func() error { return nil }); err != nil {
					t.Errorf("exportThroughBreaker(%q) = %v, want nil", key, err)
					return
				}
			}
		})
	}
	wg.Wait()
}

// breakerKeySeq makes breaker keys unique per test invocation: -count=N reruns a test against the
// process-wide registry its earlier run left behind.
var breakerKeySeq atomic.Uint64

func uniqueBreakerName(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf("%s-%d", strings.NewReplacer("/", "-", "_", "-").Replace(strings.ToLower(t.Name())), breakerKeySeq.Add(1))
}

func uniqueBreakerKey(t *testing.T) string {
	t.Helper()

	return "cb-unit/" + uniqueBreakerName(t)
}

var errInventoryConflict = errors.New("prune path \"a.yaml\" belongs to another inventory")

// callBreaker runs one export through the breaker and reports whether fn ran.
func callBreaker(key string, result error) (bool, error) {
	ran := false
	err := exportThroughBreaker(key, func() error {
		ran = true

		return result
	})

	return ran, err
}

func requireOpenBreakerError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !kollecterrors.IsTransient(err) || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Fatalf("err = %v (class %s), want a transient error containing %q", err, kollecterrors.ClassOf(err), "circuit breaker open")
	}
}

// pathStubBackend fails or succeeds per object path, and counts the calls that reached it. The
// backend sees the layout's object path, so both match on a substring (the inventory name).
type pathStubBackend struct {
	mu    sync.Mutex
	fail  map[string]error
	calls map[string]int
}

func (s *pathStubBackend) Type() string { return "stub" }

func (s *pathStubBackend) Capabilities() cap.Capabilities { return cap.SnapshotStore() }

func (s *pathStubBackend) Export(_ context.Context, _ []byte, p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[p]++
	for substr, err := range s.fail {
		if strings.Contains(p, substr) {
			return err
		}
	}

	return nil
}

func (s *pathStubBackend) callCount(substr string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for p, c := range s.calls {
		if strings.Contains(p, substr) {
			n += c
		}
	}

	return n
}

// stubExportRequest builds a RunExportEnvelope request for a stub sink named per invocation.
func stubExportRequest(t *testing.T, backend Backend, objectPath string) ExportEnvelopeRequest {
	t.Helper()
	const sinkNamespace = "cb-boundary-ns"
	sinkName := uniqueBreakerName(t)

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sinkNamespace},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             "stub",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
	}
	reg := NewRegistry()
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return backend, nil
	})
	envelope, err := export.MarshalEnvelope([]collect.Item{{Name: "demo"}}, export.Metadata{Generation: 1})
	if err != nil {
		t.Fatal(err)
	}

	return ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    objectPath,
		Envelope:      envelope,
		SinkSpec:      kollectdevv1alpha1.KollectSinkSpec{Type: "stub", Endpoint: "https://example.com/repo.git"},
	}
}

// SCB-1a: inventory-scoped results do not count as failures.
func TestExportThroughBreaker_inventoryScopedResultsDoNotTrip(t *testing.T) {
	t.Parallel()
	key := uniqueBreakerKey(t)

	for i := range 10 {
		if _, err := callBreaker(key, kollecterrors.InventoryScoped(errInventoryConflict)); !errors.Is(err, errInventoryConflict) {
			t.Fatalf("call %d: err = %v, want the inventory-scoped result itself", i+1, err)
		}
	}
	ran, err := callBreaker(key, nil)
	if !ran || err != nil {
		t.Fatalf("11th call after 10 inventory-scoped results: ran = %v, err = %v; want ran, nil", ran, err)
	}
}

// SCB-1b: one inventory's inventory-scoped rejections do not block another inventory on the same
// sink, through RunExportEnvelope.
func TestRunExportEnvelope_inventoryScopedRejectionsSpareOtherInventory(t *testing.T) {
	t.Parallel()
	backend := &pathStubBackend{fail: map[string]error{"inv-a": kollecterrors.InventoryScoped(errInventoryConflict)}}
	reqA := stubExportRequest(t, backend, "cb-boundary-ns/inv-a.json")
	reqB := reqA
	reqB.ObjectPath = "cb-boundary-ns/inv-b.json"

	var errA error
	for i := range 10 {
		if _, errA = RunExportEnvelope(reqA); errA == nil {
			t.Fatalf("export A #%d succeeded, want a rejection", i+1)
		}
	}
	if _, err := RunExportEnvelope(reqB); err != nil {
		t.Fatalf("export B after 10 inventory-scoped rejections of A: %v", err)
	}
	if !kollecterrors.IsTerminal(errA) || !kollecterrors.IsInventoryScoped(errA) || !errors.Is(errA, errInventoryConflict) {
		t.Fatalf("export A's 10th error = %v (class %s), want the terminal inventory-scoped rejection", errA, kollecterrors.ClassOf(errA))
	}
	if got := backend.callCount("inv-a"); got != 10 {
		t.Fatalf("export A reached the backend %d times, want 10", got)
	}
	if got := backend.callCount("inv-b"); got != 1 {
		t.Fatalf("export B reached the backend %d times, want 1", got)
	}
}

// SCB-1c: an excluded result neither counts nor resets the consecutive-failure streak.
func TestExportThroughBreaker_inventoryScopedKeepsFailureStreak(t *testing.T) {
	t.Parallel()
	key := uniqueBreakerKey(t)
	transient := kollecterrors.Transient(errors.New("network down"))

	for i, result := range []error{transient, kollecterrors.InventoryScoped(errInventoryConflict), transient, transient, transient} {
		if ran, _ := callBreaker(key, result); !ran {
			t.Fatalf("call %d did not run", i+1)
		}
	}
	ran, _ := callBreaker(key, transient)
	if !ran {
		t.Fatal("sixth call (fourth counted failure) did not run: the inventory-scoped result counted")
	}
	ran, err := callBreaker(key, nil)
	if ran {
		t.Fatal("seventh call ran: the breaker did not open on the fifth counted failure")
	}
	requireOpenBreakerError(t, err)
}

// SCB-2a/2b/2c/2f (controls): every unmarked failure counts, whatever its class.
func TestExportThroughBreaker_unmarkedFailuresTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"transient", kollecterrors.Transient(errors.New("network down"))},
		{"unclassified", errors.New("boom")},
		{"forbidden", kollecterrors.Forbidden(errors.New("denied"))},
		{"unwrapped-notfound", apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "x")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := uniqueBreakerKey(t)
			for i := range circuitBreakerTripAt {
				if ran, _ := callBreaker(key, tc.err); !ran {
					t.Fatalf("call %d did not run before the trip", i+1)
				}
			}
			ran, err := callBreaker(key, nil)
			if ran {
				t.Fatalf("sixth call ran after %d %s failures", circuitBreakerTripAt, tc.name)
			}
			requireOpenBreakerError(t, err)
		})
	}
}

// SCB-2d (control): a sink-wide Terminal from the Git classifier still opens the breaker.
func TestRunExportEnvelope_sinkWideTerminalTrips(t *testing.T) {
	t.Parallel()
	authErr := git.ClassifyExportError(errors.New("authentication required"))
	if !kollecterrors.IsTerminal(authErr) {
		t.Fatalf("precondition: git auth error class = %s, want terminal", kollecterrors.ClassOf(authErr))
	}
	backend := &pathStubBackend{fail: map[string]error{"inv": authErr}}
	req := stubExportRequest(t, backend, "cb-boundary-ns/inv.json")

	for range circuitBreakerTripAt {
		_, _ = RunExportEnvelope(req)
	}
	_, err := RunExportEnvelope(req)
	if got := backend.callCount("inv"); got != int(circuitBreakerTripAt) {
		t.Fatalf("backend reached %d times, want %d (sixth export must not reach it)", got, circuitBreakerTripAt)
	}
	requireOpenBreakerError(t, err)
}

// halfOpenBreaker stores a 1 s breaker under a fresh key, opens it with Transient failures and
// sleeps once past its timeout (design.md timing protocol). Later steps must not sleep.
func halfOpenBreaker(t *testing.T) string {
	t.Helper()
	key := uniqueBreakerKey(t)
	if _, loaded := breakerRegistry.LoadOrStore(key, newBreaker(key, time.Second)); loaded {
		t.Fatalf("breaker key %q already in use", key)
	}
	for range circuitBreakerTripAt {
		_, _ = callBreaker(key, kollecterrors.Transient(errors.New("network down")))
	}
	if st := breakerFor(key).State(); st != gobreaker.StateOpen {
		t.Fatalf("state after %d transient failures = %s, want open", circuitBreakerTripAt, st)
	}
	time.Sleep(1100 * time.Millisecond)

	return key
}

// SCB-3a: an inventory-scoped trial leaves the breaker half-open; the next success closes it.
func TestExportThroughBreaker_halfOpenInventoryScopedThenSuccess(t *testing.T) {
	t.Parallel()
	key := halfOpenBreaker(t)

	if ran, _ := callBreaker(key, kollecterrors.InventoryScoped(errInventoryConflict)); !ran {
		t.Fatal("trial did not run after the timeout")
	}
	if st := breakerFor(key).State(); st != gobreaker.StateHalfOpen {
		t.Fatalf("state after an inventory-scoped trial = %s, want half-open", st)
	}
	ran, err := callBreaker(key, nil)
	if !ran || err != nil {
		t.Fatalf("next trial: ran = %v, err = %v; want ran, nil", ran, err)
	}
	if st := breakerFor(key).State(); st != gobreaker.StateClosed {
		t.Fatalf("state after a successful trial = %s, want closed", st)
	}
}

// SCB-3b: an inventory-scoped trial, then a counted failure, re-opens the breaker.
func TestExportThroughBreaker_halfOpenInventoryScopedThenFailure(t *testing.T) {
	t.Parallel()
	key := halfOpenBreaker(t)

	if ran, _ := callBreaker(key, kollecterrors.InventoryScoped(errInventoryConflict)); !ran {
		t.Fatal("trial did not run after the timeout")
	}
	ran, _ := callBreaker(key, kollecterrors.Transient(errors.New("network down")))
	if !ran {
		t.Fatal("the export after an inventory-scoped trial was not admitted as the trial")
	}
	if st := breakerFor(key).State(); st != gobreaker.StateOpen {
		t.Fatalf("state after a failing trial = %s, want open", st)
	}
	if ran, err := callBreaker(key, nil); ran {
		t.Fatal("a call ran on the re-opened breaker")
	} else {
		requireOpenBreakerError(t, err)
	}
}

// SCB-3c: a concurrent export does not run while a trial is in flight; an inventory-scoped trial
// frees the slot for the next export.
func TestExportThroughBreaker_halfOpenConcurrentTrial(t *testing.T) {
	t.Parallel()
	key := halfOpenBreaker(t)

	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- exportThroughBreaker(key, func() error {
			close(started)
			<-release

			return kollecterrors.InventoryScoped(errInventoryConflict)
		})
	}()
	<-started
	ran, err := callBreaker(key, nil)
	if ran || err == nil {
		t.Fatalf("concurrent call during the trial: ran = %v, err = %v; want not run, an error", ran, err)
	}
	close(release)
	if trialErr := <-done; !errors.Is(trialErr, errInventoryConflict) {
		t.Fatalf("trial err = %v, want the inventory-scoped result", trialErr)
	}
	if ran, err := callBreaker(key, nil); !ran || err != nil {
		t.Fatalf("export after an inventory-scoped trial: ran = %v, err = %v; want ran, nil", ran, err)
	}
}

// SCB-1e: a real Git ownership rejection reaches the breaker inventory-scoped, and does not block a
// different inventory on the same sink. Not parallel: newIdentityRemote resets every breaker.
func TestRunExportEnvelope_gitOwnershipRejectionSparesOtherInventory(t *testing.T) {
	r := newIdentityRemote(t)
	sinkName := uniqueBreakerName(t)
	exportTo := func(id InventoryIdentity, objectPath string, items []collect.Item) error {
		t.Helper()
		envelope, err := export.MarshalEnvelope(items, export.Metadata{Generation: 1, ExportedAt: time.Now().UTC()})
		if err != nil {
			t.Fatalf("marshal envelope: %v", err)
		}
		_, err = RunExportEnvelope(ExportEnvelopeRequest{
			Ctx: t.Context(), Registry: NewRegistry(), SinkNamespace: "default", SinkName: sinkName,
			ObjectPath: objectPath, Inventory: id, Envelope: envelope, SinkSpec: r.spec,
		})

		return err
	}

	if err := exportTo(clusterPlatform, platformObjectPath, []collect.Item{identityItem("team-a", "Deployment", "api", "L")}); err != nil {
		t.Fatalf("owner export: %v", err)
	}
	for i := range circuitBreakerTripAt + 1 {
		err := exportTo(namespacedPlatform, platformObjectPath, []collect.Item{identityItem("team-a", "Deployment", "api", "N")})
		if !kollecterrors.IsTerminal(err) || !kollecterrors.IsInventoryScoped(err) || !strings.Contains(err.Error(), "belongs to another inventory") {
			t.Fatalf("ownership rejection #%d: err = %v (class %s, inventory-scoped %v), want a terminal inventory-scoped rejection",
				i+1, err, kollecterrors.ClassOf(err), kollecterrors.IsInventoryScoped(err))
		}
	}
	other := InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-b", Name: "other"}
	if err := exportTo(other, "inventory/team-b/other.json", []collect.Item{identityItem("team-b", "ConfigMap", "settings", "B")}); err != nil {
		t.Fatalf("other inventory's export after %d ownership rejections: %v", circuitBreakerTripAt+1, err)
	}
	if _, ok := r.files()["default/team-b/configmap/settings.yaml"]; !ok {
		t.Fatalf("other inventory's file missing; files %v", dataPaths(r.files()))
	}
}
