// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// These specs drive the real KollectInventory reconciler against a real git
// backend and a real bare repository: no stub backend stands in for the sink,
// so the export, the recorded lastExportPaths status, the git retraction and
// the CleanupRetained decision are all exercised end to end. They pin the
// K-28 follow-up: a fully retracted default git sink must stay silent, and a
// recorded path the cleanup cannot address must announce retention.

func gitBareRepo(dir, name string) string {
	remote := filepath.Join(dir, name)
	cmd := exec.Command("git", "init", "--bare", "-b", "main", remote) //nolint:gosec // test fixture
	out, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git init --bare: %s", out)

	return remote
}

func gitListTree(remote string) []string {
	cmd := exec.Command("git", "--git-dir", remote, "ls-tree", "-r", "--name-only", "refs/heads/main") //nolint:gosec // test fixture
	out, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git ls-tree")

	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}

	return paths
}

func gitHasPath(remote, path string) bool {
	cmd := exec.Command("git", "--git-dir", remote, "cat-file", "-e", "refs/heads/main:"+path) //nolint:gosec // test fixture

	return cmd.Run() == nil
}

func newGitLiveInventory(suffix, sinkName string) *kollectdevv1alpha1.KollectInventory {
	return &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "live-inv-" + suffix, Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
		},
	}
}

func liveInventoryStore(item collect.Item) *collect.Store {
	store := collect.NewStore()
	store.Upsert(item)

	return store
}

var _ = Describe("KollectInventory cleanup evidence with a real git sink (envtest)", func() {
	It("records the exported path and stays silent when the deletion fully retracts it", func() {
		suffix := testNameSuffix()
		sinkName := "live-git-clean-" + suffix
		invName := "live-inv-" + suffix
		remote := gitBareRepo(GinkgoT().TempDir(), "remote.git")

		sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
				DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint:          "file://" + remote,
					ExportMinInterval: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sinkObj) })

		inv := newGitLiveInventory(suffix, sinkName)
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		store := liveInventoryStore(collect.Item{
			TargetNamespace: "default",
			TargetName:      "nginx-deployments",
			UID:             "uid-live",
			Namespace:       "default",
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"image": "nginx:1.27-alpine"},
		})

		recorder := record.NewFakeRecorder(20)
		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: sink.NewRegistry(),
			Recorder: recorder,
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: "default"}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		var afterExport kollectdevv1alpha1.KollectInventory
		Expect(k8sClient.Get(ctx, req.NamespacedName, &afterExport)).To(Succeed())
		Expect(afterExport.Status.SinkExports).To(HaveLen(1))
		recorded := afterExport.Status.SinkExports[0].LastExportPaths
		Expect(recorded).NotTo(BeEmpty(), "the last export must be recorded in status")
		GinkgoWriter.Printf("recorded lastExportPaths=%v\n", recorded)

		// A second reconcile inside the export interval debounces; the rebuilt
		// status entry must carry the recorded paths over so the deletion-time
		// cleanup still has its retraction evidence.
		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		var afterDebounce kollectdevv1alpha1.KollectInventory
		Expect(k8sClient.Get(ctx, req.NamespacedName, &afterDebounce)).To(Succeed())
		Expect(afterDebounce.Status.SinkExports).To(HaveLen(1))
		Expect(afterDebounce.Status.SinkExports[0].LastExportPaths).To(Equal(recorded),
			"a debounced reconcile must carry the recorded export paths over")

		tree := gitListTree(remote)
		GinkgoWriter.Printf("git tree after export=%v\n", tree)
		for _, p := range recorded {
			Expect(gitHasPath(remote, p)).To(BeTrue(), "recorded path %q must exist in the real repo", p)
		}

		Expect(k8sClient.Delete(ctx, &afterExport)).To(Succeed())
		Eventually(func(g Gomega) {
			var deleting kollectdevv1alpha1.KollectInventory
			g.Expect(k8sClient.Get(ctx, req.NamespacedName, &deleting)).To(Succeed())
			g.Expect(deleting.DeletionTimestamp).NotTo(BeNil())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var gone kollectdevv1alpha1.KollectInventory
			g.Expect(k8sClient.Get(ctx, req.NamespacedName, &gone)).To(HaveOccurred())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		Expect(drainCleanupRetained(recorder)).To(BeFalse(),
			"a fully retracted default git sink must not announce CleanupRetained")

		treeAfter := gitListTree(remote)
		GinkgoWriter.Printf("git tree after deletion=%v\n", treeAfter)
		for _, p := range recorded {
			Expect(gitHasPath(remote, p)).To(BeFalse(), "recorded path %q must be retracted from the real repo", p)
		}
	})

	It("announces retention when the recorded path is no longer addressable", func() {
		suffix := testNameSuffix()
		sinkName := "live-git-changed-" + suffix
		invName := "live-inv-" + suffix
		remote := gitBareRepo(GinkgoT().TempDir(), "remote.git")

		sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
				DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "file://" + remote,
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sinkObj) })

		inv := newGitLiveInventory(suffix, sinkName)
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		store := liveInventoryStore(collect.Item{
			TargetNamespace: "default",
			TargetName:      "nginx-deployments",
			UID:             "uid-live-changed",
			Namespace:       "default",
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"image": "nginx:1.27-alpine"},
		})

		recorder := record.NewFakeRecorder(20)
		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: sink.NewRegistry(),
			Recorder: recorder,
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: "default"}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		var afterExport kollectdevv1alpha1.KollectInventory
		Expect(k8sClient.Get(ctx, req.NamespacedName, &afterExport)).To(Succeed())
		recorded := afterExport.Status.SinkExports[0].LastExportPaths
		Expect(recorded).NotTo(BeEmpty())
		GinkgoWriter.Printf("recorded lastExportPaths=%v\n", recorded)

		// The sink's pathTemplate changes after the last export: the retraction
		// now renders different candidate paths and cannot address what was
		// actually written. The recorded evidence must announce retention
		// instead of the silent false-clean tombstone.
		sinkObj.Spec.PathTemplate = "archive/{namespace}/{name}.yaml"
		Expect(k8sClient.Update(ctx, sinkObj)).To(Succeed())

		Expect(k8sClient.Delete(ctx, &afterExport)).To(Succeed())
		Eventually(func(g Gomega) {
			var deleting kollectdevv1alpha1.KollectInventory
			g.Expect(k8sClient.Get(ctx, req.NamespacedName, &deleting)).To(Succeed())
			g.Expect(deleting.DeletionTimestamp).NotTo(BeNil())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		Expect(drainCleanupRetained(recorder)).To(BeTrue(),
			"a recorded path the cleanup cannot address must announce CleanupRetained")

		for _, p := range recorded {
			Expect(gitHasPath(remote, p)).To(BeTrue(),
				"the unaddressable object %q must remain in the real repo", p)
		}
	})

	It("announces retention for a recorded auto-upgraded per-resource tree", func() {
		suffix := testNameSuffix()
		sinkName := "live-git-tree-" + suffix
		invName := "live-inv-" + suffix
		remote := gitBareRepo(GinkgoT().TempDir(), "remote.git")

		sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
				DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "file://" + remote,
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sinkObj) })

		inv := newGitLiveInventory(suffix, sinkName)
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		// An embedded-object attribute makes the unset-layout export auto-upgrade
		// to the per-resource tree (layout_export.go inferResourceLayoutHints).
		manifest := map[string]any{"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]any{"namespace": "default", "name": "nginx"}}
		store := liveInventoryStore(collect.Item{
			TargetNamespace: "default",
			TargetName:      "nginx-deployments",
			UID:             "uid-live-tree",
			Namespace:       "default",
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"payload": manifest, "image": "nginx:1.27-alpine"},
		})

		recorder := record.NewFakeRecorder(20)
		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: sink.NewRegistry(),
			Recorder: recorder,
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: "default"}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		var afterExport kollectdevv1alpha1.KollectInventory
		Expect(k8sClient.Get(ctx, req.NamespacedName, &afterExport)).To(Succeed())
		recorded := afterExport.Status.SinkExports[0].LastExportPaths
		Expect(recorded).NotTo(BeEmpty())
		GinkgoWriter.Printf("recorded tree lastExportPaths=%v\n", recorded)

		Expect(k8sClient.Delete(ctx, &afterExport)).To(Succeed())
		Eventually(func(g Gomega) {
			var deleting kollectdevv1alpha1.KollectInventory
			g.Expect(k8sClient.Get(ctx, req.NamespacedName, &deleting)).To(Succeed())
			g.Expect(deleting.DeletionTimestamp).NotTo(BeNil())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		Expect(drainCleanupRetained(recorder)).To(BeTrue(),
			"a recorded per-resource tree must announce CleanupRetained")
	})
})

// drainCleanupRetained reports whether the fake recorder captured any
// CleanupRetained warning event.
func drainCleanupRetained(recorder *record.FakeRecorder) bool {
	for {
		select {
		case ev := <-recorder.Events:
			if strings.Contains(ev, reasonCleanupRetained) {
				GinkgoWriter.Printf("cleanup event: %s\n", ev)

				return true
			}
		default:
			return false
		}
	}
}

var _ = Describe("KollectSnapshotSink deletionPolicy default (envtest)", func() {
	It("defaults a sink created without deletionPolicy to Retain and keeps its exports on inventory deletion", func() {
		suffix := testNameSuffix()
		sinkName := "live-git-retain-" + suffix
		invName := "live-inv-" + suffix
		remote := gitBareRepo(GinkgoT().TempDir(), "remote.git")

		sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "file://" + remote,
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sinkObj) })

		var stored kollectdevv1alpha1.KollectSnapshotSink
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sinkName, Namespace: "default"}, &stored)).To(Succeed())
		Expect(stored.Spec.DeletionPolicy).To(Equal(kollectdevv1alpha1.DeletionPolicyRetain),
			"the CRD must default deletionPolicy to Retain")

		inv := newGitLiveInventory(suffix, sinkName)
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		recorder := record.NewFakeRecorder(20)
		reconciler := &KollectInventoryReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Store: liveInventoryStore(collect.Item{
				TargetNamespace: "default",
				TargetName:      "nginx-deployments",
				UID:             "uid-live-retain",
				Namespace:       "default",
				Name:            "nginx",
				Version:         "v1",
				Kind:            "Deployment",
				Attributes:      map[string]any{"image": "nginx:1.27-alpine"},
			}),
			Registry: sink.NewRegistry(),
			Recorder: recorder,
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: "default"}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		var afterExport kollectdevv1alpha1.KollectInventory
		Expect(k8sClient.Get(ctx, req.NamespacedName, &afterExport)).To(Succeed())
		Expect(afterExport.Status.SinkExports).To(HaveLen(1))
		recorded := afterExport.Status.SinkExports[0].LastExportPaths
		Expect(recorded).NotTo(BeEmpty())

		Expect(k8sClient.Delete(ctx, &afterExport)).To(Succeed())
		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var gone kollectdevv1alpha1.KollectInventory
			g.Expect(k8sClient.Get(ctx, req.NamespacedName, &gone)).To(HaveOccurred())
		}).WithTimeout(5*time.Second).Should(Succeed(), "the finalizer must be released under Retain")

		for _, p := range recorded {
			Expect(gitHasPath(remote, p)).To(BeTrue(), "Retain must leave exported path %q in the repo", p)
		}

		var policyEvent bool
		for len(recorder.Events) > 0 {
			if ev := <-recorder.Events; strings.Contains(ev, reasonCleanupRetainedByPolicy) {
				policyEvent = true
			}
		}
		Expect(policyEvent).To(BeTrue(), "a %s event must name the retained sink", reasonCleanupRetainedByPolicy)
	})
})
