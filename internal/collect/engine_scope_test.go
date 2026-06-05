// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestEngineUnregisterAndForbiddenScope(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.Upsert(Item{TargetNamespace: "team-a", TargetName: "deploys", UID: "1", Namespace: "apps", Name: "web"})

	e := &Engine{store: store, targets: make(map[string]targetState), forbidden: make(map[string]struct{})}
	e.targets[targetKey("team-a", "deploys")] = targetState{
		target: kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a"},
		},
	}
	e.forbidden[targetKey("team-a", "deploys")] = struct{}{}

	if !e.HasForbiddenScope("team-a", "deploys") {
		t.Fatal("expected forbidden scope")
	}

	e.UnregisterTarget("team-a", "deploys")
	if e.ItemCount("team-a", "deploys") != 0 {
		t.Fatalf("item count = %d", e.ItemCount("team-a", "deploys"))
	}
}

func TestEngineMatchedNamespacesForTarget(t *testing.T) {
	t.Parallel()

	e := &Engine{
		targets: map[string]targetState{
			targetKey("team-a", "deploys"): {
				target: kollectdevv1alpha1.KollectTarget{
					Spec: kollectdevv1alpha1.KollectTargetSpec{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"tenant": "a"},
						},
					},
				},
			},
		},
		nsMeta: map[string]namespaceMeta{
			"apps":  {Labels: map[string]string{"tenant": "a"}},
			"other": {Labels: map[string]string{"tenant": "b"}},
		},
	}

	matched := e.MatchedNamespacesForTarget("team-a", "deploys")
	if len(matched) != 1 || matched[0] != "apps" {
		t.Fatalf("matched = %#v", matched)
	}
}

func TestEngineRegisterSuspendedTargetUnregisters(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})

	e := &Engine{
		store:     store,
		targets:   make(map[string]targetState),
		forbidden: make(map[string]struct{}),
	}

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "deploys"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{Suspend: true},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	if err := e.RegisterTarget(context.Background(), target, profile); err != nil {
		t.Fatal(err)
	}
	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0 after suspended register", store.TotalCount())
	}
}
