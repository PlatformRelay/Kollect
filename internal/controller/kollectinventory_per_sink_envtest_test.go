// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
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

		sinkObj := &kollectdevv1alpha1.KollectSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectSinkSpec{
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
				SinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
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
		Expect(updated.Status.SinkExports[0].Name).To(Equal(sinkName))
		Expect(updated.Status.SinkExports[0].LastExportTime).NotTo(BeNil())

		synced := apimeta.FindStatusCondition(updated.Status.SinkExports[0].Conditions, conditionSinkSynced)
		Expect(synced).NotTo(BeNil())
		Expect(synced.Status).To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.ItemCount).To(Equal(1))
	})

	It("exports to two sinks and debounces independently on second reconcile", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		invName := "dual-sink-inv-" + suffix
		sinkA := "dual-sink-a-" + suffix
		sinkB := "dual-sink-b-" + suffix
		ns := "default"
		longInterval := metav1.Duration{Duration: 5 * time.Minute}

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
			sinkObj := &kollectdevv1alpha1.KollectSink{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: kollectdevv1alpha1.KollectSinkSpec{
					Type: kollectdevv1alpha1.SinkTypePostgres,
					Postgres: &kollectdevv1alpha1.PostgresSpec{
						DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-" + suffix},
						Table:       "inventory_items",
					},
				},
			}
			Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
			defer func(obj *kollectdevv1alpha1.KollectSink) { _ = k8sClient.Delete(ctx, obj) }(sinkObj)
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
				SinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: sinkA},
					{Name: sinkB},
				},
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

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: ns}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2))

		updated := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		Expect(updated.Status.SinkExports).To(HaveLen(2))

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2), "second reconcile should debounce both sinks")

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, updated)).To(Succeed())
		for _, exportStatus := range updated.Status.SinkExports {
			debounced := apimeta.FindStatusCondition(exportStatus.Conditions, conditionSinkSynced)
			Expect(debounced).NotTo(BeNil())
			Expect(debounced.Reason).To(Equal(kollectdevv1alpha1.ReasonDebounced))
		}
	})
})
