// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/sink/git"
	"github.com/platformrelay/kollect/internal/sink/layout"
)

// treeBackend records every ExportFiles call so a test can assert the per-set manifest sidecar
// (REL-02-FUP) is emitted exactly once, on the final part, and survives the single union-prune.
type treeBackend struct {
	mu    sync.Mutex
	calls []treeCall
}

type treeCall struct {
	files          []git.FileEntry
	prune          bool
	pruneKeepPaths []string
}

func (b *treeBackend) Type() string { return "tree" }

func (b *treeBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

// Export satisfies sink.Backend for the single-document fallback; the multipart tree path uses
// ExportFiles below.
func (b *treeBackend) Export(_ context.Context, _ []byte, _ string) error { return nil }

func (b *treeBackend) ExportFiles(_ context.Context, files []git.FileEntry, opts git.ExportFilesOptions) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]git.FileEntry, len(files))
	copy(cp, files)
	b.calls = append(b.calls, treeCall{
		files:          cp,
		prune:          opts.Prune && !opts.SuppressPrune,
		pruneKeepPaths: append([]string(nil), opts.PruneKeepPaths...),
	})

	return nil
}

func (b *treeBackend) manifestFile(t *testing.T) (git.FileEntry, treeCall) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var found git.FileEntry
	var owner treeCall
	count := 0
	for _, call := range b.calls {
		for _, f := range call.files {
			if strings.HasSuffix(f.Path, ".manifest.json") {
				found = f
				owner = call
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("want exactly one manifest sidecar across all parts, got %d", count)
	}

	return found, owner
}

func newYAMLGitSetup(t *testing.T, itemCount int, limit int64) (*KollectInventoryReconciler, *treeBackend, types.NamespacedName) {
	t.Helper()

	return newYAMLGitSetupMode(t, itemCount, limit, kollectdevv1alpha1.LayoutModePerResource)
}

func newYAMLGitSetupMode(t *testing.T, itemCount int, limit int64, mode string) (*KollectInventoryReconciler, *treeBackend, types.NamespacedName) {
	t.Helper()

	store := collect.NewStore()
	for i := range itemCount {
		store.Upsert(collect.Item{
			TargetNamespace: "default",
			TargetName:      "nginx-deployments",
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       "default",
			Name:            fmt.Sprintf("nginx-%d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"payload": strings.Repeat("x", 220)},
		})
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	// perResource + default (YAML) serialization: the human-readable layout projection carries no
	// envelope metadata, so torn-set detection must come from the manifest sidecar.
	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "https://example.com/inventory.git",
				Layout:   &kollectdevv1alpha1.LayoutSpec{Mode: mode},
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-demo"),
			MaxExportBytes:   &limit,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	backend := &treeBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return backend, nil
	})

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}

	return rec, backend, types.NamespacedName{Name: "team-inventory", Namespace: "default"}
}

// TestKollectInventoryReconciler_multipartYAMLEmitsSetManifest is the REL-02-FUP end-to-end RED test:
// a 3-part perResource YAML export must emit exactly one per-set manifest sidecar at the deterministic
// path declaring generation, partTotal=3, per-part identifiers, and the union of projected file paths;
// the sidecar must be a member of the single union-prune keep set (prune-survival); and it must be
// distinguishable from data files (distinct .manifest.json extension + self-identifying kind).
func TestKollectInventoryReconciler_multipartYAMLEmitsSetManifest(t *testing.T) {
	t.Parallel()

	// A tight ceiling forces one item per part => three parts.
	rec, backend, invKey := newYAMLGitSetup(t, 3, 900)

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: invKey}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// The export must have succeeded (all three parts) so the final part emits the manifest.
	var got kollectdevv1alpha1.KollectInventory
	if err := rec.Get(context.Background(), invKey, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	synced := apimeta.FindStatusCondition(got.Status.Conditions, conditionSinkSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("Synced = %+v, want True (all parts succeeded)", synced)
	}

	if len(backend.calls) != 3 {
		t.Fatalf("ExportFiles calls = %d, want 3 (one per part)", len(backend.calls))
	}

	manifestFile, owner := backend.manifestFile(t)

	// Graceful-ignore: distinct extension so a *.yaml globber skips it.
	if !strings.HasSuffix(manifestFile.Path, ".manifest.json") {
		t.Fatalf("manifest path %q must end in .manifest.json (distinct from .yaml)", manifestFile.Path)
	}
	if manifestFile.Path != "inventory/default/team-inventory.manifest.json" {
		t.Fatalf("manifest path = %q, want deterministic inventory/default/team-inventory.manifest.json", manifestFile.Path)
	}

	var m layout.SetManifest
	if err := json.Unmarshal(manifestFile.Data, &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	// Self-identifying for graceful-ignore.
	if m.Kind != layout.SetManifestKind || m.SchemaVersion != collect.ExportSchemaVersion {
		t.Fatalf("manifest kind/schema = %q/%q, want %q/%q", m.Kind, m.SchemaVersion, layout.SetManifestKind, collect.ExportSchemaVersion)
	}
	if m.Generation != got.Generation {
		t.Fatalf("manifest generation = %d, want %d", m.Generation, got.Generation)
	}
	if m.PartTotal != 3 {
		t.Fatalf("manifest partTotal = %d, want 3", m.PartTotal)
	}
	if len(m.Parts) != 3 {
		t.Fatalf("manifest parts = %v, want the three per-part identifiers", m.Parts)
	}

	// The union of projected data-file paths: one per part (three items => three files).
	if len(m.Paths) != 3 {
		t.Fatalf("manifest union paths = %v, want 3 (one file per part)", m.Paths)
	}
	// Consumer applying ADR-0405 validation against the on-disk set reports COMPLETE.
	if v := layout.VerifySet(m, m.Paths, got.Generation); !v.Complete {
		t.Fatalf("consumer validation must report COMPLETE for a whole set, got %+v", v)
	}

	// Prune-survival: the manifest is written on the final part, which prunes once against the union,
	// and the manifest path is a member of the keep set (replaced-not-orphaned on regen).
	if !owner.prune {
		t.Fatal("manifest must ride the final, pruning part (single union-prune)")
	}
	foundKeep := false
	for _, p := range owner.pruneKeepPaths {
		if p == manifestFile.Path {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Fatalf("manifest path %q must be in the prune keep set %v (or prune would orphan it)", manifestFile.Path, owner.pruneKeepPaths)
	}
	// Every union data path is also kept.
	for _, dp := range m.Paths {
		kept := false
		for _, kp := range owner.pruneKeepPaths {
			if kp == dp {
				kept = true
			}
		}
		if !kept {
			t.Fatalf("data path %q missing from prune keep set %v (part would be lost)", dp, owner.pruneKeepPaths)
		}
	}
}

// TestKollectInventoryReconciler_singlePartYAMLNoManifest discharges the single-part byte-compat AC:
// a snapshot that fits in one part emits NO sidecar, so existing readers are unaffected.
func TestKollectInventoryReconciler_singlePartYAMLNoManifest(t *testing.T) {
	t.Parallel()

	// A generous ceiling keeps all items in a single part.
	rec, backend, invKey := newYAMLGitSetup(t, 3, 1<<20)

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: invKey}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(backend.calls) != 1 {
		t.Fatalf("ExportFiles calls = %d, want 1 (single part)", len(backend.calls))
	}
	for _, f := range backend.calls[0].files {
		if strings.HasSuffix(f.Path, ".manifest.json") {
			t.Fatalf("single-part export must emit NO manifest sidecar, found %q", f.Path)
		}
	}
}

// TestKollectInventoryReconciler_splitMultipartTwoControlFilesSurvivePrune covers the split-mode-unique
// interaction: split places TWO control files under inventory/{ns}/ -- the per-projection split Index
// AND the per-set manifest sidecar -- a prune/keep-set case perResource never exercises. Both control
// files must land under inventory/{ns}/ and both must survive the single union-prune (be in the final
// part's keep-set), or a consumer's index and completeness marker would be silently pruned.
func TestKollectInventoryReconciler_splitMultipartTwoControlFilesSurvivePrune(t *testing.T) {
	t.Parallel()

	// A tight ceiling forces one item per part => three parts, split layout (index auto-enabled).
	rec, backend, invKey := newYAMLGitSetupMode(t, 3, 900, kollectdevv1alpha1.LayoutModeSplit)

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: invKey}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := rec.Get(context.Background(), invKey, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	synced := apimeta.FindStatusCondition(got.Status.Conditions, conditionSinkSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("Synced = %+v, want True (all parts succeeded)", synced)
	}
	if len(backend.calls) != 3 {
		t.Fatalf("ExportFiles calls = %d, want 3 (one per part)", len(backend.calls))
	}

	const ctrlDir = "inventory/default/"

	// The per-set manifest sidecar: exactly one, at the deterministic set path under inventory/{ns}/.
	manifestFile, finalCall := backend.manifestFile(t)
	if manifestFile.Path != ctrlDir+"team-inventory.manifest.json" {
		t.Fatalf("manifest path = %q, want %steam-inventory.manifest.json", manifestFile.Path, ctrlDir)
	}

	// The split Index control file(s): under inventory/{ns}/, .yaml, and NOT the manifest. Split emits
	// one per part, so at least one must appear across the exported files.
	splitIndexPaths := map[string]struct{}{}
	for _, call := range backend.calls {
		for _, f := range call.files {
			if strings.HasPrefix(f.Path, ctrlDir) && strings.HasSuffix(f.Path, ".yaml") {
				splitIndexPaths[f.Path] = struct{}{}
			}
		}
	}
	if len(splitIndexPaths) == 0 {
		t.Fatal("split mode must emit a layout.Index control file under inventory/{ns}/, found none")
	}

	// Prune-survival: the final part prunes once against the union; BOTH control-file kinds must be in
	// the keep-set. Data files live under {cluster}/{sourceNs}/{kind}/ and are asserted kept too.
	if !finalCall.prune {
		t.Fatal("the manifest must ride the final, pruning part (single union-prune)")
	}
	keep := map[string]struct{}{}
	for _, p := range finalCall.pruneKeepPaths {
		keep[p] = struct{}{}
	}
	if _, ok := keep[manifestFile.Path]; !ok {
		t.Fatalf("manifest %q missing from union keep-set %v (prune would orphan it)", manifestFile.Path, finalCall.pruneKeepPaths)
	}
	for idx := range splitIndexPaths {
		if _, ok := keep[idx]; !ok {
			t.Fatalf("split index %q missing from union keep-set %v (prune would orphan it)", idx, finalCall.pruneKeepPaths)
		}
	}
	// Sanity: the union also keeps every data path (three items => three data files).
	dataKept := 0
	for p := range keep {
		if !strings.HasPrefix(p, ctrlDir) {
			dataKept++
		}
	}
	if dataKept != 3 {
		t.Fatalf("union keep-set data paths = %d, want 3 (one per part)", dataKept)
	}
}
