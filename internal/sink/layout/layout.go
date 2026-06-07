// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// DefaultDocumentPathTemplate is the single-file path for git/gitlab document mode (ADR-0419).
const DefaultDocumentPathTemplate = "inventory/{namespace}/{name}{extension}"

// File is one rendered repo file in a projected layout.
type File struct {
	// Path is the repo-relative slash path.
	Path string
	// Data is the encoded file content.
	Data []byte
}

// ResolveInput carries the inputs needed to resolve effective layout config (ADR-0419).
type ResolveInput struct {
	// Spec is the normalized sink spec (provides type, layout, serialization, pathTemplate, cluster).
	Spec kollectdevv1alpha1.KollectSinkSpec

	// InventoryNamespace / InventoryName identify the exported inventory.
	InventoryNamespace string
	InventoryName      string

	// Generation is the export generation for {generation} placeholders.
	Generation int64

	// ResourceExportMode reports whether the referenced profile uses export.mode: Resource
	// (ADR-0306). When true, layout auto-upgrades to perResource + manifest content unless the
	// author set those fields explicitly. The controller populates this once ADR-0306 lands; it is
	// false today, so layout behaves from explicit config only.
	ResourceExportMode bool

	// ManifestKey is the Item.attributes key holding the embedded object (ADR-0306 export.as).
	ManifestKey string
}

// ResolvedLayout is the fully-defaulted layout configuration used to project files.
type ResolvedLayout struct {
	Mode    string
	Content string
	Format  string

	Extension            string
	DocumentPathTemplate string
	PathTemplate         string

	Cluster            string
	InventoryNamespace string
	InventoryName      string
	Generation         int64

	ManifestKey string

	IndexEnabled      bool
	IndexPathTemplate string

	// ExportedAt is an optional RFC3339 timestamp embedded in the split-mode index. Left empty by
	// Resolve so projection stays pure; the export pipeline sets it from envelope metadata.
	ExportedAt string

	// Prune reports whether stale files should be removed on export (ADR-0419: auto for
	// perResource/split). The git backend honors this when writing the tree.
	Prune bool

	Filename *kollectdevv1alpha1.LayoutFilenameSpec
}

// IsDocument reports whether the resolved layout writes a single inventory file.
func (r ResolvedLayout) IsDocument() bool {
	return r.Mode == kollectdevv1alpha1.LayoutModeDocument
}

// Resolve computes the effective layout from a sink spec and convention-over-configuration rules.
func Resolve(in ResolveInput) ResolvedLayout {
	layoutSpec := in.Spec.Layout
	format := kollectdevv1alpha1.EffectiveSerializationFormat(&in.Spec)

	mode := layoutSpec.ModeOrDefault()
	if in.ResourceExportMode && !layoutSpec.ModeExplicit() {
		mode = kollectdevv1alpha1.LayoutModePerResource
	}

	content := kollectdevv1alpha1.LayoutContentItem
	if layoutSpec != nil && layoutSpec.Content != "" {
		content = layoutSpec.Content
	} else if in.ResourceExportMode && mode != kollectdevv1alpha1.LayoutModeDocument {
		content = kollectdevv1alpha1.LayoutContentManifest
	}

	docTemplate := strings.TrimSpace(in.Spec.PathTemplate)
	if docTemplate == "" {
		docTemplate = DefaultDocumentPathTemplate
	}

	itemTemplate := kollectdevv1alpha1.DefaultLayoutPathTemplate
	if layoutSpec != nil && strings.TrimSpace(layoutSpec.PathTemplate) != "" {
		itemTemplate = strings.TrimSpace(layoutSpec.PathTemplate)
	}

	indexTemplate := kollectdevv1alpha1.DefaultLayoutIndexPathTemplate
	if layoutSpec != nil && layoutSpec.Index != nil && strings.TrimSpace(layoutSpec.Index.PathTemplate) != "" {
		indexTemplate = strings.TrimSpace(layoutSpec.Index.PathTemplate)
	}

	var filename *kollectdevv1alpha1.LayoutFilenameSpec
	if layoutSpec != nil {
		filename = layoutSpec.Filename
	}

	manifestKey := strings.TrimSpace(in.ManifestKey)
	if manifestKey == "" {
		manifestKey = kollectdevv1alpha1.DefaultExportAs
	}

	return ResolvedLayout{
		Mode:                 mode,
		Content:              content,
		Format:               format,
		Extension:            ExtensionForFormat(format),
		DocumentPathTemplate: docTemplate,
		PathTemplate:         itemTemplate,
		Cluster:              strings.TrimSpace(in.Spec.Cluster),
		InventoryNamespace:   in.InventoryNamespace,
		InventoryName:        in.InventoryName,
		Generation:           in.Generation,
		ManifestKey:          manifestKey,
		IndexEnabled:         layoutSpec.IndexEnabled(),
		IndexPathTemplate:    indexTemplate,
		Prune:                mode != kollectdevv1alpha1.LayoutModeDocument,
		Filename:             filename,
	}
}

// DocumentPath renders the single-file document path for the resolved layout.
func (r ResolvedLayout) DocumentPath() string {
	return renderInventoryPath(r.DocumentPathTemplate, r)
}

// IndexPath renders the split-mode index sidecar path.
func (r ResolvedLayout) IndexPath() string {
	return renderInventoryPath(r.IndexPathTemplate, r)
}

func renderInventoryPath(template string, r ResolvedLayout) string {
	repl := strings.NewReplacer(
		"{cluster}", clusterOrDefault(r.Cluster),
		"{namespace}", r.InventoryNamespace,
		"{name}", r.InventoryName,
		"{generation}", fmt.Sprintf("%d", r.Generation),
		"{extension}", r.Extension,
	)

	return repl.Replace(template)
}

func clusterOrDefault(cluster string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return "default"
	}

	return cluster
}

// Project renders the ordered file set for items under a resolved layout (ADR-0419).
// Two rows rendering the same path is a terminal error (no silent overwrite).
func Project(items []collect.Item, r ResolvedLayout) ([]File, error) {
	switch r.Mode {
	case kollectdevv1alpha1.LayoutModePerResource:
		return projectPerResource(items, r)
	case kollectdevv1alpha1.LayoutModeSplit:
		return projectSplit(items, r)
	default:
		return projectDocument(items, r)
	}
}

func projectDocument(items []collect.Item, r ResolvedLayout) ([]File, error) {
	data, err := marshalDocument(items, r.Format)
	if err != nil {
		return nil, err
	}

	return []File{{Path: r.DocumentPath(), Data: data}}, nil
}

func projectPerResource(items []collect.Item, r ResolvedLayout) ([]File, error) {
	files := make([]File, 0, len(items))
	seen := make(map[string]string, len(items))

	for i := range items {
		item := items[i]
		path, err := renderItemPath(item, r)
		if err != nil {
			return nil, err
		}

		if prev, ok := seen[path]; ok {
			return nil, fmt.Errorf("layout collision: %q and %q both render path %q", prev, itemRef(item), path)
		}
		seen[path] = itemRef(item)

		payload, err := itemPayload(item, r.Content, r.ManifestKey)
		if err != nil {
			return nil, err
		}

		data, err := marshalValue(payload, r.Format)
		if err != nil {
			return nil, err
		}

		files = append(files, File{Path: path, Data: data})
	}

	return files, nil
}

func projectSplit(items []collect.Item, r ResolvedLayout) ([]File, error) {
	files, err := projectPerResource(items, r)
	if err != nil {
		return nil, err
	}

	if !r.IndexEnabled {
		return files, nil
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	index := buildIndex(r, items, paths)
	data, err := marshalValue(index, r.Format)
	if err != nil {
		return nil, err
	}

	indexPath := r.IndexPath()
	for _, f := range files {
		if f.Path == indexPath {
			return nil, fmt.Errorf("layout collision: index path %q collides with a resource file", indexPath)
		}
	}

	return append([]File{{Path: indexPath, Data: data}}, files...), nil
}

func itemRef(item collect.Item) string {
	return strings.TrimSpace(item.Kind + "/" + item.Namespace + "/" + item.Name)
}
