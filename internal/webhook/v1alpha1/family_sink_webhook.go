// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/validation"
)

func setupFamilySinkWebhooks(mgr ctrl.Manager) error {
	hooks := []func(ctrl.Manager) error{
		setupKollectSnapshotSinkWebhook,
		setupKollectDatabaseSinkWebhook,
		setupKollectEventSinkWebhook,
	}
	for _, hook := range hooks {
		if err := hook(mgr); err != nil {
			return err
		}
	}
	return nil
}

type kollectSnapshotSinkValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectSnapshotSink]
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectSnapshotSink] = &kollectSnapshotSinkValidator{}

//nolint:lll
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectsnapshotsink,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectsnapshotsinks,verbs=create;update,versions=v1alpha1,name=vkollectsnapshotsink.kb.io,admissionReviewVersions=v1

func setupKollectSnapshotSinkWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectSnapshotSink{}).
		WithValidator(&kollectSnapshotSinkValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectSnapshotSinkValidator) ValidateCreate(ctx context.Context, obj *kollectdevv1alpha1.KollectSnapshotSink) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}
func (v *kollectSnapshotSinkValidator) ValidateUpdate(ctx context.Context, _ *kollectdevv1alpha1.KollectSnapshotSink, obj *kollectdevv1alpha1.KollectSnapshotSink) (admission.Warnings, error) {
	if obj.DeletionTimestamp != nil {
		return nil, nil
	}
	return v.validate(ctx, obj)
}
func (v *kollectSnapshotSinkValidator) validate(ctx context.Context, obj *kollectdevv1alpha1.KollectSnapshotSink) (admission.Warnings, error) {
	errs := validation.ValidateSnapshotSinkSpec(&obj.Spec)
	if len(errs) > 0 {
		return nil, validation.SnapshotSinkInvalid(obj.Name, errs)
	}
	if err := validateNamespacedSinkScopeFloor(ctx, v.client, obj.Namespace, &obj.Spec.SinkCommonFields, validation.SnapshotSinkInvalid, obj.Name); err != nil {
		return nil, err
	}
	normalized := obj.Spec.ToKollectSinkSpec()
	warns := validation.ValidateGitSinkWarnings(&kollectdevv1alpha1.KollectSinkSpec{Type: obj.Spec.Type, Git: obj.Spec.Git})
	warns = append(warns, validation.ValidateSinkConfigWarnings(&normalized)...)
	return warns, nil
}

type kollectDatabaseSinkValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectDatabaseSink]
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectDatabaseSink] = &kollectDatabaseSinkValidator{}

//nolint:lll
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectdatabasesink,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectdatabasesinks,verbs=create;update,versions=v1alpha1,name=vkollectdatabasesink.kb.io,admissionReviewVersions=v1

func setupKollectDatabaseSinkWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectDatabaseSink{}).
		WithValidator(&kollectDatabaseSinkValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectDatabaseSinkValidator) ValidateCreate(ctx context.Context, obj *kollectdevv1alpha1.KollectDatabaseSink) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}
func (v *kollectDatabaseSinkValidator) ValidateUpdate(ctx context.Context, _ *kollectdevv1alpha1.KollectDatabaseSink, obj *kollectdevv1alpha1.KollectDatabaseSink) (admission.Warnings, error) {
	if obj.DeletionTimestamp != nil {
		return nil, nil
	}
	return v.validate(ctx, obj)
}
func (v *kollectDatabaseSinkValidator) validate(ctx context.Context, obj *kollectdevv1alpha1.KollectDatabaseSink) (admission.Warnings, error) {
	errs := validation.ValidateDatabaseSinkSpec(&obj.Spec)
	if len(errs) > 0 {
		return nil, validation.DatabaseSinkInvalid(obj.Name, errs)
	}
	if err := validateNamespacedSinkScopeFloor(ctx, v.client, obj.Namespace, &obj.Spec.SinkCommonFields, validation.DatabaseSinkInvalid, obj.Name); err != nil {
		return nil, err
	}
	normalized := obj.Spec.ToKollectSinkSpec()
	return validation.ValidateSinkConfigWarnings(&normalized), nil
}

type kollectEventSinkValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectEventSink]
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectEventSink] = &kollectEventSinkValidator{}

//nolint:lll
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollecteventsink,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollecteventsinks,verbs=create;update,versions=v1alpha1,name=vkollecteventsink.kb.io,admissionReviewVersions=v1

func setupKollectEventSinkWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectEventSink{}).
		WithValidator(&kollectEventSinkValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectEventSinkValidator) ValidateCreate(ctx context.Context, obj *kollectdevv1alpha1.KollectEventSink) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}
func (v *kollectEventSinkValidator) ValidateUpdate(ctx context.Context, _ *kollectdevv1alpha1.KollectEventSink, obj *kollectdevv1alpha1.KollectEventSink) (admission.Warnings, error) {
	if obj.DeletionTimestamp != nil {
		return nil, nil
	}
	return v.validate(ctx, obj)
}
func (v *kollectEventSinkValidator) validate(ctx context.Context, obj *kollectdevv1alpha1.KollectEventSink) (admission.Warnings, error) {
	errs := validation.ValidateEventSinkSpec(&obj.Spec)
	if len(errs) > 0 {
		return nil, validation.EventSinkInvalid(obj.Name, errs)
	}
	if err := validateNamespacedSinkScopeFloor(ctx, v.client, obj.Namespace, &obj.Spec.SinkCommonFields, validation.EventSinkInvalid, obj.Name); err != nil {
		return nil, err
	}
	normalized := obj.Spec.ToKollectSinkSpec()
	return validation.ValidateSinkConfigWarnings(&normalized), nil
}

type sinkInvalidFn func(string, field.ErrorList) error

func validateNamespacedSinkScopeFloor(
	ctx context.Context,
	c client.Client,
	namespace string,
	common *kollectdevv1alpha1.SinkCommonFields,
	invalid sinkInvalidFn,
	name string,
) error {
	binding, err := scope.Load(ctx, c, namespace)
	if err != nil {
		return invalid(name, validation.ScopeLoadErrors(err))
	}
	if binding.Enforced && binding.Scope != nil {
		floor := validation.ScopeMinExportInterval(binding.Scope)
		if common != nil {
			errs := validation.ValidateSinkIntervalAgainstScopeFloor(common.ExportMinInterval, floor)
			if len(errs) > 0 {
				return invalid(name, errs)
			}
		}
	}
	return nil
}
