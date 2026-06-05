// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var _ = Describe("KollectTarget scope enforcement", func() {
	It("degrades target when profile GVK is outside KollectScope allowlist", func() {
		const testNS = "default"

		teamScope := &kollectdevv1alpha1.KollectScope{
			ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope", Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectScopeSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
				AllowedNamespaces: []string{testNS},
				SinkRefs:          []string{"demo-git"},
			},
		}
		Expect(k8sClient.Create(ctx, teamScope)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, teamScope) }()

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "helm-release-summary", Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{
					Group:   "helm.toolkit.fluxcd.io",
					Version: "v2",
					Kind:    "HelmRelease",
				},
			},
		}
		Expect(k8sClient.Create(ctx, profile)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, profile) }()

		target := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "denied-helm", Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				ProfileRef: "helm-release-summary",
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, target) }()

		reconciler := &KollectTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "denied-helm", Namespace: testNS},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectTarget{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "denied-helm", Namespace: testNS}, updated)).To(Succeed())

		deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
		Expect(deg).NotTo(BeNil())
		Expect(deg.Reason).To(Equal(scopeReasonGVKDenied))
		Expect(deg.Message).To(ContainSubstring("allowedGVKs"))
	})
})
