// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/metrics"
)

const storeCoalesceDelay = 50 * time.Millisecond

type inventoryNamespaceSource struct {
	store  *collect.Store
	reader client.Reader
}

func newInventoryStoreSource(store *collect.Store, reader client.Reader) source.TypedSource[reconcile.Request] {
	return &inventoryNamespaceSource{store: store, reader: reader}
}

func (s *inventoryNamespaceSource) Start(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	if s == nil || s.store == nil || s.reader == nil {
		<-ctx.Done()

		return nil
	}

	ch := s.store.SubscribeNamespaces()
	defer s.store.UnsubscribeNamespaces(ch)

	var (
		mu      sync.Mutex
		pending = make(map[string]struct{})
	)

	flush := func() {
		mu.Lock()
		namespaces := pending
		pending = make(map[string]struct{})
		mu.Unlock()

		for ns := range namespaces {
			for _, req := range inventoriesInNamespace(ctx, s.reader, ns) {
				queue.Add(req)
			}
		}
	}

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-ctx.Done():
			timer.Stop()

			return nil
		case ns := <-ch:
			mu.Lock()
			pending[ns] = struct{}{}
			mu.Unlock()
			timer.Reset(storeCoalesceDelay)
		case <-timer.C:
			flush()
		}
	}
}

func inventoriesInNamespace(
	ctx context.Context,
	reader client.Reader,
	namespace string,
) []reconcile.Request {
	var list kollectdevv1alpha1.KollectInventoryList
	if err := reader.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list inventories for store watch",
			"namespace", namespace)
		metrics.WatchMapListErrorsTotal.WithLabelValues("KollectInventory", "store").Inc()

		return nil
	}

	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
		})
	}

	return reqs
}
