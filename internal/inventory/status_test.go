// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestClientStatusReader_listsAndExportStatus(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a", Generation: 2},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: kollectdevv1alpha1.NewSinkRefList("git", "s3")},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			ObservedGeneration: 2,
			ItemCount:          3,
			LastExportTime:     &now,
			Conditions: []metav1.Condition{{
				Type:    kollectdevv1alpha1.ConditionSynced,
				Status:  metav1.ConditionTrue,
				Message: "exported",
			}},
		},
	}
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a", Generation: 1},
		Status:     kollectdevv1alpha1.KollectTargetStatus{ObservedGeneration: 1},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv, target).Build()
	reader := &ClientStatusReader{Client: cl}

	inventories, err := reader.ListInventoryStatus(context.Background(), "team-a")
	if err != nil || len(inventories) != 1 || inventories[0].ItemCount != 3 {
		t.Fatalf("inventories = %#v err=%v", inventories, err)
	}
	if inventories[0].LastExportTime == "" {
		t.Fatal("expected last export time")
	}

	targets, err := reader.ListTargetStatus(context.Background(), "team-a")
	if err != nil || len(targets) != 1 || targets[0].Name != "deploys" {
		t.Fatalf("targets = %#v err=%v", targets, err)
	}

	exportStatus, err := reader.GetInventoryExportStatus(context.Background(), "team-a", "platform")
	if err != nil || len(exportStatus) != 2 || exportStatus[0].Status != "ok" {
		t.Fatalf("export status = %#v err=%v", exportStatus, err)
	}

	if got, err := reader.GetInventoryExportStatus(context.Background(), "", "platform"); err != nil || got != nil {
		t.Fatalf("empty namespace = %#v err=%v", got, err)
	}
}

func TestExportStatusFromInventory_degraded(t *testing.T) {
	t.Parallel()

	inv := &kollectdevv1alpha1.KollectInventory{
		Spec: kollectdevv1alpha1.KollectInventorySpec{SinkRefs: kollectdevv1alpha1.NewSinkRefList("git")},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			Conditions: []metav1.Condition{{
				Type:    kollectdevv1alpha1.ConditionSynced,
				Status:  metav1.ConditionFalse,
				Message: "sink down",
			}},
		},
	}

	got := exportStatusFromInventory(inv)
	if len(got) != 1 || got[0].Status != "degraded" || got[0].Message != "sink down" {
		t.Fatalf("export status = %#v", got)
	}

	if exportStatusFromInventory(&kollectdevv1alpha1.KollectInventory{}) != nil {
		t.Fatal("no sink refs should return nil")
	}
}

func TestServerStatusHandlers(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a"},
	}
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv, target).Build()

	srv := &Server{
		Enabled: true,
		Status:  &ClientStatusReader{Client: cl},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/inventories?namespace=team-a", nil)
	rec := httptest.NewRecorder()
	srv.handleStatusInventories(rec, req)

	var resp StatusListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "platform" {
		t.Fatalf("inventories = %#v", resp.Items)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1alpha1/status/targets?namespace=team-a", nil)
	rec = httptest.NewRecorder()
	srv.handleStatusTargets(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "deploys" {
		t.Fatalf("targets = %#v", resp.Items)
	}

	emptySrv := &Server{Enabled: true}
	req = httptest.NewRequest(http.MethodGet, "/v1alpha1/status/inventories", nil)
	rec = httptest.NewRecorder()
	emptySrv.handleStatusInventories(rec, req)
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("nil status reader should return empty list, got %#v", resp.Items)
	}
}

func TestRequestAuthScope_statusPaths(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/status/inventories?namespace=team-a", nil)
	ns, name, verb, resource := requestAuthScope(req)
	if ns != "team-a" || name != "" || verb != "list" || resource != "kollectinventories" {
		t.Fatalf("scope = (%q,%q,%q,%q)", ns, name, verb, resource)
	}
}

func TestInventoryResourceStatus_lastExport(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "team-a", Generation: 4},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			ObservedGeneration: 4,
			ItemCount:          9,
			LastExportTime:     &now,
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	got := inventoryResourceStatus(inv)
	if got.ItemCount != 9 || got.LastExportTime != now.UTC().Format(time.RFC3339) {
		t.Fatalf("status = %#v", got)
	}

	inv.Status.LastExportTime = nil
	apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:   kollectdevv1alpha1.ConditionSynced,
		Status: metav1.ConditionUnknown,
	})
	for _, sink := range exportStatusFromInventory(&kollectdevv1alpha1.KollectInventory{
		Spec:   kollectdevv1alpha1.KollectInventorySpec{SinkRefs: kollectdevv1alpha1.NewSinkRefList("git")},
		Status: inv.Status,
	}) {
		if sink.Status != "unknown" {
			t.Fatalf("unknown synced status = %q", sink.Status)
		}
	}
}
