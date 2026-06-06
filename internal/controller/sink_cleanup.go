// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink"
)

func cleanupSinkExports(
	ctx context.Context,
	c client.Client,
	registry *sink.Registry,
	sinkNamespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
	clusterFallback bool,
	objectPath string,
	generation int64,
) error {
	if registry == nil || len(bindings) == 0 {
		return nil
	}

	var errs []error
	for _, binding := range bindings {
		var (
			resolved *sink.ResolvedSink
			err      error
		)
		if clusterFallback {
			resolved, err = loadClusterInventorySink(ctx, c, sinkNamespace, binding)
		} else {
			resolved, err = loadResolvedSink(ctx, c, sinkNamespace, binding, false)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := sink.RunExportItems(sink.ExportItemsRequest{
			Ctx:           ctx,
			Client:        c,
			Registry:      registry,
			SinkNamespace: sink.SinkNamespaceForResolved(resolved, sinkNamespace),
			SinkName:      binding.Name,
			SinkFamily:    binding.Family,
			ObjectPath:    objectPath,
			Items:         []collect.Item{},
			Meta:          export.Metadata{Generation: generation},
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
