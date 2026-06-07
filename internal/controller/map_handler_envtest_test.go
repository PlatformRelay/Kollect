// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

var _ = Describe("Map handler List failure (envtest)", func() {
	It("increments metric and returns no enqueue when inventory List fails (EC-P1-04)", func() {
		suffix := testNameSuffix()
		ns := "map-list-" + suffix
		sinkName := "map-list-sink-" + suffix

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}()

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "inv-" + suffix, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		sinkObj, pgSecret := createPostgresSinkFixtures(sinkName, "pg-"+suffix, ns)
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, sinkObj)
			_ = k8sClient.Delete(ctx, pgSecret)
		}()

		before := counterValue(metrics.WatchMapListErrorsTotal, "KollectInventory", "sink")

		failClient := newListFailInventoryClient(k8sClient, errors.New("simulated RBAC list denied"))
		reconciler := &KollectInventoryReconciler{
			Client: failClient,
			Scheme: k8sClient.Scheme(),
		}

		reqs := reconciler.mapDatabaseSinkToInventories(ctx, sinkObj)
		Expect(reqs).To(BeEmpty(), "List failure must not silently enqueue stale requests")

		after := counterValue(metrics.WatchMapListErrorsTotal, "KollectInventory", "sink")
		Expect(after - before).To(Equal(float64(1)))
	})

	It("inventoriesInNamespace enqueues only inventories in changed namespace (PERF-01 / ADR-0301)", func() {
		suffix := testNameSuffix()
		nsA := "map-target-a-" + suffix
		nsB := "map-target-b-" + suffix

		for _, name := range []string{nsA, nsB} {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			})).To(Succeed())
		}
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsA}})
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsB}})
		}()

		Expect(k8sClient.Create(ctx, &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: nsA},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: nsB},
		})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: nsA},
			})
			_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: nsB},
			})
		}()

		reqs := inventoriesInNamespace(ctx, k8sClient, nsA)
		Expect(reqs).To(ConsistOf(reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: nsA, Name: "inv-a"},
		}))
	})
})

func counterValue(vec *prometheus.CounterVec, labels ...string) float64 {
	return testutil.ToFloat64(vec.WithLabelValues(labels...))
}
