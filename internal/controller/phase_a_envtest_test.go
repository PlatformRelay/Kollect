// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"os/exec"
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

const envtestGitProbeEndpoint = "https://github.com/octocat/Hello-World.git"

func gitProbeReachable() bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", envtestGitProbeEndpoint) //nolint:gosec // G204: probe fixture
	return cmd.Run() == nil
}

var _ = Describe("Phase A envtest — map sink and degraded conflict", func() {
	It("re-exports inventory when a referenced sink changes (EC-P1-04)", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		invName := "map-sink-inv-" + suffix
		sinkName := "map-sink-" + suffix
		ns := "default"

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-map",
			Namespace:       ns,
			Name:            "nginx",
			Version:         "v1",
			Kind:            "Deployment",
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

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: ns}}
		_, err := reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(1))

		updatedSink := &kollectdevv1alpha1.KollectSink{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sinkName, Namespace: ns}, updatedSink)).To(Succeed())
		updatedSink.Spec.Postgres.Table = "inventory_items_v2"
		Expect(k8sClient.Update(ctx, updatedSink)).To(Succeed())

		reqs := reconciler.mapSinkToInventories(ctx, updatedSink)
		Expect(reqs).To(ConsistOf(req))

		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      "demo-target",
			UID:             "uid-map-2",
			Namespace:       ns,
			Name:            "redis",
			Version:         "v1",
			Kind:            "Deployment",
		})

		_, err = reconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2), "sink watch mapping must enqueue inventory re-export")
	})

	It("requeues on optimistic concurrency conflict when marking degraded (EC-P1-05)", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		invName := "degraded-conflict-" + suffix
		ns := "default"

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns, Generation: 1},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				SinkRefs: kollectdevv1alpha1.NewSinkRefList("missing-sink"),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		key := types.NamespacedName{Name: invName, Namespace: ns}
		stale := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, key, stale)).To(Succeed())

		fresh := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, key, fresh)).To(Succeed())
		fresh.Status.ObservedGeneration = 99
		fresh.Status.ItemCount = 1
		Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

		reconciler := &KollectInventoryReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		result, err := reconciler.setInventoryDegraded(
			context.Background(),
			stale,
			0,
			"ConflictTest",
			"simulated degraded path",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Requeue).To(BeTrue(), "Conflict on status update must requeue without error")

		unchanged := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, key, unchanged)).To(Succeed())
		degraded := apimeta.FindStatusCondition(unchanged.Status.Conditions, conditionDegraded)
		Expect(degraded).To(BeNil(), "stale write must not apply degraded condition after conflict")
		Expect(unchanged.Status.ObservedGeneration).To(Equal(int64(99)))
	})
})

var _ = Describe("Phase A envtest — connection test reconcilers", func() {
	BeforeEach(func() {
		if !gitProbeReachable() {
			Skip("git remote probe endpoint unavailable")
		}
	})

	It("marks KollectConnectionTest probe succeeded against a reachable git sink", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		sinkName := "probe-sink-" + suffix
		testName := "probe-test-" + suffix
		ns := "default"

		sinkObj := &kollectdevv1alpha1.KollectSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     "git",
				Endpoint: envtestGitProbeEndpoint,
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, sinkObj) }()

		test := &kollectdevv1alpha1.KollectConnectionTest{
			ObjectMeta: metav1.ObjectMeta{Name: testName, Namespace: ns, Generation: 1},
			Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: sinkName},
		}
		Expect(k8sClient.Create(ctx, test)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, test) }()

		reconciler := &KollectConnectionTestReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Name: testName, Namespace: ns},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectConnectionTest{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testName, Namespace: ns}, updated)).To(Succeed())
		Expect(updated.Status.Completed).To(BeTrue())
		Expect(updated.Status.ObservedGeneration).To(Equal(int64(1)))

		verified := apimeta.FindStatusCondition(updated.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
		Expect(verified).NotTo(BeNil())
		Expect(verified.Status).To(Equal(metav1.ConditionTrue))
		Expect(verified.Reason).To(Equal("ConnectionOK"))
	})

	It("runs connection test on KollectSink reconcile when probe is enabled", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		sinkName := "conn-sink-" + suffix
		ns := "default"

		sinkObj := &kollectdevv1alpha1.KollectSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns, Generation: 1},
			Spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     "git",
				Endpoint: envtestGitProbeEndpoint,
			},
		}
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, sinkObj) }()

		reconciler := &KollectSinkReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Name: sinkName, Namespace: ns},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectSink{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sinkName, Namespace: ns}, updated)).To(Succeed())

		verified := apimeta.FindStatusCondition(updated.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
		Expect(verified).NotTo(BeNil())
		Expect(verified.Status).To(Equal(metav1.ConditionTrue))
		Expect(verified.Reason).To(Equal("ConnectionOK"))
	})
})
