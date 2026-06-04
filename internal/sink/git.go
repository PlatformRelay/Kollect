// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type gitBackend struct {
	endpoint string
}

func newGitBackend(spec kollectdevv1alpha1.KollectSinkSpec) (Backend, error) {
	return &gitBackend{endpoint: spec.Endpoint}, nil
}

func (g *gitBackend) Type() string {
	return "git"
}

func (g *gitBackend) Export(ctx context.Context, payload []byte) error {
	_ = ctx
	_ = payload

	// Phase 1 placeholder: Git export wiring lands in a follow-up change.
	return nil
}
