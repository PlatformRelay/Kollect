// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

var _ = Describe("KollectInventory per-sink export (envtest)", func() {
	It("records sinkExports after reconciler export", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		invName := "per-sink-inv-" + suffix
		sinkName := "per-sink-pg-" + suffix
		ns := "default"

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-demo",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"image": "nginx:1.27"},
		})

		sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-" + suffix},
					Table:       "inventory_items",
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, sinkObj) }()

		pgSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "pg-" + suffix, Namespace: ns},
			Data:       map[string][]byte{"dsn": []byte("postgres://example")},
		}
		Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, pgSecret) }()

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		recorder := &recordingBackend{}
		reg := sink.NewRegistry()
		reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
			_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
		) (sink.Backend, error) {
			return recorder, nil
		})

		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: reg,
		}

		_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Name: invName, Namespace: ns},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(1))

		updated := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		Expect(updated.Status.SinkExports).To(HaveLen(1))
		Expect(updated.Status.SinkExports[0].Name).To(Equal(string(kollectdevv1alpha1.SinkFamilyDatabase) + "/" + sinkName))
		Expect(updated.Status.SinkExports[0].LastExportTime).NotTo(BeNil())

		synced := apimeta.FindStatusCondition(updated.Status.SinkExports[0].Conditions, conditionSinkSynced)
		Expect(synced).NotTo(BeNil())
		Expect(synced.Status).To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.ItemCount).To(Equal(1))
	})

	It("exports to two sinks and debounces independently on second reconcile", func() {
		suffix := testNameSuffix()
		invName := "dual-sink-inv-" + suffix
		sinkA := "dual-sink-a-" + suffix
		sinkB := "dual-sink-b-" + suffix
		ns := "dual-sink-ns-" + suffix
		longInterval := metav1.Duration{Duration: 15 * time.Minute}

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}()

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-dual",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
		})

		for _, name := range []string{sinkA, sinkB} {
			sinkObj, _ := createPostgresSinkFixtures(name, "pg-"+suffix, ns)
			Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
			defer func(obj *kollectdevv1alpha1.KollectDatabaseSink) { _ = k8sClient.Delete(ctx, obj) }(sinkObj)
		}

		pgSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "pg-" + suffix, Namespace: ns},
			Data:       map[string][]byte{"dsn": []byte("postgres://example")},
		}
		Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, pgSecret) }()

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				ExportMinInterval: &longInterval,
				DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: sinkA},
					{Name: sinkB},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		recorder := &recordingBackend{}
		reg := newPostgresRecordingRegistry(recorder)

		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: reg,
		}

		DeferCleanup(func() {
			sink.EvictBackendPool(ns, sinkA)
			sink.EvictBackendPool(ns, sinkB)
		})

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: ns}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2))

		updated := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		Expect(updated.Status.SinkExports).To(HaveLen(2))

		recorder.mu.Lock()
		recorder.exported = recorder.exported[:0]
		recorder.mu.Unlock()

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		recorder.mu.Lock()
		secondExports := len(recorder.exported)
		recorder.mu.Unlock()
		Expect(secondExports).To(Equal(0), "second reconcile should debounce both sinks")

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		for _, exportStatus := range updated.Status.SinkExports {
			debounced := apimeta.FindStatusCondition(exportStatus.Conditions, conditionSinkSynced)
			Expect(debounced).NotTo(BeNil())
			Expect(debounced.Reason).To(Equal(kollectdevv1alpha1.ReasonDebounced))
		}
	})
})

var _ = Describe("KollectInventory deletion cleanup (envtest)", func() {
	It("exports empty snapshot and removes finalizer on delete", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		invName := "delete-inv-" + suffix
		sinkName := "delete-pg-" + suffix
		ns := "default"

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-delete",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
		})

		sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-" + suffix},
					Table:       "inventory_items",
				},
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, sinkObj) }()

		pgSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "pg-" + suffix, Namespace: ns},
			Data:       map[string][]byte{"dsn": []byte("postgres://example")},
		}
		Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, pgSecret) }()

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		recorder := &relationalRecordingBackend{}
		reg := sink.NewRegistry()
		reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
			_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
		) (sink.Backend, error) {
			return recorder, nil
		})

		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: reg,
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: ns}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(1))

		updated := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		Expect(updated.Finalizers).To(ContainElement(inventoryCleanupFinalizer))

		Expect(k8sClient.Delete(ctx, updated)).To(Succeed())

		Eventually(func(g Gomega) {
			var deleting kollectdevv1alpha1.KollectInventory
			getErr := k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, &deleting)
			g.Expect(getErr).NotTo(HaveOccurred())
			g.Expect(deleting.DeletionTimestamp).NotTo(BeNil())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2))

		Eventually(func(g Gomega) {
			var gone kollectdevv1alpha1.KollectInventory
			getErr := k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, &gone)
			g.Expect(getErr).To(HaveOccurred())
		}).WithTimeout(5 * time.Second).Should(Succeed())

		Expect(recorder.exported[1]).NotTo(BeEmpty(), "relational delete-recon must export empty snapshot payload")
		Expect(string(recorder.exported[1])).To(ContainSubstring(`"items":[]`))
	})
})

var _ = Describe("KollectInventory partial multi-sink export (envtest)", func() {
	It("marks PartiallySynced when one sink fails and one succeeds (EC-P1-02)", func() {
		suffix := testNameSuffix()
		invName := "partial-inv-" + suffix
		sinkOK := "partial-ok-" + suffix
		sinkFail := "partial-fail-" + suffix
		ns := "default"

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-partial",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
		})

		for _, pair := range []struct{ sinkName, secretName, table string }{
			{sinkOK, "pg-ok-" + suffix, "items_ok"},
			{sinkFail, "pg-fail-" + suffix, "items_fail"},
		} {
			sinkObj, pgSecret := createPostgresSinkFixtures(pair.sinkName, pair.secretName, ns)
			sinkObj.Spec.Postgres.Table = pair.table
			Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
			defer func(obj *kollectdevv1alpha1.KollectDatabaseSink) { _ = k8sClient.Delete(ctx, obj) }(sinkObj)
			Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
			defer func(obj *corev1.Secret) { _ = k8sClient.Delete(ctx, obj) }(pgSecret)
		}

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns, Generation: 1},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: sinkFail},
					{Name: sinkOK},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		recorder := &recordingBackend{}
		exportErr := kollecterrors.Transient(errors.New("sink unavailable"))
		reg := newPostgresRecordingRegistryWithSelector(func(spec kollectdevv1alpha1.KollectSinkSpec) (sink.Backend, error) {
			if spec.Postgres != nil && spec.Postgres.Table == "items_fail" {
				return &failingBackend{err: exportErr}, nil
			}
			return recorder, nil
		})

		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Registry: reg,
		}

		DeferCleanup(func() {
			sink.EvictBackendPool(ns, sinkFail)
			sink.EvictBackendPool(ns, sinkOK)
		})

		_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Name: invName, Namespace: ns},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(1))

		updated := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())

		synced := apimeta.FindStatusCondition(updated.Status.Conditions, conditionSynced)
		Expect(synced).NotTo(BeNil())
		Expect(synced.Reason).To(Equal(kollectdevv1alpha1.ReasonPartiallySynced))

		var failExport *kollectdevv1alpha1.InventorySinkExportStatus
		for i := range updated.Status.SinkExports {
			if updated.Status.SinkExports[i].Name == string(kollectdevv1alpha1.SinkFamilyDatabase)+"/"+sinkFail {
				failExport = &updated.Status.SinkExports[i]
				break
			}
		}
		Expect(failExport).NotTo(BeNil())
		failSynced := apimeta.FindStatusCondition(failExport.Conditions, conditionSinkSynced)
		Expect(failSynced).NotTo(BeNil())
		Expect(failSynced.Reason).To(Equal(reasonExportFailed))
	})

	It("sets SpokePublishFailed when hub publish fails (EC-P1-07)", func() {
		suffix := testNameSuffix()
		invName := "spoke-fail-" + suffix
		ns := "spoke-fail-ns-" + suffix

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}()

		DeferCleanup(func() {
			_ = os.Unsetenv("KOLLECT_SPOKE_CLUSTER")
			_ = os.Unsetenv("KOLLECT_TRANSPORT_TYPE")
		})
		Expect(os.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")).To(Succeed())
		Expect(os.Setenv("KOLLECT_TRANSPORT_TYPE", "not-a-real-transport")).To(Succeed())

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-spoke",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
		})

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns, Generation: 1},
			Spec:       kollectdevv1alpha1.KollectInventorySpec{},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		fakeRecorder := record.NewFakeRecorder(1)
		reconciler := &KollectInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Store:    store,
			Recorder: fakeRecorder,
		}

		_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Name: invName, Namespace: ns},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(fakeRecorder.Events, 5*time.Second).Should(Receive(ContainSubstring("SpokePublishFailed")))
	})
})
