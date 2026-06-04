// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// KollectTargetReconciler reconciles a KollectTarget object
type KollectTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectprofiles,verbs=get;list;watch

// Reconcile validates the target spec and referenced profile before collection is wired.
func (r *KollectTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var target kollectdevv1alpha1.KollectTarget
	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if target.Spec.Suspend {
		log.Info("target suspended", "name", target.Name, "namespace", target.Namespace)

		return ctrl.Result{}, nil
	}

	if target.Spec.ProfileRef == "" {
		return r.setDegraded(ctx, &target, "MissingProfileRef", "spec.profileRef is required")
	}

	var profile kollectdevv1alpha1.KollectProfile
	if err := r.Get(ctx, client.ObjectKey{Name: target.Spec.ProfileRef}, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setDegraded(ctx, &target, "ProfileNotFound",
				fmt.Sprintf("KollectProfile %q not found", target.Spec.ProfileRef))
		}

		return ctrl.Result{}, err
	}

	return r.setReady(ctx, &target)
}

func (r *KollectTargetReconciler) setDegraded(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	reason, message string,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionReady)
	if err := setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionDegraded, metav1.ConditionTrue, reason, message,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectTargetReconciler) setReady(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionDegraded)
	target.Status.ObservedGeneration = target.Generation

	if err := setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionReady, metav1.ConditionTrue, "ProfileResolved",
		fmt.Sprintf("profileRef %q resolved", target.Spec.ProfileRef),
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectTarget{}).
		Named("kollecttarget").
		Complete(r)
}
