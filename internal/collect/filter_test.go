// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func targetWithWatchMode(mode string) *kollectdevv1alpha1.KollectTarget {
	return &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			WatchMode: mode,
		},
	}
}

func TestShouldCollect_disabledWinsOverEnabled(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeAll)
	ns := namespaceMeta{
		Annotations: map[string]string{
			kollectdevv1alpha1.AnnotationNamespaceWatch: kollectdevv1alpha1.WatchValueEnabled,
		},
	}

	resourceLabels := labels.Set{
		kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueDisabled,
	}

	if ShouldCollect(resourceLabels, ns, target) {
		t.Fatal("resource disabled must skip even when namespace is enabled")
	}
}

func TestShouldCollect_resourceEnabledOverridesNamespaceDisabled(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeAll)
	ns := namespaceMeta{
		Annotations: map[string]string{
			kollectdevv1alpha1.AnnotationNamespaceWatch: kollectdevv1alpha1.WatchValueDisabled,
		},
	}
	resourceLabels := labels.Set{
		kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueEnabled,
	}

	if !ShouldCollect(resourceLabels, ns, target) {
		t.Fatal("resource enabled should override namespace disabled")
	}
}

func TestShouldCollect_namespaceDisabledSkipsAll(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeAll)
	ns := namespaceMeta{
		Annotations: map[string]string{
			kollectdevv1alpha1.AnnotationNamespaceWatch: kollectdevv1alpha1.WatchValueDisabled,
		},
	}

	if ShouldCollect(nil, ns, target) {
		t.Fatal("namespace disabled should skip resources without override label")
	}
}

func TestShouldCollect_allModeCollectsByDefault(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeAll)

	if !ShouldCollect(nil, namespaceMeta{}, target) {
		t.Fatal("All mode should collect when no opt-out signals are present")
	}

	if !ShouldCollect(nil, namespaceMeta{}, &kollectdevv1alpha1.KollectTarget{}) {
		t.Fatal("empty watchMode should default to All")
	}
}

func TestShouldCollect_optInRequiresEnabled(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeOptIn)

	if ShouldCollect(nil, namespaceMeta{}, target) {
		t.Fatal("OptIn mode should skip without explicit enable")
	}

	nsEnabled := namespaceMeta{
		Annotations: map[string]string{
			kollectdevv1alpha1.AnnotationNamespaceWatch: kollectdevv1alpha1.WatchValueEnabled,
		},
	}
	if !ShouldCollect(nil, nsEnabled, target) {
		t.Fatal("OptIn mode should collect when namespace annotation is enabled")
	}

	resourceEnabled := labels.Set{
		kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueEnabled,
	}
	if !ShouldCollect(resourceEnabled, namespaceMeta{}, target) {
		t.Fatal("OptIn mode should collect when resource label is enabled")
	}
}

func TestShouldCollect_optInNamespaceLabelEnabled(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeOptIn)
	ns := namespaceMeta{
		Labels: labels.Set{
			kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueEnabled,
		},
	}

	if !ShouldCollect(nil, ns, target) {
		t.Fatal("OptIn mode should collect when namespace label is enabled")
	}
}

func TestShouldCollect_namespaceLabelDisabled(t *testing.T) {
	t.Parallel()

	target := targetWithWatchMode(kollectdevv1alpha1.WatchModeAll)
	ns := namespaceMeta{
		Labels: labels.Set{
			kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueDisabled,
		},
	}

	if ShouldCollect(nil, ns, target) {
		t.Fatal("namespace label disabled should skip all resources")
	}
}
