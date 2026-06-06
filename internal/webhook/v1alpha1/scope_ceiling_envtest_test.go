// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var _ = Describe("Webhook scope ceiling (envtest)", func() {
	It("denies KollectTarget when profile GVK is outside KollectScope allowlist", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		testNS := "webhook-scope-" + suffix

		Expect(webhookClient.Create(webhookCtx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: testNS},
		})).To(Succeed())
		defer func() {
			_ = webhookClient.Delete(webhookCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}})
		}()

		teamScope := &kollectdevv1alpha1.KollectScope{
			ObjectMeta: metav1.ObjectMeta{Name: "team-scope-" + suffix, Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectScopeSpec{
				ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
					AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
						{Group: "apps", Version: "v1", Kind: "Deployment"},
					},
					AllowedNamespaces: []string{testNS},
				},
			},
		}
		Expect(webhookClient.Create(webhookCtx, teamScope)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, teamScope) }()

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
		Expect(webhookClient.Create(webhookCtx, profile)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, profile) }()

		target := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "denied-helm-" + suffix, Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				ProfileRef: "helm-release-summary",
			},
		}
		err := webhookClient.Create(webhookCtx, target)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsForbidden(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("allowedGVKs"))
	})

	It("accepts KollectTarget when profile GVK is within scope", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		testNS := "webhook-ok-" + suffix

		Expect(webhookClient.Create(webhookCtx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: testNS},
		})).To(Succeed())
		defer func() {
			_ = webhookClient.Delete(webhookCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}})
		}()

		teamScope := &kollectdevv1alpha1.KollectScope{
			ObjectMeta: metav1.ObjectMeta{Name: "team-scope-" + suffix, Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectScopeSpec{
				ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
					AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
						{Group: "apps", Version: "v1", Kind: "Deployment"},
					},
					AllowedNamespaces: []string{testNS},
				},
			},
		}
		Expect(webhookClient.Create(webhookCtx, teamScope)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, teamScope) }()

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{
					Group: "apps", Version: "v1", Kind: "Deployment",
				},
			},
		}
		Expect(webhookClient.Create(webhookCtx, profile)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, profile) }()

		target := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "ok-deploy-" + suffix, Namespace: testNS},
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				ProfileRef: "deployments",
				CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
					IncludedNamespaces: []string{testNS},
				},
			},
		}
		Expect(webhookClient.Create(webhookCtx, target)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, target) }()
	})
})
