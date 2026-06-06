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

var _ = Describe("KollectClusterTarget Controller", func() {
	const tenantLabel = "kollect.dev/tenant"

	var (
		targetName   string
		profileName  string
		nsMatched    string
		nsOther      string
		tenantValue  string
		engineCtx    context.Context
		engineCancel context.CancelFunc
		engine       *collect.Engine
		kubeClient   kubernetes.Interface
	)

	BeforeEach(func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		targetName = "cluster-cm-target-" + suffix
		profileName = "cluster-cm-profile-" + suffix
		nsMatched = "cluster-collect-a-" + suffix
		nsOther = "cluster-collect-b-" + suffix
		tenantValue = "alpha-" + suffix

		var err error
		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dyn, err := dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		store := collect.NewStore()
		engine, err = collect.NewEngine(dyn, kubeClient, store)
		Expect(err).NotTo(HaveOccurred())

		engineCtx, engineCancel = context.WithCancel(ctx)
		Expect(engine.Start(engineCtx)).To(Succeed())
	})

	AfterEach(func() {
		if engineCancel != nil {
			engineCancel()
		}

		_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetName},
		})
		_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: sink.DefaultSecretNamespace},
		})
		deleteNamespaceBestEffort(ctx, kubeClient, nsMatched)
		deleteNamespaceBestEffort(ctx, kubeClient, nsOther)
	})

	It("wires namespaceSelector matches to the collection engine end-to-end", func() {
		ensureNamespace(ctx, kubeClient, nsMatched, map[string]string{tenantLabel: tenantValue})
		ensureNamespace(ctx, kubeClient, nsOther, nil)

		var err error

		for _, name := range []string{"cm-one", "cm-two"} {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsMatched},
				Data:       map[string]string{"name": name},
			}
			_, err = kubeClient.CoreV1().ConfigMaps(nsMatched).Create(ctx, cm, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}

		ignored := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm-ignored", Namespace: nsOther},
			Data:       map[string]string{"skip": "true"},
		}
		_, err = kubeClient.CoreV1().ConfigMaps(nsOther).Create(ctx, ignored, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

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
					MatchLabels: map[string]string{tenantLabel: tenantValue},
				},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		reconciler := &KollectClusterTargetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			return engine.ItemCount(nsMatched, targetName)
		}, 30*time.Second, 200*time.Millisecond).Should(Equal(2))

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectClusterTarget{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: targetName}, updated)).To(Succeed())

		ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal("Collecting"))
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))

		Expect(engine.NamespacesForClusterTarget(targetName)).To(ConsistOf(nsMatched))
	})

	It("re-enqueues targets when cluster profile changes", func() {
		profile := &kollectdevv1alpha1.KollectClusterProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName},
			Spec: kollectdevv1alpha1.KollectClusterProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			},
		}
		Expect(k8sClient.Create(ctx, profile)).To(Succeed())

		targetA := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetName},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				ProfileRef: profileName,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantValue},
				},
			},
		}
		Expect(k8sClient.Create(ctx, targetA)).To(Succeed())

		targetBName := "cluster-cm-target-b-" + testNameSuffix()
		targetB := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetBName},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				ProfileRef: profileName,
			},
		}
		Expect(k8sClient.Create(ctx, targetB)).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectClusterTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetBName},
			})
		}()

		reconciler := &KollectClusterTargetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}

		reqs := reconciler.mapClusterProfileToClusterTargets(ctx, profile)
		Expect(reqs).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{Name: targetName}},
			reconcile.Request{NamespacedName: types.NamespacedName{Name: targetBName}},
		))
	})
})
