// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestExportStatusFromInventory(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: []string{"git", "postgres"}},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			LastExportTime: &now,
			Conditions: []metav1.Condition{{
				Type:    kollectdevv1alpha1.ConditionSynced,
				Status:  metav1.ConditionTrue,
				Message: "exported",
			}},
		},
	}

	statuses := exportStatusFromInventory(inv)
	if len(statuses) != 2 || statuses[0].Status != "ok" || statuses[0].SinkName != "git" {
		t.Fatalf("statuses = %#v", statuses)
	}
}

func TestClientStatusReaderListInventories(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "demo"},
		Status:     kollectdevv1alpha1.KollectInventoryStatus{ItemCount: 3},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()
	reader := &ClientStatusReader{Client: cl}

	items, err := reader.ListInventoryStatus(context.Background(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ItemCount != 3 {
		t.Fatalf("items = %#v", items)
	}
}

func TestServerHandleStatusTargetsNilReader(t *testing.T) {
	t.Parallel()

	srv := &Server{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/targets?namespace=team-a", nil)
	rec := httptest.NewRecorder()
	srv.handleStatusTargets(rec, req)

	var resp StatusListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.SchemaVersion != collect.ExportSchemaVersion || len(resp.Items) != 0 {
		t.Fatalf("resp = %#v", resp)
	}
}

func TestServerBuildSummaryWithExportStatus(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "demo"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: []string{"git"}},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			LastExportTime: &now,
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionSynced,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()
	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		Namespace:       "apps",
		Name:            "web",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	srv := &Server{
		Store:  store,
		Status: &ClientStatusReader{Client: cl},
	}

	summary := srv.buildSummary(context.Background(), ListFilter{
		Namespace: "team-a",
		Inventory: "demo",
	})
	if summary.SchemaVersion != collect.ExportSchemaVersion {
		t.Fatalf("schemaVersion = %q", summary.SchemaVersion)
	}
	if len(summary.ExportStatus) != 1 || summary.ExportStatus[0].SinkName != "git" {
		t.Fatalf("exportStatus = %#v", summary.ExportStatus)
	}
}
