// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

var _ = Describe("KollectInventory Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		var (
			reconcileCtx       context.Context
			typeNamespacedName types.NamespacedName
			kollectinventory   *kollectdevv1alpha1.KollectInventory
		)

		BeforeEach(func() {
			reconcileCtx = context.Background()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			kollectinventory = &kollectdevv1alpha1.KollectInventory{}
			By("creating the custom resource for the Kind KollectInventory")
			err := k8sClient.Get(reconcileCtx, typeNamespacedName, kollectinventory)
			if err != nil && errors.IsNotFound(err) {
				resource := &kollectdevv1alpha1.KollectInventory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(reconcileCtx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &kollectdevv1alpha1.KollectInventory{}
			err := k8sClient.Get(reconcileCtx, typeNamespacedName, resource)
			if err != nil {
				return
			}

			By("Cleanup the specific resource instance KollectInventory")
			_ = k8sClient.Delete(reconcileCtx, resource)
		})
		It("should mark Degraded when a referenced sink is missing", func() {
			inv := &kollectdevv1alpha1.KollectInventory{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, inv)).To(Succeed())
			inv.Spec.SinkRefs = kollectdevv1alpha1.NewSinkRefList("missing-sink-" + testNameSuffix())
			Expect(k8sClient.Update(reconcileCtx, inv)).To(Succeed())

			store := collect.NewStore()
			controllerReconciler := &KollectInventoryReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Store:  store,
			}

			_, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectInventory{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, updated)).To(Succeed())

			degraded := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(degraded).NotTo(BeNil())
			Expect(degraded.Reason).To(Equal(reasonSinkNotFound))
		})

		It("should export and mark Synced when sink registry is configured", func() {
			suffix := testNameSuffix()
			sinkName := "scaffold-pg-" + suffix
			invName := "scaffold-inv-" + suffix
			ns := "scaffold-ns-" + suffix

			Expect(k8sClient.Create(reconcileCtx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(reconcileCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
			}()

			sinkObj, pgSecret := createPostgresSinkFixtures(sinkName, "pg-"+suffix, ns)
			Expect(k8sClient.Create(reconcileCtx, sinkObj)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, sinkObj) }()
			Expect(k8sClient.Create(reconcileCtx, pgSecret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, pgSecret) }()

			inv := &kollectdevv1alpha1.KollectInventory{
				ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns},
				Spec: kollectdevv1alpha1.KollectInventorySpec{
					SinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
				},
			}
			Expect(k8sClient.Create(reconcileCtx, inv)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, inv) }()

			store := collect.NewStore()
			store.Upsert(collect.Item{
				TargetNamespace: ns,
				TargetName:      "demo",
				UID:             "uid-scaffold",
				Namespace:       ns,
				Name:            "nginx",
				Version:         "v1",
				Kind:            "Deployment",
			})

			recorder := &recordingBackend{}
			controllerReconciler := &KollectInventoryReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Store:    store,
				Registry: newPostgresRecordingRegistry(recorder),
			}
			DeferCleanup(func() { sink.EvictBackendPool(ns, sinkName) })

			invKey := types.NamespacedName{Name: invName, Namespace: ns}
			_, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: invKey,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(recorder.exported).To(HaveLen(1))

			updated := &kollectdevv1alpha1.KollectInventory{}
			Expect(k8sClient.Get(reconcileCtx, invKey, updated)).To(Succeed())

			synced := apimeta.FindStatusCondition(updated.Status.Conditions, conditionSynced)
			Expect(synced).NotTo(BeNil())
			Expect(synced.Status).To(Equal(metav1.ConditionTrue))
			Expect(updated.Status.SinkExports).To(HaveLen(1))
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &KollectInventoryReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
