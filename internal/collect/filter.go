// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"k8s.io/apimachinery/pkg/labels"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type namespaceMeta struct {
	Labels      labels.Set
	Annotations map[string]string
}

// ShouldCollect reports whether a resource should be collected for the target after selector
// matching, based on watch opt-in/opt-out labels and namespace annotations (ADR-0029).
//
// Precedence: resource disabled > resource enabled (overrides namespace disabled) > namespace
// disabled > watchMode (All vs OptIn).
func ShouldCollect(
	resourceLabels labels.Set,
	ns namespaceMeta,
	target *kollectdevv1alpha1.KollectTarget,
) bool {
	if resourceLabels == nil {
		resourceLabels = labels.Set{}
	}

	watchMode := target.Spec.WatchMode
	if watchMode == "" {
		watchMode = kollectdevv1alpha1.WatchModeAll
	}

	if resourceLabels[kollectdevv1alpha1.LabelWatch] == kollectdevv1alpha1.WatchValueDisabled {
		return false
	}

	if resourceLabels[kollectdevv1alpha1.LabelWatch] == kollectdevv1alpha1.WatchValueEnabled {
		return true
	}

	nsLabels := ns.Labels
	if nsLabels == nil {
		nsLabels = labels.Set{}
	}

	nsAnnot := ns.Annotations
	if nsAnnot == nil {
		nsAnnot = map[string]string{}
	}

	nsDisabled := nsLabels[kollectdevv1alpha1.LabelWatch] == kollectdevv1alpha1.WatchValueDisabled ||
		nsAnnot[kollectdevv1alpha1.AnnotationNamespaceWatch] == kollectdevv1alpha1.WatchValueDisabled
	nsEnabled := nsLabels[kollectdevv1alpha1.LabelWatch] == kollectdevv1alpha1.WatchValueEnabled ||
		nsAnnot[kollectdevv1alpha1.AnnotationNamespaceWatch] == kollectdevv1alpha1.WatchValueEnabled

	if nsDisabled {
		return false
	}

	switch watchMode {
	case kollectdevv1alpha1.WatchModeOptIn:
		return nsEnabled
	default:
		return true
	}
}
