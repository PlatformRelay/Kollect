// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

var _ = Describe("KollectClusterInventory Controller", func() {
	const tenantLabel = "kollect.dev/tenant"

	var (
		targetName     string
		profileName    string
		inventoryName  string
		nsMatched      string
		tenantLabelVal string
		engineCtx      context.Context
		engineCancel   context.CancelFunc
		engine         *collect.Engine
		kubeClient     kubernetes.Interface
	)

	BeforeEach(func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		targetName = "rollup-target-" + suffix
		profileName = "rollup-profile-" + suffix
		inventoryName = "platform-rollup-" + suffix
		nsMatched = "cluster-rollup-a-" + suffix
		tenantLabelVal = "rollup-" + suffix

		var err error
		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dyn, err := dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		store := collect.NewStore()
		engine, err = collect.NewEngine(dyn, kubeClient, store, collect.EngineConfig{})
		Expect(err).NotTo(HaveOccurred())

		engineCtx, engineCancel = context.WithCancel(ctx)
		Expect(engine.Start(engineCtx)).To(Succeed())
	})

	AfterEach(func() {
		if engineCancel != nil {
			engineCancel()
		}

		Expect(removeKollectClusterInventoryWithFinalizer(ctx, inventoryName, nil, nil)).To(Succeed())
		Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, engine)).To(Succeed())
		_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: sink.DefaultSecretNamespace},
		})
		deleteNamespaceBestEffort(ctx, kubeClient, nsMatched)
	})

	It("rolls up Ready cluster target status and collected item counts", func() {
		ensureNamespace(ctx, kubeClient, nsMatched, map[string]string{tenantLabel: tenantLabelVal})

		var err error

		for _, name := range []string{"rollup-cm-1", "rollup-cm-2", "rollup-cm-3"} {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsMatched},
				Data:       map[string]string{"key": name},
			}
			_, err = kubeClient.CoreV1().ConfigMaps(nsMatched).Create(ctx, cm, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}

		ensureNamespace(ctx, kubeClient, sink.DefaultSecretNamespace, nil)

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: sink.DefaultSecretNamespace},
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				Attributes: []kollectdevv1alpha1.AttributeSpec{
					{Name: "name", Path: "{.metadata.name}"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, profile)).To(Succeed())

		target := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetName},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				ProfileRef: profileName,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		targetReconciler := &KollectClusterTargetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}

		_, err = targetReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			return engine.ItemCount(nsMatched, targetName)
		}, 30*time.Second, 200*time.Millisecond).Should(Equal(3))

		_, err = targetReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		inventory := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: inventoryName},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inventory)).To(Succeed())

		invReconciler := &KollectClusterInventoryReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}

		_, err = invReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: inventoryName},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectClusterInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: inventoryName}, updated)).To(Succeed())

		ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal("RolledUp"))
		Expect(updated.Status.TargetCount).To(Equal(1))
		Expect(updated.Status.ItemCount).To(Equal(3))
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
	})
})
