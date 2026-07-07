// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var _ = Describe("Envtest cache-backed client", func() {
	It("serves reads from the informer cache after direct-client writes", func() {
		suffix := testNameSuffix()
		ns := "cache-client-" + suffix
		invName := "cache-inv-" + suffix

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		defer func() {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}()

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: ns},
			Spec:       kollectdevv1alpha1.KollectInventorySpec{},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, inv) }()

		Eventually(func(g Gomega) {
			got := &kollectdevv1alpha1.KollectInventory{}
			g.Expect(mapperEnvtestClient().Get(ctx, types.NamespacedName{Name: invName, Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Name).To(Equal(invName))
		}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	})
})
