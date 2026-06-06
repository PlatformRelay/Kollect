// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"

	"github.com/konih/kollect/internal/sink/git"
)

// TestConnection verifies TLS to the GitLab remote and optionally runs git ls-remote.
func TestConnection(ctx context.Context, cfg Config, auth git.Auth) error {
	return git.TestConnection(ctx, cfg.GitConfig(), auth)
}
