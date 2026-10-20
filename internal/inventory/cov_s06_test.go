// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// --- status.go: per-sink SinkExports resolution (exportStatusFromInventory) ---

// TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus locks SDC-6. The
// per-sink status comes from the status.sinkExports entry named "<family>/<name>", the name the
// reconcilers write (sinkExportKey). Mapping is reason first: Debounced -> debounced for either
// status, then True -> ok, False -> degraded, Unknown -> unknown. A matched entry's
// lastExportTime is reported as is, empty when it has none; the inventory-wide status and time
// apply only to a binding with no entry. An entry whose name matches no binding is ignored, and
// a bare-name entry (no family) matches nothing.
func TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus(t *testing.T) {
	t.Parallel()

	invExport := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	okExport := metav1.NewTime(time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC))
	snapshotGitExport := metav1.NewTime(time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC))
	databaseGitExport := metav1.NewTime(time.Date(2026, 9, 2, 9, 30, 0, 0, time.UTC))
	rfc := func(ts metav1.Time) string { return ts.UTC().Format(time.RFC3339) }
	synced := func(status metav1.ConditionStatus, reason, message string) []metav1.Condition {
		return []metav1.Condition{{
			Type: kollectdevv1alpha1.ConditionSynced, Status: status, Reason: reason, Message: message,
		}}
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList(
				"git-ok", "git-debounced", "git-degraded", "git-unknown", "git", "git-no-entry"),
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("git"),
		},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			LastExportTime: &invExport,
			// Inventory-wide condition is degraded; per-sink conditions must override it.
			Conditions: synced(metav1.ConditionFalse, "Progressing", "inventory-wide message"),
			SinkExports: []kollectdevv1alpha1.InventorySinkExportStatus{
				{
					Name:           "snapshot/git-ok",
					LastExportTime: &okExport,
					Conditions:     synced(metav1.ConditionTrue, "Exported", "sink ok"),
				},
				{
					// No lastExportTime: never exported successfully.
					Name:       "snapshot/git-debounced",
					Conditions: synced(metav1.ConditionFalse, kollectdevv1alpha1.ReasonDebounced, "unchanged within interval"),
				},
				{
					Name:       "snapshot/git-degraded",
					Conditions: synced(metav1.ConditionFalse, "SinkError", "push rejected"),
				},
				{
					Name:       "snapshot/git-unknown",
					Conditions: []metav1.Condition{{Type: kollectdevv1alpha1.ConditionSynced, Status: metav1.ConditionUnknown}},
				},
				{
					Name:           "snapshot/git",
					LastExportTime: &snapshotGitExport,
					Conditions:     synced(metav1.ConditionTrue, kollectdevv1alpha1.ReasonDebounced, "payload unchanged"),
				},
				{
					Name:           "database/git",
					LastExportTime: &databaseGitExport,
					Conditions:     synced(metav1.ConditionTrue, "Exported", "export completed"),
				},
				// Bare-name entry: not the controller's "<family>/<name>" key -> ignored.
				{
					Name:           "git-no-entry",
					LastExportTime: &okExport,
					Conditions:     synced(metav1.ConditionTrue, "Exported", "bare name"),
				},
				// Orphan export status: no binding named "stale-sink" -> ignored.
				{
					Name:       "snapshot/stale-sink",
					Conditions: synced(metav1.ConditionTrue, "Exported", ""),
				},
			},
		},
	}

	got := exportStatusFromInventory(inv)
	type want struct{ name, status, message, lastExport string }
	wants := []want{
		{"git-ok", "ok", "sink ok", rfc(okExport)},
		{"git-debounced", "debounced", "unchanged within interval", ""},
		{"git-degraded", statusDegraded, "push rejected", ""},
		{"git-unknown", statusUnknown, "", ""},
		{"git", "debounced", "payload unchanged", rfc(snapshotGitExport)},
		{"git-no-entry", statusDegraded, "inventory-wide message", rfc(invExport)},
		{"git", "ok", "export completed", rfc(databaseGitExport)},
	}
	if len(got) != len(wants) {
		t.Fatalf("expected %d export statuses (one per binding, orphan ignored), got %d: %#v", len(wants), len(got), got)
	}
	for i, w := range wants {
		es := got[i]
		if es.SinkName != w.name || es.Status != w.status || es.Message != w.message || es.LastExportTime != w.lastExport {
			t.Errorf("binding %d = {name:%q status:%q message:%q lastExportTime:%q}, want {name:%q status:%q message:%q lastExportTime:%q}",
				i, es.SinkName, es.Status, es.Message, es.LastExportTime, w.name, w.status, w.message, w.lastExport)
		}
	}
}

// --- server.go: collectItems store/filter branches + buildSummary nil-items ---

// TestCollectItems_nilStoreYieldsEmptySummary locks the collectItems nil-store guard: with
// no store attached, buildSummary must produce an empty (never nil) item list and not panic.
func TestCollectItems_nilStoreYieldsEmptySummary(t *testing.T) {
	t.Parallel()

	srv := &Server{Enabled: true} // Store is nil
	summary := srv.buildSummary(context.Background(), ListFilter{Namespace: "team-a", Inventory: "platform"})

	if summary.ItemCount != 0 {
		t.Errorf("ItemCount = %d, want 0 for a nil store", summary.ItemCount)
	}
	if summary.Items == nil {
		t.Error("Items must be a non-nil empty slice, got nil")
	}
	if summary.Pagination != nil {
		t.Errorf("Pagination = %#v, want nil when there are no items", summary.Pagination)
	}
}

// TestCollectItems_noNamespaceNoInventoryUsesGlobalSummary locks the unfiltered branch:
// when neither namespace nor inventory is set, collectItems must read the store's global
// summary rather than a namespace-scoped one.
func TestCollectItems_noNamespaceNoInventoryUsesGlobalSummary(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a", TargetName: "deploys",
		Namespace: "apps", Name: "web", UID: "uid-1", Version: "v1", Kind: "Deployment",
	})
	store.Upsert(collect.Item{
		TargetNamespace: "team-b", TargetName: "deploys",
		Namespace: "infra", Name: "db", UID: "uid-2", Version: "v1", Kind: "StatefulSet",
	})

	srv := &Server{Enabled: true, Store: store}
	summary := srv.buildSummary(context.Background(), ListFilter{}) // no namespace, no inventory

	if summary.ItemCount != 2 {
		t.Errorf("ItemCount = %d, want 2 (global summary across namespaces)", summary.ItemCount)
	}
}

// --- status.go: reader List/Get error propagation + writeStatusList branches ---

// erroringReader forces List and Get to fail so the status reader's error-wrapping paths
// are exercised without a live apiserver.
func erroringReader(t *testing.T) *ClientStatusReader {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("apiserver unavailable")
	cl := interceptor.NewClient(
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return boom
			},
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return boom
			},
		},
	)

	return &ClientStatusReader{Client: cl}
}

func TestClientStatusReader_listErrorsAreWrapped(t *testing.T) {
	t.Parallel()

	reader := erroringReader(t)

	if _, err := reader.ListInventoryStatus(context.Background(), "team-a"); err == nil {
		t.Error("ListInventoryStatus: expected wrapped error, got nil")
	}
	if _, err := reader.ListTargetStatus(context.Background(), "team-a"); err == nil {
		t.Error("ListTargetStatus: expected wrapped error, got nil")
	}
	if _, err := reader.GetInventoryExportStatus(context.Background(), "team-a", "platform"); err == nil {
		t.Error("GetInventoryExportStatus: expected wrapped error, got nil")
	}
}

// TestWriteStatusList_readerErrorIs500 locks the error branch of writeStatusList: a status
// reader failure must surface as HTTP 500, not a partial or empty 200 body.
func TestWriteStatusList_readerErrorIs500(t *testing.T) {
	t.Parallel()

	srv := &Server{Enabled: true, Status: erroringReader(t)}
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/inventories?namespace=team-a", nil)
	rec := httptest.NewRecorder()

	srv.handleStatusInventories(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on reader error", rec.Code)
	}
}

// TestWriteStatusList_unknownKindIs500 locks the default arm of the kind switch: an
// unrecognized status kind is an internal error, not silently empty.
func TestWriteStatusList_unknownKindIs500(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	srv := &Server{Enabled: true, Status: &ClientStatusReader{Client: cl}}

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/bogus", nil)
	rec := httptest.NewRecorder()

	srv.writeStatusList(rec, req, "bogus")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for an unknown status kind", rec.Code)
	}
}

// nilItemsReader returns (nil, nil) for the list calls so writeStatusList's nil->empty
// normalization is exercised (ClientStatusReader always returns a non-nil slice, so a stub
// is required to reach that branch).
type nilItemsReader struct{}

func (nilItemsReader) ListInventoryStatus(context.Context, string) ([]ResourceStatus, error) {
	return nil, nil
}

func (nilItemsReader) ListTargetStatus(context.Context, string) ([]ResourceStatus, error) {
	return nil, nil
}

func (nilItemsReader) GetInventoryExportStatus(context.Context, string, string) ([]ExportStatus, error) {
	return nil, nil
}

// TestWriteStatusList_nilItemsRenderedAsEmptyArray locks the items==nil normalization: a
// reader that returns a nil slice must still serialize items as [] (never JSON null).
func TestWriteStatusList_nilItemsRenderedAsEmptyArray(t *testing.T) {
	t.Parallel()

	srv := &Server{Enabled: true, Status: nilItemsReader{}}
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/inventories", nil)
	rec := httptest.NewRecorder()

	srv.handleStatusInventories(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp StatusListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Items == nil {
		t.Error("Items serialized as null; want an empty array")
	}
	if len(resp.Items) != 0 {
		t.Errorf("Items = %#v, want empty", resp.Items)
	}
}

// --- filter.go: parseListFilter + itemMatchesFilter uncovered arms ---

// TestParseListFilter_negativeOffsetClampedToZero locks the offset<0 guard.
func TestParseListFilter_negativeOffsetClampedToZero(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory?offset=-42", nil)
	filter := parseListFilter(req)

	if filter.Offset != 0 {
		t.Errorf("Offset = %d, want 0 (negative offset clamped)", filter.Offset)
	}
}

// TestFilterItems_emptyInputReturnsNil locks the len==0 short-circuit in filterItems.
func TestFilterItems_emptyInputReturnsNil(t *testing.T) {
	t.Parallel()

	if got := filterItems(nil, ListFilter{Kind: "Deployment"}); got != nil {
		t.Errorf("filterItems(nil) = %#v, want nil", got)
	}
}

// TestItemMatchesFilter_groupVersionNameMismatches locks the group/version/name rejection
// arms of itemMatchesFilter, each of which excludes an item that fails only that predicate.
func TestItemMatchesFilter_groupVersionNameMismatches(t *testing.T) {
	t.Parallel()

	base := collect.Item{
		TargetName: "deploys", Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "apps", TargetNamespace: "team-a", Name: "web",
	}

	cases := map[string]ListFilter{
		"group mismatch":   {Group: "batch"},
		"version mismatch": {Version: "v2"},
		"name mismatch":    {Name: "cache"},
	}
	for label, f := range cases {
		if itemMatchesFilter(base, f) {
			t.Errorf("%s: expected item to be excluded, but it matched (filter=%#v)", label, f)
		}
	}

	// Control: an all-matching filter still admits the item.
	if !itemMatchesFilter(base, ListFilter{Group: "apps", Version: "v1", Name: "web"}) {
		t.Error("expected the item to match a fully-satisfied filter")
	}
}
