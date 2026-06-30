// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konih/kollect/internal/sink/objectstore"
)

var generationPathPattern = regexp.MustCompile(`generation=(\d+)`)

// defaultClusterName is used when neither the envelope nor the sink config name a cluster.
const defaultClusterName = "default"

// CommitContext carries inventory and export metadata for commit message templates (ADR-0415).
type CommitContext struct {
	Namespace  string
	Name       string
	Cluster    string
	Generation int64
	ExportGen  int64
	ItemCount  int
	Checksum   string
	ExportedAt time.Time
	Path       string
	SinkName   string
}

func CommitContextFromObjectPath(objectPath, cluster string) CommitContext {
	ns, name := objectstore.InventoryFromObjectPath(objectPath)
	ctx := CommitContext{
		Namespace: ns,
		Name:      name,
		Cluster:   strings.TrimSpace(cluster),
		Path:      objectPath,
	}

	if ctx.Cluster == "" {
		ctx.Cluster = defaultClusterName
	}

	if matches := generationPathPattern.FindStringSubmatch(objectPath); len(matches) == 2 {
		if gen, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			ctx.Generation = gen
			ctx.ExportGen = gen
		}
	}

	return ctx
}

func checksumShort(checksum string) string {
	checksum = strings.TrimPrefix(strings.TrimSpace(checksum), "sha256:")
	if len(checksum) > 12 {
		return checksum[:12]
	}

	return checksum
}

func exportedAtRFC3339(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(time.RFC3339)
	}

	return t.UTC().Format(time.RFC3339)
}

func renderCommitTemplate(template string, ctx CommitContext) string {
	replacer := strings.NewReplacer(
		"{namespace}", ctx.Namespace,
		"{name}", ctx.Name,
		"{cluster}", ctx.Cluster,
		"{generation}", strconv.FormatInt(ctx.Generation, 10),
		"{exportGeneration}", strconv.FormatInt(ctx.ExportGen, 10),
		"{itemCount}", strconv.Itoa(ctx.ItemCount),
		"{checksum}", ctx.Checksum,
		"{checksumShort}", checksumShort(ctx.Checksum),
		"{exportedAt}", exportedAtRFC3339(ctx.ExportedAt),
		"{sink}", ctx.SinkName,
		"{path}", ctx.Path,
	)

	return replacer.Replace(template)
}

type renderedCommit struct {
	Subject  string
	Body     string
	Trailers []string
	Full     string
}

func renderCommit(cfg Config, ctx CommitContext) renderedCommit {
	subject := renderCommitTemplate(cfg.CommitMessage, ctx)
	body := ""
	if cfg.CommitBody != "" {
		body = renderCommitTemplate(cfg.CommitBody, ctx)
	}

	trailers := make([]string, 0, len(cfg.CommitTrailers))
	for _, trailer := range cfg.CommitTrailers {
		line := strings.TrimSpace(renderCommitTemplate(trailer, ctx))
		if line != "" {
			trailers = append(trailers, line)
		}
	}

	full := subject
	if body != "" {
		full = subject + "\n\n" + body
	}

	for _, line := range trailers {
		full += "\n\n" + line
	}

	return renderedCommit{Subject: subject, Body: body, Trailers: trailers, Full: full}
}

func renderCommitMessage(template string, ctx CommitContext) string {
	return renderCommitTemplate(template, ctx)
}
