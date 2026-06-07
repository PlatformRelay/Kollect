// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func boolPtr(b bool) *bool { return &b }

func sampleDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":            "web",
			"namespace":       "team-a",
			"uid":             "abc-123",
			"resourceVersion": "9999",
			"generation":      int64(7),
			"managedFields":   []any{map[string]any{"manager": "kubectl"}},
			"labels": map[string]any{
				"app":               "web",
				"pod-template-hash": "deadbeef",
			},
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "{}",
				"team": "platform",
			},
		},
		"spec": map[string]any{
			"replicas": int64(3),
			"template": map[string]any{
				"spec": map[string]any{
					"env": map[string]any{
						"password": "hunter2",
						"host":     "db",
					},
				},
			},
		},
		"status": map[string]any{
			"readyReplicas": int64(3),
			"conditions":    []any{map[string]any{"type": "Available"}},
		},
	}}
}

func TestPruneResource_includeSpecAndStatusDropsMetadata(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource}
	got := PruneResource(sampleDeployment(), export, nil)

	if _, ok := got["metadata"]; ok {
		t.Fatalf("SpecAndStatus must drop metadata, got keys %v", keysOf(got))
	}
	for _, want := range []string{"apiVersion", "kind", "spec", "status"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("missing %q in pruned object: %v", want, keysOf(got))
		}
	}
}

func TestPruneResource_includeModes(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		kollectdevv1alpha1.ExportIncludeSpecOnly:   {"apiVersion", "kind", "spec"},
		kollectdevv1alpha1.ExportIncludeStatusOnly: {"apiVersion", "kind", "status"},
		kollectdevv1alpha1.ExportIncludeMetadataOnly: {
			"apiVersion", "kind", "metadata",
		},
	}

	for include, wantKeys := range cases {
		t.Run(include, func(t *testing.T) {
			t.Parallel()

			export := &kollectdevv1alpha1.ExportSpec{
				Mode:    kollectdevv1alpha1.ExportModeResource,
				Include: include,
				Prune:   &kollectdevv1alpha1.PruneSpec{Defaults: boolPtr(false)},
			}
			got := PruneResource(sampleDeployment(), export, nil)

			if len(keysOf(got)) != len(wantKeys) {
				t.Fatalf("include %s: got keys %v want %v", include, keysOf(got), wantKeys)
			}
			for _, k := range wantKeys {
				if _, ok := got[k]; !ok {
					t.Fatalf("include %s: missing key %q (%v)", include, k, keysOf(got))
				}
			}
		})
	}
}

func TestPruneResource_includeAllKeepsEverything(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
		Prune:   &kollectdevv1alpha1.PruneSpec{Defaults: boolPtr(false)},
	}
	got := PruneResource(sampleDeployment(), export, nil)

	meta, ok := got["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("All must keep metadata")
	}
	if _, ok := meta["resourceVersion"]; !ok {
		t.Fatalf("defaults disabled: resourceVersion should remain")
	}
}

func TestPruneResource_defaultsRemoveNoise(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:           kollectdevv1alpha1.ExportModeResource,
		Include:        kollectdevv1alpha1.ExportIncludeAll,
		DedupeIdentity: boolPtr(false),
	}
	got := PruneResource(sampleDeployment(), export, nil)

	meta := got["metadata"].(map[string]any)
	for _, gone := range []string{"resourceVersion", "generation", "managedFields"} {
		if _, ok := meta[gone]; ok {
			t.Fatalf("default prune should remove metadata.%s", gone)
		}
	}

	annotations := meta["annotations"].(map[string]any)
	if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Fatalf("default prune should remove last-applied-configuration annotation")
	}
	if _, ok := annotations["team"]; !ok {
		t.Fatalf("default prune must not remove unrelated annotations")
	}
}

func TestPruneResource_userJSONPointers(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
		Prune: &kollectdevv1alpha1.PruneSpec{
			Defaults:     boolPtr(false),
			JSONPointers: []string{"/status/conditions", "/metadata/annotations/team"},
		},
	}
	got := PruneResource(sampleDeployment(), export, nil)

	status := got["status"].(map[string]any)
	if _, ok := status["conditions"]; ok {
		t.Fatalf("jsonPointer /status/conditions not removed")
	}

	annotations := got["metadata"].(map[string]any)["annotations"].(map[string]any)
	if _, ok := annotations["team"]; ok {
		t.Fatalf("jsonPointer /metadata/annotations/team not removed")
	}
}

func TestPruneResource_jsonPathRemoval(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
		Prune: &kollectdevv1alpha1.PruneSpec{
			Defaults:  boolPtr(false),
			JSONPaths: []string{`$.metadata.labels["pod-template-hash"]`},
		},
	}
	got := PruneResource(sampleDeployment(), export, nil)

	labels := got["metadata"].(map[string]any)["labels"].(map[string]any)
	if _, ok := labels["pod-template-hash"]; ok {
		t.Fatalf("jsonPath did not remove pod-template-hash label")
	}
	if _, ok := labels["app"]; !ok {
		t.Fatalf("jsonPath removed unrelated label")
	}
}

func TestPruneResource_dedupeIdentity(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeMetadataOnly,
		Prune:   &kollectdevv1alpha1.PruneSpec{Defaults: boolPtr(false)},
	}
	got := PruneResource(sampleDeployment(), export, nil)

	meta := got["metadata"].(map[string]any)
	for _, gone := range []string{"name", "namespace", "uid"} {
		if _, ok := meta[gone]; ok {
			t.Fatalf("dedupeIdentity should remove metadata.%s", gone)
		}
	}
	if got["apiVersion"] != "apps/v1" {
		t.Fatalf("dedupeIdentity must keep apiVersion for self-description")
	}
}

func TestPruneResource_scrubMerged(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeSpecOnly,
		Prune:   &kollectdevv1alpha1.PruneSpec{Defaults: boolPtr(false)},
	}
	got := PruneResource(sampleDeployment(), export, NewScrubber(nil))

	env := got["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["env"].(map[string]any)
	red, ok := env["password"].(map[string]any)
	if v, okBool := red["redacted"].(bool); !ok || !okBool || !v {
		t.Fatalf("scrubber should redact spec env password, got %v", env["password"])
	}
	if env["host"] != "db" {
		t.Fatalf("scrubber removed non-sensitive value")
	}
}

func TestPruneResource_doesNotMutateSource(t *testing.T) {
	t.Parallel()

	src := sampleDeployment()
	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
	}
	_ = PruneResource(src, export, NewScrubber(nil))

	meta := src.Object["metadata"].(map[string]any)
	if _, ok := meta["managedFields"]; !ok {
		t.Fatalf("source object was mutated: managedFields removed from informer copy")
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

func TestJSONPathTokens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in     string
		want   []string
		wantOK bool
	}{
		{`$.metadata.labels["pod-template-hash"]`, []string{"metadata", "labels", "pod-template-hash"}, true},
		{`$.spec.replicas`, []string{"spec", "replicas"}, true},
		{`$.spec.containers[0]`, []string{"spec", "containers", "0"}, true},
		{`$.spec.containers[*]`, nil, false},
		{`$.spec.containers[?(@.name=="x")]`, nil, false},
	}

	for _, tc := range cases {
		got, ok := jsonPathTokens(tc.in)
		if ok != tc.wantOK {
			t.Fatalf("%q ok=%v want %v", tc.in, ok, tc.wantOK)
		}
		if tc.wantOK && !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%q tokens=%v want %v", tc.in, got, tc.want)
		}
	}
}
