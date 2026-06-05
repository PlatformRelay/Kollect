// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// KollectTargetReconciler reconciles a KollectTarget object
type KollectTargetReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Engine  *collect.Engine
	Options RuntimeOptions
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectprofiles,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

// Reconcile validates the target spec, registers collection, and updates status.
func (r *KollectTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollecttarget")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var target kollectdevv1alpha1.KollectTarget
	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		if apierrors.IsNotFound(err) {
			if r.Engine != nil {
				r.Engine.UnregisterTarget(req.Namespace, req.Name)
			}

			return ctrl.Result{}, nil
		}

		retErr = err

		return ctrl.Result{}, err
	}

	if target.Spec.Suspend {
		log.Info("target suspended", "name", target.Name, "namespace", target.Namespace)
		if r.Engine != nil {
			r.Engine.UnregisterTarget(target.Namespace, target.Name)
		}

		if err := r.setDegraded(ctx, &target, "Suspended", "spec.suspend is true"); err != nil {
			retErr = err
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if target.Spec.ProfileRef == "" {
		if err := r.setDegraded(ctx, &target, "MissingProfileRef", "spec.profileRef is required"); err != nil {
			retErr = err
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	var profile kollectdevv1alpha1.KollectProfile
	if err := r.Get(ctx, client.ObjectKey{Name: target.Spec.ProfileRef}, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			if degErr := r.setDegraded(ctx, &target, "ProfileNotFound",
				fmt.Sprintf("KollectProfile %q not found", target.Spec.ProfileRef)); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}

		retErr = err

		return ctrl.Result{}, err
	}

	if r.Engine != nil {
		if err := r.Engine.RegisterTarget(ctx, &target, &profile); err != nil {
			if degErr := r.setDegraded(ctx, &target, "InformerRegistrationFailed", err.Error()); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}
	}

	count := 0
	if r.Engine != nil {
		count = r.Engine.ItemCount(target.Namespace, target.Name)
		if r.Engine.HasForbiddenScope(target.Namespace, target.Name) {
			if degErr := r.setDegraded(ctx, &target, "Forbidden",
				"RBAC denied list access for one or more scoped namespaces; partial collection skipped"); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}
	}

	return r.setReady(ctx, &target, count)
}

func (r *KollectTargetReconciler) setDegraded(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	reason, message string,
) error {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionReady)
	return setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionDegraded, metav1.ConditionTrue, reason, message,
	)
}

func (r *KollectTargetReconciler) setReady(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	collected int,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionDegraded)
	target.Status.ObservedGeneration = target.Generation

	msg := fmt.Sprintf("profileRef %q resolved; collecting %d resource(s)",
		target.Spec.ProfileRef, collected)
	if err := setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionReady, metav1.ConditionTrue, "Collecting",
		msg,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultCollectRequeue}, nil
}

const defaultCollectRequeue = 30 * time.Second

// SetupWithManager sets up the controller with the Manager.
func (r *KollectTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentTarget)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentTarget
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectTarget{}).
		WithOptions(opts).
		Named("kollecttarget").
		Complete(r)
}
