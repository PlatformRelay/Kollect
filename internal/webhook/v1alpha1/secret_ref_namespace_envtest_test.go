// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
)

// K-04 test lock: a cross-namespace SecretReference on a namespaced family sink
// is rejected at admission through the real API server + validating webhook when
// the operator allowlist is empty, and admitted when the referenced namespace is
// allowlisted. Driven end-to-end rather than calling the validator directly, so
// the webhook wiring (not just the helper) is covered.
var _ = Describe("Webhook cross-namespace secretRef admission (envtest)", func() {
	AfterEach(func() {
		validation.SetAllowedSecretRefNamespaces(nil)
	})

	It("denies a snapshot sink whose secretRef names another namespace", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())

		sink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: "xns-denied-" + suffix, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					SecretRef: &kollectdevv1alpha1.SecretReference{
						Name:      "victim-creds",
						Namespace: "other",
					},
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
		}

		err := webhookClient.Create(webhookCtx, sink)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsForbidden(err)).To(BeTrue())
		// The rejection must name the offending field, not merely fail.
		Expect(err.Error()).To(ContainSubstring("spec.secretRef.namespace"))
	})

	It("admits a snapshot sink whose secretRef namespace is allowlisted", func() {
		validation.SetAllowedSecretRefNamespaces([]string{"other"})

		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		sink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: "xns-allowed-" + suffix, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					SecretRef: &kollectdevv1alpha1.SecretReference{
						Name:      "shared-creds",
						Namespace: "other",
					},
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
		}

		Expect(webhookClient.Create(webhookCtx, sink)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, sink) }()
	})

	It("admits a same-namespace secretRef with the allowlist empty", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		sink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: "xns-same-" + suffix, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "own-creds"},
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
		}

		Expect(webhookClient.Create(webhookCtx, sink)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, sink) }()
	})
})
