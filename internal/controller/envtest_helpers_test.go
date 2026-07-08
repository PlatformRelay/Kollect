// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

func testNameSuffix() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func newPostgresRecordingRegistry(recorder sink.Backend) *sink.Registry {
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return recorder, nil
	})

	return reg
}

func newPostgresRecordingRegistryWithSelector(
	selectBackend func(kollectdevv1alpha1.KollectSinkSpec) (sink.Backend, error),
) *sink.Registry {
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		spec kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return selectBackend(spec)
	})

	return reg
}

func createPostgresSinkFixtures(sinkName, secretName, ns string) (*kollectdevv1alpha1.KollectDatabaseSink, *corev1.Secret) {
	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: ns},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: secretName},
				Table:       "inventory_items",
			},
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	return sinkObj, pgSecret
}

// listFailInventoryClient injects List errors for KollectInventoryList (map-handler tests).
type listFailInventoryClient struct {
	client.Client
	listErr error
}

func (c *listFailInventoryClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	if c.listErr != nil {
		if _, ok := list.(*kollectdevv1alpha1.KollectInventoryList); ok {
			return c.listErr
		}
	}

	return c.Client.List(ctx, list, opts...)
}

func newListFailInventoryClient(base client.Client, listErr error) client.Client {
	if listErr == nil {
		listErr = errors.New("simulated inventory list failure")
	}

	return &listFailInventoryClient{Client: base, listErr: listErr}
}

// listFailTargetClient injects List errors for KollectTargetList (map-handler tests).
type listFailTargetClient struct {
	client.Client
	listErr error
}

func (c *listFailTargetClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	if c.listErr != nil {
		if _, ok := list.(*kollectdevv1alpha1.KollectTargetList); ok {
			return c.listErr
		}
	}

	return c.Client.List(ctx, list, opts...)
}

func newListFailTargetClient(base client.Client, listErr error) client.Client {
	if listErr == nil {
		listErr = errors.New("simulated target list failure")
	}

	return &listFailTargetClient{Client: base, listErr: listErr}
}

// listFailClusterTargetClient injects List errors for KollectClusterTargetList (map-handler tests).
type listFailClusterTargetClient struct {
	client.Client
	listErr error
}

func (c *listFailClusterTargetClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	if c.listErr != nil {
		if _, ok := list.(*kollectdevv1alpha1.KollectClusterTargetList); ok {
			return c.listErr
		}
	}

	return c.Client.List(ctx, list, opts...)
}

func newListFailClusterTargetClient(base client.Client, listErr error) client.Client {
	if listErr == nil {
		listErr = errors.New("simulated cluster target list failure")
	}

	return &listFailClusterTargetClient{Client: base, listErr: listErr}
}

func removeKollectTargetWithFinalizer(ctx context.Context, key types.NamespacedName) error {
	target := &kollectdevv1alpha1.KollectTarget{}
	if err := k8sClient.Get(ctx, key, target); err != nil {
		return client.IgnoreNotFound(err)
	}

	if target.DeletionTimestamp.IsZero() {
		if err := k8sClient.Delete(ctx, target); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	rec := &KollectTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, key, target); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		if _, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
			return err
		}
	}

	return errors.New("timed out waiting for KollectTarget deletion")
}

func removeKollectClusterTargetWithFinalizer(ctx context.Context, name string, engine *collect.Engine) error {
	key := types.NamespacedName{Name: name}
	target := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := k8sClient.Get(ctx, key, target); err != nil {
		return client.IgnoreNotFound(err)
	}

	if target.DeletionTimestamp.IsZero() {
		if err := k8sClient.Delete(ctx, target); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	rec := &KollectClusterTargetReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
		Engine: engine,
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, key, target); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		if _, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
			return err
		}
	}

	return errors.New("timed out waiting for KollectClusterTarget deletion")
}

func removeKollectClusterInventoryWithFinalizer(
	ctx context.Context,
	name string,
	store *collect.Store,
	registry *sink.Registry,
) error {
	key := types.NamespacedName{Name: name}
	inv := &kollectdevv1alpha1.KollectClusterInventory{}
	if err := k8sClient.Get(ctx, key, inv); err != nil {
		return client.IgnoreNotFound(err)
	}

	if inv.DeletionTimestamp.IsZero() {
		if err := k8sClient.Delete(ctx, inv); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	rec := &KollectClusterInventoryReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Store:    store,
		Registry: registry,
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, key, inv); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		if _, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
			return err
		}
	}

	return errors.New("timed out waiting for KollectClusterInventory deletion")
}
