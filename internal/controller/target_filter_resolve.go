// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/scope"
)

func scopeCeilingForTarget(ctx context.Context, c client.Client, tenantNS string) collect.ScopeCeiling {
	binding, err := scope.Load(ctx, c, tenantNS)
	if err != nil || !binding.Enforced {
		return collect.ScopeCeiling{}
	}

	return collect.ScopeCeilingFromScope(binding.Scope)
}

func resolveTargetFilterStatus(
	ctx context.Context,
	c client.Client,
	engine *collect.Engine,
	target *kollectdevv1alpha1.KollectTarget,
) (matched, effective []string, activeRules int, ceiling collect.ScopeCeiling) {
	ceiling = scopeCeilingForTarget(ctx, c, target.Namespace)

	var nsMeta map[string]collect.NamespaceMeta
	defaults := collect.NamespaceDefaults{}
	if engine != nil {
		// Read-after-refresh (COLLECT-NS-BACKFILL): the engine's namespace snapshot is
		// written only by RegisterTarget, which the reconciler calls *after* this resolve.
		// Reading it unrefreshed leaves the resolve one registration behind, so a target
		// created in a fresh namespace resolves an effective set that omits its own
		// namespace — and the engine then silently drops every object in it.
		if err := engine.RefreshNamespaces(ctx); err != nil {
			logf.FromContext(ctx).Error(err, "refresh namespace cache before filter resolve")
		}

		nsMeta = engine.NamespaceMetaSnapshot()
		defaults = engine.NamespaceDefaultsSnapshot()
	} else {
		nsMeta = listNamespaceMeta(ctx, c)
	}

	matched, effective, activeRules = collect.ComputeFilterStatus(
		target.Spec.CollectionFilterSpec,
		target.Spec.NamespaceSelector,
		nsMeta,
		ceiling,
		defaults,
	)

	return matched, effective, activeRules, ceiling
}

func listNamespaceMeta(ctx context.Context, c client.Client) map[string]collect.NamespaceMeta {
	var nsList corev1.NamespaceList
	if err := c.List(ctx, &nsList); err != nil {
		return nil
	}

	meta := make(map[string]collect.NamespaceMeta, len(nsList.Items))
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		meta[ns.Name] = collect.NamespaceMeta{
			Labels:      labels.Set(ns.Labels),
			Annotations: ns.Annotations,
		}
	}

	return meta
}
