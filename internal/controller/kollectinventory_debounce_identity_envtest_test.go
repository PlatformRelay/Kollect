// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// restorePayload removes the item changePayload(name) added, so the namespace payload returns
// to its earlier checksum.
func (env *syncedDebounceEnv) restorePayload(name string) {
	env.store.Remove(env.ns, "demo-target", "uid-"+name)
}

// deleteInventoryAndFinalize deletes the inventory and reconciles it with the env's reconciler
// until its cleanup finalizer is gone and the object no longer exists.
func (env *syncedDebounceEnv) deleteInventoryAndFinalize() {
	GinkgoHelper()

	inv := &kollectdevv1alpha1.KollectInventory{}
	Expect(k8sClient.Get(ctx, env.req.NamespacedName, inv)).To(Succeed())
	Expect(k8sClient.Delete(ctx, inv)).To(Succeed())
	Eventually(func() bool {
		_, err := env.reconciler.Reconcile(context.Background(), env.req)
		Expect(err).NotTo(HaveOccurred())

		return apierrors.IsNotFound(k8sClient.Get(ctx, env.req.NamespacedName, &kollectdevv1alpha1.KollectInventory{}))
	}).WithTimeout(10*time.Second).WithPolling(100*time.Millisecond).Should(BeTrue(),
		"the reconciler removes the cleanup finalizer and the inventory is gone")
}

var _ = Describe("KollectInventory debounce identity (envtest)", func() {
	It("re-exports after a failed export when the payload returns (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-ret", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1), "first pass exports C1")

		env.changePayload("redis")
		only.fail.Store(true)
		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1), "the export of C2 fails")

		env.restorePayload("redis")
		only.fail.Store(false)
		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "the payload back at C1 must export after the failed export")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})

	It("re-exports after a sink spec change (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-spec", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))

		sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: env.ns, Name: only.name}, sinkObj)).To(Succeed())
		before := sinkObj.Generation
		newTable := only.table + "_v2"
		sinkObj.Spec.Postgres.Table = newTable
		Expect(k8sClient.Update(ctx, sinkObj)).To(Succeed())
		Expect(sinkObj.Generation).To(BeNumerically(">", before), "a sink spec edit bumps metadata.generation")
		only.table = newTable

		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "a sink spec change must re-export the unchanged payload")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})

	It("re-exports to a recreated inventory (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-reinv", nil)
		only := env.sinks[0]

		first := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))
		spec := first.Spec.DeepCopy()

		env.deleteInventoryAndFinalize()

		recreated := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: env.req.Name, Namespace: env.ns},
			Spec:       *spec,
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		Expect(recreated.UID).NotTo(Equal(first.UID))
		exportsBefore := only.exports()

		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(exportsBefore+1), "the recreated inventory's first reconcile must export")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})

	It("re-exports to a recreated sink (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-resink", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))

		oldSink := &kollectdevv1alpha1.KollectDatabaseSink{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: env.ns, Name: only.name}, oldSink)).To(Succeed())
		Expect(k8sClient.Delete(ctx, oldSink)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Namespace: env.ns, Name: only.name}, &kollectdevv1alpha1.KollectDatabaseSink{}))
		}).WithTimeout(10 * time.Second).Should(BeTrue())

		env.reconcileAndGet() // reachability fails while the sink is gone
		Expect(only.exports()).To(Equal(1))

		recreated, _ := createPostgresSinkFixtures(only.name, "pg-"+only.name, env.ns)
		recreated.Spec.Postgres.Table = only.table
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, recreated) })
		Expect(recreated.UID).NotTo(Equal(oldSink.UID))

		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "a recreated sink must receive the unchanged payload")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})

	// Control (green on the G2 commit): guards SDC-4's first clause.
	It("exports a changed payload inside the interval (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-chg", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))

		env.changePayload("redis")
		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "a changed payload exports inside the interval")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})

	// Control (green on the G2 commit): a restart starts with an empty tracker.
	It("exports on the first pass of a new reconciler (SDC-4)", func() {
		env := newSyncedDebounceEnv("sdc-new", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))

		first := env.reconciler
		env.reconciler = &KollectInventoryReconciler{
			Client: first.Client, Scheme: first.Scheme, Store: first.Store, Registry: first.Registry, Recorder: first.Recorder,
		}
		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "a new reconciler exports on its first pass")
		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionTrue, reasonExported)
	})
})
