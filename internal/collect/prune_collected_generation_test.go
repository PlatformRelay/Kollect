// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// collectedGenerationAnnotation is the key ERA-2 pins on the Resource-mode
// embedded copy. It stays a test-local constant until T06 adds the API
// constant: this task must not reference production symbols that do not exist
// yet (the red has to be a behaviour failure, not a compile error).
const collectedGenerationAnnotation = "kollect.dev/collectedGeneration"

func deploymentWithGeneration(gen int64) *unstructured.Unstructured {
	obj := sampleDeployment()
	meta := obj.Object["metadata"].(map[string]any)
	meta["generation"] = gen

	return obj
}

func embeddedAnnotationsOf(
	t *testing.T,
	root map[string]any,
) map[string]any {
	t.Helper()

	meta, ok := root["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("embedded copy has no metadata section: %v", keysOf(root))
	}

	annotations, ok := meta["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("embedded copy has no annotations map: %v", keysOf(meta))
	}

	return annotations
}

// TestPruneResource_stampsCollectedGeneration is the ERA-2 core claim: the
// embedded Resource-mode copy must carry kollect.dev/collectedGeneration set
// to the source object's metadata.generation at collection time.
func TestPruneResource_stampsCollectedGeneration(t *testing.T) {
	t.Parallel()

	obj := deploymentWithGeneration(42)
	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
	}

	got := PruneResource(obj, export, NewScrubber(nil))

	annotations := embeddedAnnotationsOf(t, got)
	if stamp := annotations[collectedGenerationAnnotation]; stamp != "42" {
		t.Fatalf("collectedGeneration stamp = %v, want \"42\" "+
			"(the source generation is not recorded on the embedded copy)", stamp)
	}
}

// TestPruneResource_stampSurvivesAnnotationPrune locks the ordering claim:
// the stamp is applied after profile pruning, so a prune path that removes
// metadata.annotations wholesale cannot erase it.
func TestPruneResource_stampSurvivesAnnotationPrune(t *testing.T) {
	t.Parallel()

	obj := deploymentWithGeneration(42)
	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
		Prune: &kollectdevv1alpha1.PruneSpec{
			Defaults:     boolPtr(false),
			JSONPointers: []string{"/metadata/annotations"},
		},
	}

	got := PruneResource(obj, export, NewScrubber(nil))

	annotations := embeddedAnnotationsOf(t, got)
	for key := range annotations {
		if key == collectedGenerationAnnotation {
			return
		}
	}
	t.Fatalf("pruned annotations %v lost the collectedGeneration stamp "+
		"(the stamp must be applied after prune)", keysOf(annotations))
}

// TestPruneResource_stampSurvivesScrubRule locks the other half of the
// ordering claim (the stamp is applied after scrubbing): a profile scrub
// denylist that suffix-matches the stamp key (scrub.go suffix matching) must
// not redact the stamp value.
func TestPruneResource_stampSurvivesScrubRule(t *testing.T) {
	t.Parallel()

	obj := deploymentWithGeneration(42)
	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeAll,
		Prune: &kollectdevv1alpha1.PruneSpec{
			ScrubKeys: []string{collectedGenerationAnnotation},
		},
	}

	got := PruneResource(obj, export, NewScrubber([]string{collectedGenerationAnnotation}))

	annotations := embeddedAnnotationsOf(t, got)
	stamp := annotations[collectedGenerationAnnotation]
	if redacted, ok := stamp.(map[string]any); ok {
		t.Fatalf("scrub rule redacted the collectedGeneration stamp: %v (the stamp must be applied after scrub)", redacted)
	}
	if stamp != "42" {
		t.Fatalf("collectedGeneration stamp = %v, want \"42\" (the stamp must be applied after scrub)", stamp)
	}
}

// TestPruneResource_noStampWithoutMetadata locks the honesty clause: when the
// profile's include section excludes metadata there is no place for the
// stamp, the copy carries none, and the export payload is still built.
func TestPruneResource_noStampWithoutMetadata(t *testing.T) {
	t.Parallel()

	obj := deploymentWithGeneration(42)
	export := &kollectdevv1alpha1.ExportSpec{
		Mode:    kollectdevv1alpha1.ExportModeResource,
		Include: kollectdevv1alpha1.ExportIncludeStatusOnly,
	}

	got := PruneResource(obj, export, NewScrubber(nil))
	if got == nil {
		t.Fatal("export must succeed without metadata, got nil")
	}
	if _, ok := got["metadata"]; ok {
		t.Fatalf("StatusOnly must drop metadata, got keys %v", keysOf(got))
	}

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal pruned copy: %v", err)
	}
	if strings.Contains(string(blob), collectedGenerationAnnotation) {
		t.Fatalf("the pruned copy carries the collectedGeneration stamp somewhere else: %s", blob)
	}
}

// TestProcessDispatch_resourceExportPayloadStampedWithGeneration drives the
// production collection path (engine dispatch, Resource mode): the exported
// Item's embedded copy must carry the stamp with the source generation.
func TestProcessDispatch_resourceExportPayloadStampedWithGeneration(t *testing.T) {
	t.Parallel()

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Export: &kollectdevv1alpha1.ExportSpec{
				Mode:    kollectdevv1alpha1.ExportModeResource,
				As:      "resource",
				Include: kollectdevv1alpha1.ExportIncludeAll,
			},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "containerCount", Path: "cel:size(object.spec.template.spec.containers)", Type: "int"},
			},
		},
	}

	e, store := newResourceExportEngine(t, profile)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	obj := deploymentWithGeneration(42)
	obj.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"] =
		[]any{map[string]any{"name": "web"}}
	e.processDispatch(context.Background(), gvr, obj, false)

	items := store.SnapshotTarget("team-a", "deploys")
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}

	blob, ok := items[0].Attributes["resource"].(map[string]any)
	if !ok {
		t.Fatalf("missing embedded resource attribute: %v", items[0].Attributes)
	}

	annotations := embeddedAnnotationsOf(t, blob)
	if stamp := annotations[collectedGenerationAnnotation]; stamp != "42" {
		t.Fatalf("collectedGeneration stamp on the exported copy = %v, want \"42\"", stamp)
	}
}

// TestProcessDispatch_attributesModeUntouchedByStamp is the Attributes-mode
// differential pin: the default Attributes mode must stay byte-identical to
// today's output — declared attributes only, no embedded copy, no stamp key
// anywhere in the exported item.
func TestProcessDispatch_attributesModeUntouchedByStamp(t *testing.T) {
	t.Parallel()

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
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
	if _, ok := item.Attributes["resource"]; ok {
		t.Fatal("Attributes mode must not embed a resource copy")
	}
	if _, ok := item.Attributes[collectedGenerationAnnotation]; ok {
		t.Fatal("Attributes mode must not carry the collectedGeneration stamp at the top level")
	}

	blob, err := json.Marshal(item.Attributes)
	if err != nil {
		t.Fatalf("marshal attributes: %v", err)
	}
	if strings.Contains(string(blob), collectedGenerationAnnotation) {
		t.Fatalf("Attributes-mode output gained the collectedGeneration key: %s", blob)
	}
	if len(item.Attributes) != 1 {
		t.Fatalf("Attributes-mode item gained keys: %v", keysOf(item.Attributes))
	}
}
