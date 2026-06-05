// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import "context"

// WireClusterIDKey is the Redis stream / NATS metadata field for spoke cluster identity (ADR-0028).
const WireClusterIDKey = "cluster_id"

type wireClusterKey struct{}

// WithWireClusterID attaches spoke cluster identity to a publish context for queue transports.
func WithWireClusterID(ctx context.Context, clusterID string) context.Context {
	if clusterID == "" {
		return ctx
	}

	return context.WithValue(ctx, wireClusterKey{}, clusterID)
}

// WireClusterID returns the cluster id from ctx when set by WithWireClusterID.
func WireClusterID(ctx context.Context) string {
	v, _ := ctx.Value(wireClusterKey{}).(string)

	return v
}
