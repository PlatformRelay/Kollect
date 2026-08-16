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

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
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
		engine, err = collect.NewEngine(dyn, kubeClient, store, collect.EngineConfig{})
		Expect(err).NotTo(HaveOccurred())

		engineCtx, engineCancel = context.WithCancel(ctx)
		Expect(engine.Start(engineCtx)).To(Succeed())
	})

	AfterEach(func() {
		if engineCancel != nil {
			engineCancel()
		}

		Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, engine)).To(Succeed())
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
				ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
					Name:      profileName,
					Namespace: sink.DefaultSecretNamespace,
				},
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

	// Review finding F-E. syncEngineTargets refreshes the engine's namespace metadata
	// cache before registering, and that call is a compensating control this lane's own
	// change created: RegisterTarget no longer refreshes when the caller supplies
	// EffectiveNamespaces, and this controller always does. Nothing else on the
	// cluster-target path populates that cache — it resolves its own namespaces from the
	// cached client, which does not even carry annotations.
	//
	// The cache is what ShouldCollect reads to honour `kollect.dev/namespace-watch:
	// disabled`. Drop the refresh and the annotation becomes invisible: the namespace is
	// still registered and its objects are collected anyway. That is over-collection with
	// no error anywhere — an operator's explicit opt-out silently ignored — so assert the
	// observable outcome rather than the cache contents.
	It("honours a namespace watch opt-out for a cluster target", func() {
		ensureNamespace(ctx, kubeClient, nsMatched, map[string]string{tenantLabel: tenantValue})

		// Same tenant label, so this namespace IS in the target's scope and DOES get a
		// synthetic target registered. Only the annotation should keep its objects out —
		// and annotations reach the engine exclusively through the namespace cache.
		optedOut := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        nsOther,
				Labels:      map[string]string{tenantLabel: tenantValue},
				Annotations: map[string]string{kollectdevv1alpha1.AnnotationNamespaceWatch: kollectdevv1alpha1.WatchValueDisabled},
			},
		}
		_, err := kubeClient.CoreV1().Namespaces().Create(ctx, optedOut, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		for ns, name := range map[string]string{nsMatched: "cm-collected", nsOther: "cm-opted-out"} {
			_, err = kubeClient.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Data:       map[string]string{"name": name},
			}, metav1.CreateOptions{})
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
				ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
					Name:      profileName,
					Namespace: sink.DefaultSecretNamespace,
				},
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

		// Both namespaces are registered. Without this, a zero count below could mean
		// "never in scope" rather than "opt-out honoured", and the test would pass for
		// the wrong reason.
		Expect(engine.NamespacesForClusterTarget(targetName)).To(ConsistOf(nsMatched, nsOther))

		// Positive control: the pipeline demonstrably works and has drained the informer's
		// initial Adds, so the opted-out namespace has had its chance to be collected.
		Eventually(func() int {
			return engine.ItemCount(nsMatched, targetName)
		}, 30*time.Second, 200*time.Millisecond).Should(Equal(1))

		Consistently(func() int {
			return engine.ItemCount(nsOther, targetName)
		}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
			"the namespace watch opt-out must be honoured; a non-zero count means the "+
				"engine's namespace cache was never refreshed and the annotation was invisible")
	})

	It("re-enqueues targets when the referenced profile changes", func() {
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

		profileRef := kollectdevv1alpha1.NamespacedObjectReference{
			Name:      profileName,
			Namespace: sink.DefaultSecretNamespace,
		}

		targetA := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetName},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				ProfileRef: profileRef,
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
				ProfileRef: profileRef,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantValue},
				},
			},
		}
		Expect(k8sClient.Create(ctx, targetB)).To(Succeed())
		defer func() {
			Expect(removeKollectClusterTargetWithFinalizer(ctx, targetBName, engine)).To(Succeed())
		}()

		reconciler := &KollectClusterTargetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}

		reqs := reconciler.mapProfileToClusterTargets(ctx, profile)
		Expect(reqs).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{Name: targetName}},
			reconcile.Request{NamespacedName: types.NamespacedName{Name: targetBName}},
		))
	})
})
