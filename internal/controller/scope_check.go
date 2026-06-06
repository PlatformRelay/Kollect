// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/scope"
)

const (
	scopeReasonMissingScope = "ScopeMissing"
	scopeReasonGVKDenied    = "ScopeGVKDenied"
	scopeReasonNSDenied     = "ScopeNamespaceDenied"
	scopeReasonSinkDenied   = "ScopeSinkDenied"
)

type scopeCheck struct {
	client   client.Client
	recorder record.EventRecorder
	engine   *collect.Engine
}

func (s scopeCheck) enforceTarget(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
) (bool, string, string) {
	binding, err := scope.Load(ctx, s.client, target.Namespace)
	if err != nil {
		return false, "ScopeLookupFailed", err.Error()
	}

	if !binding.Enforced {
		return true, "", ""
	}

	gvks := scope.CollectRuleGVKs(target.Spec.CollectionFilterSpec, profile.Spec.TargetGVK)
	if err := scope.ValidateResourceRuleGVKs(binding.Scope, gvks); err != nil {
		recordWarning(s.recorder, target, scopeReasonGVKDenied, err.Error())
		return false, scopeReasonGVKDenied, err.Error()
	}

	matched, effective, _, _ := resolveTargetFilterStatus(ctx, s.client, s.engine, target)
	intentNS := matched
	if len(intentNS) == 0 {
		intentNS = effective
	}
	if len(intentNS) == 0 {
		intentNS = []string{target.Namespace}
	}

	if err := scope.ValidateTargetIncludedNamespaces(binding.Scope, intentNS); err != nil {
		recordWarning(s.recorder, target, scopeReasonNSDenied, err.Error())
		return false, scopeReasonNSDenied, err.Error()
	}

	if len(effective) > 0 {
		if err := scope.ValidateWorkloadNamespaces(binding.Scope, effective); err != nil {
			recordWarning(s.recorder, target, scopeReasonNSDenied, err.Error())
			return false, scopeReasonNSDenied, err.Error()
		}
	}

	return true, "", ""
}

func (s scopeCheck) enforceInventory(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (bool, string, string) {
	binding, err := scope.Load(ctx, s.client, inv.Namespace)
	if err != nil {
		return false, "ScopeLookupFailed", err.Error()
	}

	if !binding.Enforced {
		return true, "", ""
	}

	if err := scope.ValidateInventoryFamilySinkRefs(binding.Scope, inventorySinkBindings(inv)); err != nil {
		recordWarning(s.recorder, inv, scopeReasonSinkDenied, err.Error())
		return false, scopeReasonSinkDenied, err.Error()
	}

	return true, "", ""
}
