// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/tools/cache"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestEngineMatchesTarget(t *testing.T) {
	t.Parallel()

	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	target := kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			WatchMode: kollectdevv1alpha1.WatchModeAll,
		},
	}
	rules, err := CompileResourceRules(nil, ext.celEnv)
	if err != nil {
		t.Fatal(err)
	}

	e := &Engine{
		store: NewStore(),
		nsMeta: map[string]namespaceMeta{
			"team-a": {Labels: labels.Set{kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueEnabled}},
		},
		targets: make(map[string]targetState),
	}
	st := targetState{
		target:              target,
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
			"labels": map[string]any{kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueEnabled},
		},
	}}
	if got := e.matchesTarget(context.Background(), st, gvr, obj); got != targetMatchAccepted {
		t.Fatalf("match = %v, want accepted", got)
	}

	wrongNS := obj.DeepCopy()
	wrongNS.SetNamespace("team-b")
	if got := e.matchesTarget(context.Background(), st, gvr, wrongNS); got != targetMatchNamespaceMismatch {
		t.Fatalf("match = %v, want namespace mismatch", got)
	}

	optOut := obj.DeepCopy()
	optOut.SetLabels(map[string]string{kollectdevv1alpha1.LabelWatch: kollectdevv1alpha1.WatchValueDisabled})
	if got := e.matchesTarget(context.Background(), st, gvr, optOut); got != targetMatchFiltered {
		t.Fatalf("match = %v, want filtered", got)
	}

	nameFiltered := targetState{
		target: kollectdevv1alpha1.KollectTarget{
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				Names: []string{"api"},
			},
		},
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	if got := e.matchesTarget(context.Background(), nameFiltered, gvr, obj); got != targetMatchFiltered {
		t.Fatalf("match = %v, want filtered", got)
	}
}

func TestEngineDispatchDeleteAndUpsert(t *testing.T) {
	t.Parallel()

	store := NewStore()
	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := CompileResourceRules(nil, ext.celEnv)
	if err != nil {
		t.Fatal(err)
	}

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	target := kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "deploys"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
	}

	gvr := gvrFromProfile(profile.Spec.TargetGVK)
	key := targetKey("team-a", "deploys")

	e := &Engine{
		store:        store,
		extractor:    ext,
		access:       &AccessChecker{},
		nsMeta:       map[string]namespaceMeta{"team-a": {}},
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
	}
	e.targets[key] = targetState{
		target:              target,
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	e.targetsByGVR[gvr] = []string{key}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
		},
	}}

	e.processDispatch(context.Background(), gvr, obj, false)
	if store.CountForTarget("team-a", "deploys") != 1 {
		t.Fatalf("count = %d", store.CountForTarget("team-a", "deploys"))
	}

	tombstone := cache.DeletedFinalStateUnknown{Obj: obj}
	e.processDispatch(context.Background(), gvr, tombstone, true)
	if store.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("count after tombstone delete = %d", store.CountForTarget("team-a", "deploys"))
	}
}
