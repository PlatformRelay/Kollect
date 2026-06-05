// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

const statusUnknown = "unknown"

// StatusReader lists CRD status for optional Read API proxy endpoints (B3).
type StatusReader interface {
	ListInventoryStatus(ctx context.Context, namespace string) ([]ResourceStatus, error)
	ListTargetStatus(ctx context.Context, namespace string) ([]ResourceStatus, error)
	GetInventoryExportStatus(ctx context.Context, namespace, name string) ([]ExportStatus, error)
}

// ClientStatusReader implements StatusReader using a controller-runtime client.
type ClientStatusReader struct {
	Client client.Reader
}

func (r *ClientStatusReader) ListInventoryStatus(
	ctx context.Context,
	namespace string,
) ([]ResourceStatus, error) {
	list := &kollectdevv1alpha1.KollectInventoryList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := r.Client.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list kollectinventories: %w", err)
	}

	out := make([]ResourceStatus, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, inventoryResourceStatus(&list.Items[i]))
	}

	return out, nil
}

func (r *ClientStatusReader) ListTargetStatus(
	ctx context.Context,
	namespace string,
) ([]ResourceStatus, error) {
	list := &kollectdevv1alpha1.KollectTargetList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := r.Client.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list kollecttargets: %w", err)
	}

	out := make([]ResourceStatus, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, targetResourceStatus(&list.Items[i]))
	}

	return out, nil
}

func (r *ClientStatusReader) GetInventoryExportStatus(
	ctx context.Context,
	namespace, name string,
) ([]ExportStatus, error) {
	if namespace == "" || name == "" {
		return nil, nil
	}

	var inv kollectdevv1alpha1.KollectInventory
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &inv); err != nil {
		return nil, fmt.Errorf("get kollectinventory: %w", err)
	}

	return exportStatusFromInventory(&inv), nil
}

func inventoryResourceStatus(inv *kollectdevv1alpha1.KollectInventory) ResourceStatus {
	rs := ResourceStatus{
		Name:               inv.Name,
		Namespace:          inv.Namespace,
		Generation:         inv.Generation,
		ObservedGeneration: inv.Status.ObservedGeneration,
		ItemCount:          inv.Status.ItemCount,
		Conditions:         inv.Status.Conditions,
	}
	if inv.Status.LastExportTime != nil {
		rs.LastExportTime = inv.Status.LastExportTime.UTC().Format(time.RFC3339)
	}

	return rs
}

func targetResourceStatus(target *kollectdevv1alpha1.KollectTarget) ResourceStatus {
	return ResourceStatus{
		Name:               target.Name,
		Namespace:          target.Namespace,
		Generation:         target.Generation,
		ObservedGeneration: target.Status.ObservedGeneration,
		Conditions:         target.Status.Conditions,
	}
}

func exportStatusFromInventory(inv *kollectdevv1alpha1.KollectInventory) []ExportStatus {
	if len(inv.Spec.SinkRefs) == 0 {
		return nil
	}

	synced := apimeta.FindStatusCondition(inv.Status.Conditions, kollectdevv1alpha1.ConditionSynced)
	status := statusUnknown
	message := ""
	if synced != nil {
		switch synced.Status {
		case metav1.ConditionTrue:
			status = "ok"
		case metav1.ConditionFalse:
			status = "degraded"
		default:
			status = statusUnknown
		}

		message = synced.Message
	}

	lastExport := ""
	if inv.Status.LastExportTime != nil {
		lastExport = inv.Status.LastExportTime.UTC().Format(time.RFC3339)
	}

	out := make([]ExportStatus, 0, len(inv.Spec.SinkRefs))
	for _, sinkName := range inv.Spec.SinkRefs {
		out = append(out, ExportStatus{
			SinkName:       sinkName,
			SinkNamespace:  inv.Namespace,
			Status:         status,
			LastExportTime: lastExport,
			Message:        message,
		})
	}

	return out
}

func (s *Server) handleStatusInventories(w http.ResponseWriter, r *http.Request) {
	s.writeStatusList(w, r, "inventories")
}

func (s *Server) handleStatusTargets(w http.ResponseWriter, r *http.Request) {
	s.writeStatusList(w, r, "targets")
}

func (s *Server) writeStatusList(w http.ResponseWriter, r *http.Request, kind string) {
	if s.Status == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StatusListResponse{
			SchemaVersion: collect.ExportSchemaVersion,
			Items:         []ResourceStatus{},
		})

		return
	}

	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))

	var (
		items []ResourceStatus
		err   error
	)

	ctx := r.Context()
	switch kind {
	case "inventories":
		items, err = s.Status.ListInventoryStatus(ctx, namespace)
	case "targets":
		items, err = s.Status.ListTargetStatus(ctx, namespace)
	default:
		http.Error(w, "unknown status kind", http.StatusInternalServerError)

		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if items == nil {
		items = []ResourceStatus{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(StatusListResponse{
		SchemaVersion: collect.ExportSchemaVersion,
		Items:         items,
	})
}
