// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"encoding/json"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func allowingClient() *fake.Clientset {
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			}, nil
		},
	)

	return client
}

func newResourceExportEngine(t *testing.T, profile kollectdevv1alpha1.KollectProfile) (*Engine, *Store) {
	t.Helper()

	store := NewStore()
	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := CompileResourceRules(nil, ext.celEnv)
	if err != nil {
		t.Fatal(err)
	}

	gvr := gvrFromProfile(profile.Spec.TargetGVK)
	key := targetKey("team-a", "deploys")

	e := &Engine{
		store:        store,
		extractor:    ext,
		scrubber:     NewScrubber(nil),
		access:       NewAccessChecker(allowingClient()),
		forbidden:    make(map[string]struct{}),
		accessErr:    make(map[string]struct{}),
		nsMeta:       map[string]namespaceMeta{"team-a": {}},
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
	}
	e.targets[key] = targetState{
		target:              kollectdevv1alpha1.KollectTarget{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "deploys"}},
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	e.targetsByGVR[gvr] = []string{key}

	return e, store
}

func TestProcessDispatch_resourceExportPayloadShape(t *testing.T) {
	t.Parallel()

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Export: &kollectdevv1alpha1.ExportSpec{
				Mode: kollectdevv1alpha1.ExportModeResource,
				As:   "resource",
			},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "containerCount", Path: "cel:size(object.spec.template.spec.containers)", Type: "int"},
			},
		},
	}

	e, store := newResourceExportEngine(t, profile)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	obj := sampleDeployment()
	obj.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"] =
		[]any{map[string]any{"name": "web"}}

	e.processDispatch(context.Background(), gvr, obj, false)

	items := store.SnapshotTarget("team-a", "deploys")
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Namespace != "team-a" || item.Name != "web" || item.UID != "abc-123" {
		t.Fatalf("envelope identity mismatch: %+v", item)
	}
	if item.Kind != "Deployment" || item.Group != "apps" || item.Version != "v1" {
		t.Fatalf("envelope GVK mismatch: %+v", item)
	}

	blob, ok := item.Attributes["resource"].(map[string]any)
	if !ok {
		t.Fatalf("missing embedded resource attribute: %v", item.Attributes)
	}
	if _, ok := blob["spec"]; !ok {
		t.Fatalf("embedded resource missing spec")
	}
	if _, ok := blob["metadata"]; ok {
		t.Fatalf("SpecAndStatus default must drop metadata from embedded object")
	}
	if item.Attributes["containerCount"] == nil {
		t.Fatalf("hybrid attribute containerCount missing")
	}

	// Envelope round-trips through the export contract (ADR-0405).
	payload, err := MarshalExportEnvelope(items, ExportMetadata{})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	decoded, err := ItemsFromExportPayload(payload)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("round-trip item count = %d", len(decoded))
	}

	// Embedded blob must be JSON-serializable with stable (sorted) keys.
	if _, err := json.Marshal(decoded[0].Attributes["resource"]); err != nil {
		t.Fatalf("embedded resource not serializable: %v", err)
	}
}

func TestProcessDispatch_resourceExportMergesProfileScrubKeys(t *testing.T) {
	t.Parallel()

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Export: &kollectdevv1alpha1.ExportSpec{
				Mode:    kollectdevv1alpha1.ExportModeResource,
				Include: kollectdevv1alpha1.ExportIncludeSpecOnly,
				Prune:   &kollectdevv1alpha1.PruneSpec{ScrubKeys: []string{"host"}},
			},
		},
	}

	e, store := newResourceExportEngine(t, profile)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	e.processDispatch(context.Background(), gvr, sampleDeployment(), false)

	items := store.SnapshotTarget("team-a", "deploys")
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}

	env := items[0].Attributes["resource"].(map[string]any)["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["env"].(map[string]any)
	if red, ok := env["host"].(map[string]any); !ok {
		t.Fatalf("profile prune.scrubKeys should redact host, got %v", env["host"])
	} else if v, okBool := red["redacted"].(bool); !okBool || !v {
		t.Fatalf("profile prune.scrubKeys should redact host, got %v", env["host"])
	}
	if red, ok := env["password"].(map[string]any); !ok {
		t.Fatalf("built-in scrubKeys should still redact password, got %v", env["password"])
	} else if v, okBool := red["redacted"].(bool); !okBool || !v {
		t.Fatalf("built-in scrubKeys should still redact password, got %v", env["password"])
	}
}
