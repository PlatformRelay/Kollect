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
	sinkRefs []kollectdevv1alpha1.InventorySinkRef,
	objectPath string,
	generation int64,
) error {
	if registry == nil || len(sinkRefs) == 0 {
		return nil
	}

	var errs []error
	for _, ref := range sinkRefs {
		if err := sink.RunExportItems(sink.ExportItemsRequest{
			Ctx:           ctx,
			Client:        c,
			Registry:      registry,
			SinkNamespace: sinkNamespace,
			SinkName:      ref.Name,
			ObjectPath:    objectPath,
			Items:         []collect.Item{},
			Meta:          export.Metadata{Generation: generation},
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
