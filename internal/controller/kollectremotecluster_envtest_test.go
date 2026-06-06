// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"encoding/base64"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/remotecredentials"
	"github.com/konih/kollect/internal/remotesecret"
)

var _ = Describe("KollectRemoteCluster lifecycle (envtest)", func() {
	It("bootstraps credential secret and sets CredentialsVerified", func() {
		suffix := testNameSuffix()
		ns := "remote-" + suffix
		rcName := "spoke-" + suffix

		nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		Expect(k8sClient.Create(ctx, nsObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, nsObj) }()

		yaml, err := remotesecret.GenerateYAML(remotesecret.Options{
			ClusterName: rcName,
			Namespace:   ns,
			APIServer:   "https://spoke.example:6443",
			Token:       "test-token",
			CAData:      base64.StdEncoding.EncodeToString([]byte("ca")),
		})
		Expect(err).NotTo(HaveOccurred())

		marker := "  " + rcName + ": "
		start := strings.Index(yaml, marker)
		Expect(start).To(BeNumerically(">=", 0))
		start += len(marker)
		end := strings.Index(yaml[start:], "\n")
		Expect(end).To(BeNumerically(">=", 0))
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(yaml[start : start+end]))
		Expect(err).NotTo(HaveOccurred())

		secretName := "kollect-remote-secret-" + suffix
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns},
			Data:       map[string][]byte{rcName: raw},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		rc := &kollectdevv1alpha1.KollectRemoteCluster{
			ObjectMeta: metav1.ObjectMeta{Name: rcName, Namespace: ns, Generation: 1},
			Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{
				ClusterName: rcName,
				CredentialsSecretRef: &corev1.LocalObjectReference{
					Name: secretName,
				},
			},
		}
		Expect(createRemoteClusterWithRequiredStatus(ctx, rc)).To(Succeed())
		defer func() {
			Expect(removeKollectRemoteClusterWithFinalizer(ctx, types.NamespacedName{Namespace: ns, Name: rcName}, nil)).To(Succeed())
		}()

		reconciler := &KollectRemoteClusterReconciler{
			Client:     k8sClient,
			Scheme:     k8sClient.Scheme(),
			APIChecker: noopAPIChecker{},
		}

		_, err = reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: rcName},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &kollectdevv1alpha1.KollectRemoteCluster{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: rcName}, updated)).To(Succeed())

		cred := apimeta.FindStatusCondition(updated.Status.Conditions, kollectdevv1alpha1.ConditionCredentialsVerified)
		Expect(cred).NotTo(BeNil())
		Expect(cred.Status).To(Equal(metav1.ConditionTrue))
		Expect(cred.Reason).To(Equal("CredentialsVerified"))

		Expect(remotecredentials.VerifySecret(context.Background(), secret, rcName, noopAPIChecker{})).To(Succeed())
	})

	It("AwaitingFirstReport requeues", func() {
		suffix := testNameSuffix()
		ns := "remote-await-" + suffix
		rcName := "await-" + suffix

		nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		Expect(k8sClient.Create(ctx, nsObj)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, nsObj) }()

		rc := &kollectdevv1alpha1.KollectRemoteCluster{
			ObjectMeta: metav1.ObjectMeta{Name: rcName, Namespace: ns, Generation: 1},
			Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{
				ClusterName: rcName,
			},
		}
		Expect(createRemoteClusterWithRequiredStatus(ctx, rc)).To(Succeed())
		defer func() {
			Expect(removeKollectRemoteClusterWithFinalizer(ctx, types.NamespacedName{Namespace: ns, Name: rcName}, nil)).To(Succeed())
		}()

		reconciler := &KollectRemoteClusterReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: rcName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(remoteClusterRequeueInterval))

		updated := &kollectdevv1alpha1.KollectRemoteCluster{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: rcName}, updated)).To(Succeed())

		connected := apimeta.FindStatusCondition(updated.Status.Conditions, kollectdevv1alpha1.ConditionConnected)
		Expect(connected).NotTo(BeNil())
		Expect(connected.Status).To(Equal(metav1.ConditionFalse))
		Expect(connected.Reason).To(Equal("AwaitingFirstReport"))
	})
})
