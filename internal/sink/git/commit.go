// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/konih/kollect/internal/sink/objectstore"
)

var generationPathPattern = regexp.MustCompile(`generation=(\d+)`)

type CommitContext struct {
	Namespace  string
	Name       string
	Cluster    string
	Generation int64
}

func CommitContextFromObjectPath(objectPath, cluster string) CommitContext {
	ns, name := objectstore.InventoryFromObjectPath(objectPath)
	ctx := CommitContext{
		Namespace: ns,
		Name:      name,
		Cluster:   strings.TrimSpace(cluster),
	}

	if ctx.Cluster == "" {
		ctx.Cluster = "default"
	}

	if matches := generationPathPattern.FindStringSubmatch(objectPath); len(matches) == 2 {
		if gen, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			ctx.Generation = gen
		}
	}

	return ctx
}

func renderCommitMessage(template string, ctx CommitContext) string {
	replacer := strings.NewReplacer(
		"{namespace}", ctx.Namespace,
		"{name}", ctx.Name,
		"{cluster}", ctx.Cluster,
		"{generation}", strconv.FormatInt(ctx.Generation, 10),
	)

	return replacer.Replace(template)
}
