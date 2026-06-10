// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/scope"
)

// KollectClusterTargetReconciler wires cluster-scoped targets to the collection engine per
// namespace matched by spec.namespaceSelector.
type KollectClusterTargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Engine   *collect.Engine
	Options  RuntimeOptions
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectprofiles,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile registers synthetic per-namespace targets in the collection engine.
func (r *KollectClusterTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectclustertarget")
	var retErr error
	defer func() { finish(retErr) }()

	var ct kollectdevv1alpha1.KollectClusterTarget
	if err := r.Get(ctx, req.NamespacedName, &ct); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		retErr = err

		return ctrl.Result{}, err
	}

	return guardReconcile(ctx, r.Recorder, &ct, func() (ctrl.Result, error) {
		if !ct.DeletionTimestamp.IsZero() {
			result, err := r.finalizeClusterTargetDeletion(ctx, &ct)
			if err != nil {
				retErr = err
			}

			return result, err
		}

		if err := r.ensureClusterTargetFinalizer(ctx, &ct); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}

			retErr = err

			return ctrl.Result{}, err
		}

		if ct.Spec.Suspend {
			r.unregisterAll(&ct)
			if err := r.setDegraded(ctx, &ct, "Suspended", "spec.suspend is true"); err != nil {
				retErr = err
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		profile, err := resolveClusterTargetProfile(ctx, r.Client, ct.Spec.ProfileRef)
		recordStaticRefResolution("KollectClusterTarget", metrics.StaticRefTypeProfile, err)
		if err != nil {
			r.unregisterAll(&ct)
			reason := reasonProfileNotFound
			if apierrors.IsForbidden(err) {
				reason = reasonProfileForbidden
				recordWarning(r.Recorder, &ct, reason, err.Error())
			}
			if degErr := r.setDegraded(ctx, &ct, reason, err.Error()); degErr != nil {
				retErr = degErr
				return ctrl.Result{}, degErr
			}

			return ctrl.Result{}, nil
		}

		matched, err := r.matchedNamespaces(ctx, &ct)
		if err != nil {
			retErr = err
			return ctrl.Result{}, err
		}

		clusterBinding, err := scope.LoadCluster(ctx, r.Client)
		if err != nil {
			retErr = err
			return ctrl.Result{}, err
		}

		ceiling := collect.ScopeCeiling{}
		if clusterBinding.Enforced {
			ceiling = collect.ScopeCeilingFromClusterScope(clusterBinding.Scope)
		}

		nsMeta := listNamespaceMeta(ctx, r.Client)
		defaults := collect.NamespaceDefaults{}
		if r.Engine != nil {
			defaults = r.Engine.NamespaceDefaultsSnapshot()
		}

		_, effective, activeRules := collect.ComputeFilterStatus(
			ct.Spec.CollectionFilterSpec,
			ct.Spec.NamespaceSelector,
			nsMeta,
			ceiling,
			defaults,
		)
		updateClusterTargetFilterStatus(&ct, matched, effective, activeRules)

		if r.Engine != nil {
			if err := r.syncEngineTargets(ctx, &ct, profile, effective, ceiling); err != nil {
				if degErr := r.setDegraded(ctx, &ct, "InformerRegistrationFailed", err.Error()); degErr != nil {
					retErr = degErr
					return ctrl.Result{}, degErr
				}

				return ctrl.Result{}, nil
			}
		}

		if err := r.setReady(ctx, &ct, effective); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	})
}

func (r *KollectClusterTargetReconciler) matchedNamespaces(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) ([]string, error) {
	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	meta := make(map[string]collect.NamespaceMeta, len(nsList.Items))
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		meta[ns.Name] = collect.NamespaceMeta{Labels: labels.Set(ns.Labels)}
	}

	defaults := collect.NamespaceDefaults{}
	if r.Engine != nil {
		defaults = r.Engine.NamespaceDefaultsSnapshot()
	}

	matched := collect.MatchIntentNamespaces(
		ct.Spec.CollectionFilterSpec,
		ct.Spec.NamespaceSelector,
		meta,
		defaults,
	)

	return matched, nil
}

func (r *KollectClusterTargetReconciler) syncEngineTargets(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
	profile *kollectdevv1alpha1.KollectProfile,
	effective []string,
	ceiling collect.ScopeCeiling,
) error {
	want := make(map[string]struct{}, len(effective))
	for _, ns := range effective {
		want[ns] = struct{}{}

		synthetic := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ct.Name,
				Namespace: ns,
			},
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				ProfileRef:           ct.Spec.ProfileRef.Name,
				CollectionFilterSpec: ct.Spec.CollectionFilterSpec,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						corev1.LabelMetadataName: ns,
					},
				},
			},
		}

		if err := r.Engine.RegisterTarget(ctx, synthetic, profile, collect.RegisterTargetOptions{
			ScopeCeiling:        ceiling,
			EffectiveNamespaces: []string{ns},
		}); err != nil {
			return err
		}
	}

	for _, ns := range r.registeredNamespaces(ct.Name) {
		if _, ok := want[ns]; ok {
			continue
		}

		r.Engine.UnregisterTarget(ns, ct.Name)
	}

	return nil
}

func (r *KollectClusterTargetReconciler) registeredNamespaces(clusterTargetName string) []string {
	if r.Engine == nil {
		return nil
	}

	return r.Engine.NamespacesForClusterTarget(clusterTargetName)
}

func (r *KollectClusterTargetReconciler) unregisterAll(ct *kollectdevv1alpha1.KollectClusterTarget) {
	if r.Engine == nil {
		return
	}

	for _, ns := range r.registeredNamespaces(ct.Name) {
		r.Engine.UnregisterTarget(ns, ct.Name)
	}
}

func (r *KollectClusterTargetReconciler) collectedCount(
	ct *kollectdevv1alpha1.KollectClusterTarget,
	matched []string,
) int {
	if r.Engine == nil {
		return 0
	}

	total := 0
	for _, ns := range matched {
		total += r.Engine.ItemCount(ns, ct.Name)
	}

	return total
}

func (r *KollectClusterTargetReconciler) setDegraded(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
	reason, message string,
) error {
	apimeta.RemoveStatusCondition(&ct.Status.Conditions, conditionReady)
	apimeta.RemoveStatusCondition(&ct.Status.Conditions, conditionSynced)
	return setClusterTargetCondition(
		ctx, r.Client, ct, ct.Generation, &ct.Status.Conditions,
		conditionDegraded, metav1.ConditionTrue, reason, message,
	)
}

func (r *KollectClusterTargetReconciler) setReady(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
	matched []string,
) error {
	count := r.collectedCount(ct, matched)
	msg := fmt.Sprintf(
		"profileRef %q in namespace %q resolved; %d namespace(s) matched; collecting %d resource(s)",
		ct.Spec.ProfileRef.Name, ct.Spec.ProfileRef.Namespace, len(matched), count,
	)

	apimeta.RemoveStatusCondition(&ct.Status.Conditions, conditionDegraded)
	ct.Status.ObservedGeneration = ct.Generation
	updateClusterTargetFilterStatus(
		ct, ct.Status.MatchedNamespaces, ct.Status.EffectiveNamespaces, ct.Status.ActiveResourceRules,
	)
	setSyncedCondition(&ct.Status.Conditions, ct.Generation, true, "Collecting", msg)

	return setClusterTargetCondition(
		ctx, r.Client, ct, ct.Generation, &ct.Status.Conditions,
		conditionReady, metav1.ConditionTrue, "Collecting", msg,
	)
}

func setClusterTargetCondition(
	ctx context.Context,
	c client.Client,
	ct *kollectdevv1alpha1.KollectClusterTarget,
	generation int64,
	conditions *[]metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return c.Status().Update(ctx, ct)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectClusterTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentClusterTarget)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentClusterTarget
	}

	if r.Recorder == nil {
		//nolint:staticcheck // SA1019: record API until events migration
		r.Recorder = mgr.GetEventRecorderFor("kollectclustertarget-controller")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectClusterTarget{}).
		WithOptions(opts).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToClusterTargets),
		).
		Watches(
			&kollectdevv1alpha1.KollectProfile{},
			handler.EnqueueRequestsFromMapFunc(r.mapProfileToClusterTargets),
		).
		Named("kollectclustertarget").
		Complete(r)
}

func (r *KollectClusterTargetReconciler) mapNamespaceToClusterTargets(
	ctx context.Context,
	_ client.Object,
) []reconcile.Request {
	var list kollectdevv1alpha1.KollectClusterTargetList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: list.Items[i].Name},
		})
	}

	return reqs
}

func (r *KollectClusterTargetReconciler) mapProfileToClusterTargets(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	profile, ok := obj.(*kollectdevv1alpha1.KollectProfile)
	if !ok {
		return nil
	}

	var list kollectdevv1alpha1.KollectClusterTargetList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		ref := list.Items[i].Spec.ProfileRef
		if ref.Name == profile.Name && ref.Namespace == profile.Namespace {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: list.Items[i].Name},
			})
		}
	}

	return reqs
}
