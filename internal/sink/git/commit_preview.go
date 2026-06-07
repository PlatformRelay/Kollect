// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// RenderCommitPreview renders commit subject/body templates without a live git remote (ADR-0416).
func RenderCommitPreview(spec kollectdevv1alpha1.KollectSinkSpec, ctx CommitContext) (subject, body string) {
	cfg := Config{
		CommitMessage: defaultCommitMessage,
		Author: CommitAuthor{
			Name:  defaultAuthorName,
			Email: defaultAuthorEmail,
		},
	}

	if spec.Git != nil {
		if msg := strings.TrimSpace(spec.Git.CommitMessage); msg != "" {
			cfg.CommitMessage = msg
		}
		cfg.CommitBody = strings.TrimSpace(spec.Git.CommitBody)
		cfg.CommitTrailers = append([]string(nil), spec.Git.CommitTrailers...)
	}

	rendered := renderCommit(cfg.withDefaults(), ctx)
	return rendered.Subject, rendered.Body
}
