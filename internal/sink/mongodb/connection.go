// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const probeTimeout = 15 * time.Second

// TestConnection pings the MongoDB deployment.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) error {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return err
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	client, err := mongo.Connect(probeCtx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return fmt.Errorf("mongodb connect: %w", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(probeCtx, nil); err != nil {
		return fmt.Errorf("mongodb ping: %w", err)
	}

	return nil
}
