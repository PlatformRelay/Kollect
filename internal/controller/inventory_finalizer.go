// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink"
)

const inventoryCleanupFinalizer = "kollect.dev/inventory-cleanup"

func (r *KollectInventoryReconciler) ensureInventoryFinalizer(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	if controllerutil.ContainsFinalizer(inv, inventoryCleanupFinalizer) {
		return nil
	}

	controllerutil.AddFinalizer(inv, inventoryCleanupFinalizer)

	return r.Update(ctx, inv)
}

func (r *KollectInventoryReconciler) finalizeInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(inv, inventoryCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupInventorySinks(ctx, inv); err != nil {
		logf.FromContext(ctx).Error(err, "inventory sink cleanup failed",
			"inventory", inv.Name, "namespace", inv.Namespace)

		result := ctrl.Result{RequeueAfter: r.exportDebounce(inv)}
		if kollecterrors.IsTerminal(err) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	controllerutil.RemoveFinalizer(inv, inventoryCleanupFinalizer)
	if err := r.Update(ctx, inv); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectInventoryReconciler) cleanupInventorySinks(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	if r.Registry == nil || len(inv.Spec.SinkRefs) == 0 {
		return nil
	}

	var errs []error
	for _, ref := range inv.Spec.SinkRefs {
		if err := sink.RunExportItems(sink.ExportItemsRequest{
			Ctx:           ctx,
			Client:        r.Client,
			Registry:      r.Registry,
			SinkNamespace: inv.Namespace,
			SinkName:      ref.Name,
			ObjectPath:    fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name),
			Items:         []collect.Item{},
			Meta:          export.Metadata{Generation: inv.Generation},
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
