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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// KollectTargetReconciler reconciles a KollectTarget object
type KollectTargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Engine   *collect.Engine
	Options  RuntimeOptions
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectprofiles,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectscopes,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch

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

	profileKey := client.ObjectKey{Namespace: target.Namespace, Name: target.Spec.ProfileRef}
	var profile kollectdevv1alpha1.KollectProfile
	if err := r.Get(ctx, profileKey, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			if degErr := r.setDegraded(ctx, &target, "ProfileNotFound",
				fmt.Sprintf("KollectProfile %q not found in namespace %q",
					target.Spec.ProfileRef, target.Namespace)); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}

		retErr = err

		return ctrl.Result{}, err
	}

	checker := scopeCheck{client: r.Client, recorder: r.Recorder, engine: r.Engine}
	if ok, reason, msg := checker.enforceTarget(ctx, &target, &profile); !ok {
		if degErr := r.setDegraded(ctx, &target, reason, msg); degErr != nil {
			retErr = degErr
			return ctrl.Result{}, degErr
		}

		return ctrl.Result{}, nil
	}

	matched, effective, activeRules, ceiling := resolveTargetFilterStatus(ctx, r.Client, r.Engine, &target)
	updateTargetFilterStatus(&target, matched, effective, activeRules)

	if r.Engine != nil {
		if err := r.Engine.RegisterTarget(ctx, &target, &profile, collect.RegisterTargetOptions{
			ScopeCeiling:        ceiling,
			EffectiveNamespaces: effective,
		}); err != nil {
			if degErr := r.setDegraded(ctx, &target, "InformerRegistrationFailed", err.Error()); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}
	}

	result, err := r.reconcileTargetReady(ctx, &target)
	if err != nil {
		retErr = err
	}

	return result, err
}

func (r *KollectTargetReconciler) reconcileTargetReady(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (ctrl.Result, error) {
	count := 0
	if r.Engine != nil {
		count = r.Engine.ItemCount(target.Namespace, target.Name)
		if r.Engine.HasForbiddenScope(target.Namespace, target.Name) {
			if err := r.setDegraded(ctx, target, "Forbidden",
				"RBAC denied list access for one or more scoped namespaces; partial collection skipped"); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
	}

	sinkOK, sinkReason, sinkMsg := checkTargetNamespaceSinksReachable(ctx, r.Client, target.Namespace)
	if !sinkOK {
		recordWarning(r.Recorder, target, sinkReason, sinkMsg)
		if err := r.setDegraded(ctx, target, sinkReason, sinkMsg); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return r.setReady(ctx, target, count, sinkReason, sinkMsg)
}

func (r *KollectTargetReconciler) setDegraded(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	reason, message string,
) error {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionReady)
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionSynced)
	setSinkReachableCondition(&target.Status.Conditions, target.Generation, false, reason, message)
	return setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionDegraded, metav1.ConditionTrue, reason, message,
	)
}

func (r *KollectTargetReconciler) setReady(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	collected int,
	sinkReason, sinkMsg string,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&target.Status.Conditions, conditionDegraded)
	target.Status.ObservedGeneration = target.Generation
	updateTargetFilterStatus(
		target, target.Status.MatchedNamespaces, target.Status.EffectiveNamespaces, target.Status.ActiveResourceRules,
	)

	msg := fmt.Sprintf("profileRef %q resolved; collecting %d resource(s)",
		target.Spec.ProfileRef, collected)
	if sinkMsg == "" {
		sinkMsg = "namespace inventory sinks reachable"
	}
	setSinkReachableCondition(&target.Status.Conditions, target.Generation, true, sinkReason, sinkMsg)
	recordNormal(r.Recorder, target, sinkReason, sinkMsg)
	setSyncedCondition(&target.Status.Conditions, target.Generation, true, "Collecting", msg)
	if err := setTargetCondition(
		ctx, r.Client, target, target.Generation, &target.Status.Conditions,
		conditionReady, metav1.ConditionTrue, "Collecting",
		msg,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentTarget)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentTarget
	}

	if r.Recorder == nil {
		//nolint:staticcheck // SA1019: record API until events migration
		r.Recorder = mgr.GetEventRecorderFor("kollecttarget-controller")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectTarget{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectProfile{},
			handler.EnqueueRequestsFromMapFunc(r.mapProfileToTargets),
		).
		Named("kollecttarget").
		Complete(r)
}

func (r *KollectTargetReconciler) mapProfileToTargets(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	profile, ok := obj.(*kollectdevv1alpha1.KollectProfile)
	if !ok {
		return nil
	}

	var list kollectdevv1alpha1.KollectTargetList
	if err := r.List(ctx, &list, client.InNamespace(profile.Namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list targets for profile watch mapping",
			"profile", profile.Name, "namespace", profile.Namespace)

		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		if list.Items[i].Spec.ProfileRef == profile.Name {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
			})
		}
	}

	return reqs
}
