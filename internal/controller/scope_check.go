// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if err := scope.ValidateSinkRefs(binding.Scope, inv.Spec.SinkRefs.Names()); err != nil {
		recordWarning(s.recorder, inv, scopeReasonSinkDenied, err.Error())
		return false, scopeReasonSinkDenied, err.Error()
	}

	return true, "", ""
}

func (s scopeCheck) sinkReachable(
	ctx context.Context,
	namespace, sinkName string,
) (bool, string, string) {
	var ks kollectdevv1alpha1.KollectSink
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: sinkName}, &ks); err != nil {
		return false, reasonSinkNotFound, fmt.Sprintf("KollectSink %q not found", sinkName)
	}

	verified := apimeta.FindStatusCondition(ks.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if verified != nil && verified.Status == metav1.ConditionFalse {
		msg := "sink connection not verified"
		if verified.Message != "" {
			msg = verified.Message
		}

		return false, reasonSinkUnreachable, msg
	}

	if verified != nil && verified.Status == metav1.ConditionTrue {
		msg := fmt.Sprintf("KollectSink %q connection verified", sinkName)
		if verified.Message != "" {
			msg = verified.Message
		}

		return true, "ConnectionVerified", msg
	}

	return true, "SinkResolved", fmt.Sprintf("KollectSink %q found", sinkName)
}
