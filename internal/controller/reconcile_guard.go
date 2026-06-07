// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"runtime/debug"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// guardReconcile recovers panics at reconcile entry (GUIDELINES §1, EC-P2-01).
func guardReconcile(
	ctx context.Context,
	recorder record.EventRecorder,
	obj runtime.Object,
	fn func() (ctrl.Result, error),
) (result ctrl.Result, err error) {
	log := logf.FromContext(ctx)

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(fmt.Errorf("panic: %v", recovered), "reconcile panic recovered",
				"stack", string(debug.Stack()))
			if recorder != nil && obj != nil {
				recorder.Event(obj, corev1.EventTypeWarning, "ReconcilePanic",
					fmt.Sprintf("panic recovered: %v", recovered))
			}
			result = ctrl.Result{Requeue: true}
			err = nil
		}
	}()

	return fn()
}
