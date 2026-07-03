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

	It("mapProfileToTargets enqueues only targets in the profile's own namespace, "+
		"even when a same-named profile has a matching target in another namespace "+
		"(namespace-scoped List enqueue correctness)", func() {
		suffix := testNameSuffix()
		nsA := "map-target-profile-a-" + suffix
		nsB := "map-target-profile-b-" + suffix

		for _, name := range []string{nsA, nsB} {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			})).To(Succeed())
		}
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsA}})
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsB}})
		}()

		targetA := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "deploys-a", Namespace: nsA},
			Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
		}
		targetB := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "deploys-b", Namespace: nsB},
			Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
		}
		Expect(k8sClient.Create(ctx, targetA)).To(Succeed())
		Expect(k8sClient.Create(ctx, targetB)).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, targetA)
			_ = k8sClient.Delete(ctx, targetB)
		}()

		// Same profile name, but scoped to nsA only. A real apiserver List with
		// client.InNamespace(nsA) must not return targetB even though it shares
		// the same profileRef and profile name.
		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: nsA},
		}

		reconciler := &KollectTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

		reqs := reconciler.mapProfileToTargets(ctx, profile)
		Expect(reqs).To(ConsistOf(reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: nsA, Name: "deploys-a"},
		}))
	})

	It("mapProfileToTargets suppresses the enqueue it would otherwise produce when "+
		"target List fails (RBAC-shaped list denial)", func() {
		suffix := testNameSuffix()
		ns := "map-target-list-fail-" + suffix

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}()

		target := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: ns},
			Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, target) }()

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: ns},
		}

		// Control: List succeeds against the real apiserver, so the target must be enqueued.
		reconciler := &KollectTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		control := reconciler.mapProfileToTargets(ctx, profile)
		Expect(control).To(ConsistOf(reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: "deploys"},
		}), "sanity check: the target must be enqueued when List succeeds")

		// Failure path: List errors must suppress the enqueue rather than returning stale requests.
		failClient := newListFailTargetClient(k8sClient, errors.New("simulated RBAC list denied"))
		failReconciler := &KollectTargetReconciler{Client: failClient, Scheme: k8sClient.Scheme()}
		reqs := failReconciler.mapProfileToTargets(ctx, profile)
		Expect(reqs).To(BeEmpty(), "List failure must not silently enqueue stale requests")
	})

	It("mapProfileToClusterTargets suppresses the enqueue it would otherwise produce when "+
		"cluster target List fails (RBAC-shaped list denial)", func() {
		suffix := testNameSuffix()
		ctName := "ct-list-fail-" + suffix
		profileNs := "map-ct-list-fail-" + suffix

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: profileNs}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: profileNs}})
		}()

		profileRef := kollectdevv1alpha1.NamespacedObjectReference{Name: "deployments", Namespace: profileNs}
		clusterTarget := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: ctName},
			Spec:       kollectdevv1alpha1.KollectClusterTargetSpec{ProfileRef: profileRef},
		}
		Expect(k8sClient.Create(ctx, clusterTarget)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, clusterTarget) }()

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: profileNs},
		}

		// Control: List succeeds against the real apiserver, so the cluster target must be enqueued.
		reconciler := &KollectClusterTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		control := reconciler.mapProfileToClusterTargets(ctx, profile)
		Expect(control).To(ConsistOf(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: ctName},
		}), "sanity check: the cluster target must be enqueued when List succeeds")

		// Failure path: List errors must suppress the enqueue rather than returning stale requests.
		failClient := newListFailClusterTargetClient(k8sClient, errors.New("simulated RBAC list denied"))
		failReconciler := &KollectClusterTargetReconciler{Client: failClient, Scheme: k8sClient.Scheme()}
		reqs := failReconciler.mapProfileToClusterTargets(ctx, profile)
		Expect(reqs).To(BeEmpty(), "List failure must not silently enqueue stale requests")
	})
})

func counterValue(vec *prometheus.CounterVec, labels ...string) float64 {
	return testutil.ToFloat64(vec.WithLabelValues(labels...))
}
