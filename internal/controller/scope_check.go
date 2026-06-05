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

	if err := scope.ValidateTargetGVK(binding.Scope, profile.Spec.TargetGVK); err != nil {
		recordWarning(s.recorder, target, scopeReasonGVKDenied, err.Error())
		return false, scopeReasonGVKDenied, err.Error()
	}

	var workloadNS []string
	if s.engine != nil {
		workloadNS = s.engine.MatchedNamespacesForTarget(target.Namespace, target.Name)
	}
	if len(workloadNS) == 0 {
		workloadNS = []string{target.Namespace}
	}

	if err := scope.ValidateWorkloadNamespaces(binding.Scope, workloadNS); err != nil {
		recordWarning(s.recorder, target, scopeReasonNSDenied, err.Error())
		return false, scopeReasonNSDenied, err.Error()
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

	if err := scope.ValidateSinkRefs(binding.Scope, inv.Spec.SinkRefs); err != nil {
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
		return false, "SinkNotFound", fmt.Sprintf("KollectSink %q not found", sinkName)
	}

	verified := apimeta.FindStatusCondition(ks.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if verified != nil && verified.Status == metav1.ConditionFalse {
		msg := "sink connection not verified"
		if verified.Message != "" {
			msg = verified.Message
		}

		return false, "SinkUnreachable", msg
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
