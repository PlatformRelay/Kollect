// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/metrics"
)

const inventoryCleanupFinalizer = "kollect.dev/inventory-cleanup"

// terminalCleanupRequeue keeps a terminal cleanup wedge re-checking on a fixed
// cadence (K-30): the wedge stays alertable via kollect_cleanup_terminal_total
// and the Warning Event, but the documented first remedy — fixing the sink
// credential (a Secret edit, which fires no watch) — self-heals within minutes
// instead of waiting out the 12-hour informer resync. Nil-error RequeueAfter
// deliberately avoids the backoff escalation a returned error would trigger.
const terminalCleanupRequeue = 5 * time.Minute

func (r *KollectInventoryReconciler) ensureInventoryFinalizer(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	return ensureFinalizer(ctx, r.Client, inv, inventoryCleanupFinalizer)
}

func (r *KollectInventoryReconciler) finalizeInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (ctrl.Result, error) {
	if !containsFinalizer(inv.Finalizers, inventoryCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	// K-30 escape hatch: operator-acknowledged escape from a terminal cleanup
	// wedge; drop the finalizer without backend contact.
	if forceCleanupRequested(inv.Annotations) {
		recordCleanupForced(r.Recorder, inv, inventoryCleanupFinalizer)

		return removeFinalizerAndUpdate(ctx, r.Client, inv, inventoryCleanupFinalizer)
	}

	report, err := r.cleanupInventoryDeletion(ctx, inv)
	recordCleanupAnnouncements(r.Recorder, inv, report)

	if err != nil {
		logf.FromContext(ctx).Error(err, "inventory cleanup failed",
			"inventory", inv.Name, "namespace", inv.Namespace)

		if kollecterrors.IsTerminal(err) {
			// Returning a non-nil error would make controller-runtime requeue
			// with backoff, defeating the no-requeue intent for terminal errors.
			msg := fmt.Sprintf(
				"sink cleanup failed terminally: %v — fix the sink configuration, set the %q annotation to \"true\", or remove the %q finalizer manually",
				err, kollectdevv1alpha1.AnnotationForceCleanup, inventoryCleanupFinalizer)
			recordWarning(r.Recorder, inv, reasonCleanupTerminal, msg)
			// K-30: the wedge needs a counter an operator can alert on.
			metrics.CleanupTerminalTotal.WithLabelValues("inventory").Inc()
			// Best-effort Degraded status: the object is deleting, update errors are ignored.
			_, _ = r.setInventoryDegraded(ctx, inv, inv.Status.ItemCount, reasonCleanupTerminal, msg)

			// Re-check on a fixed cadence so a fix of the sink configuration —
			// in particular a Secret data edit, which fires no watch event —
			// clears the wedge within minutes rather than at the informer
			// resync. A nil error keeps controller-runtime off the backoff path;
			// the terminal counter and Warning Event above stay the alertable
			// signal.
			return ctrl.Result{RequeueAfter: terminalCleanupRequeue}, nil
		}

		return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, err
	}

	return removeFinalizerAndUpdate(ctx, r.Client, inv, inventoryCleanupFinalizer)
}

func (r *KollectInventoryReconciler) cleanupInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (SinkCleanupReport, error) {
	return cleanupSinkExports(
		ctx,
		r.Client,
		r.Registry,
		inv.Namespace,
		inventorySinkBindings(inv),
		false,
		fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name),
		inv.Generation,
	)
}

// forceCleanupRequested reports whether the operator asked for the K-30 escape
// hatch. Callers only invoke the finalizer path while the object is deleting,
// so a live object can never skip cleanup this way.
func forceCleanupRequested(annotations map[string]string) bool {
	return annotations[kollectdevv1alpha1.AnnotationForceCleanup] == kollectdevv1alpha1.ForceCleanupTrue
}

func recordCleanupForced(recorder record.EventRecorder, inv client.Object, finalizer string) {
	recordWarning(recorder, inv, reasonCleanupForced, fmt.Sprintf(
		"annotation %s=%s: dropping cleanup finalizer %q without backend retraction; "+
			"previously exported objects are NOT deleted and may need manual removal",
		kollectdevv1alpha1.AnnotationForceCleanup, kollectdevv1alpha1.ForceCleanupTrue, finalizer))
}
