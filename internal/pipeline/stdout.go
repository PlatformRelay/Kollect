// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	sigsyaml "sigs.k8s.io/yaml"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// StdoutSentinel is the --output value that routes the pipeline export to stdout instead
// of a filesystem directory or a configured sink (ADR-0802). A single dash is the
// conventional "write to stdout" token; ResolveSink treats it specially so it is never
// misread as a literal directory named "-".
const StdoutSentinel = "-"

// StdoutSinkType is the synthetic sink type produced for `--output -`. It never reaches a
// real sink.Backend: runOneContext detects it and collects records for stdout emission
// instead of building a network/filesystem backend.
const StdoutSinkType = "stdout"

// StdoutFormat is the encoding for stdout export records (ADR-0802).
type StdoutFormat string

const (
	// FormatNDJSON writes one compact JSON object per line (the default: stream-friendly,
	// greppable, and the shape most log/collection tooling expects).
	FormatNDJSON StdoutFormat = "ndjson"
	// FormatYAML writes a multi-document YAML stream (one `---`-separated document per record).
	FormatYAML StdoutFormat = "yaml"
	// FormatJSON writes all records as one buffered, indented JSON array.
	FormatJSON StdoutFormat = "json"
)

// ParseStdoutFormat validates a --format value and returns the canonical StdoutFormat.
// An empty string resolves to the NDJSON default so callers can pass the raw flag value.
func ParseStdoutFormat(s string) (StdoutFormat, error) {
	switch StdoutFormat(s) {
	case "", FormatNDJSON:
		return FormatNDJSON, nil
	case FormatYAML:
		return FormatYAML, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid --format %q: must be one of ndjson|yaml|json", s)
	}
}

// IsStdoutSink reports whether a resolved sink spec is the synthetic stdout sink.
func IsStdoutSink(sinkSpec kollectdevv1alpha1.KollectSinkSpec) bool {
	return sinkSpec.Type == StdoutSinkType
}

// StdoutRecord is one self-describing target result emitted to stdout. It carries the
// kubecontext, target identity, and the rendered export path alongside the canonical
// versioned export envelope — the same envelope (and therefore the same redaction and
// extraction) a filesystem or database sink would receive, so a stdout trial is a faithful
// preview of what a real sink would store.
type StdoutRecord struct {
	Context         string                 `json:"context"`
	TargetNamespace string                 `json:"targetNamespace"`
	TargetName      string                 `json:"targetName"`
	Path            string                 `json:"path"`
	Envelope        collect.ExportEnvelope `json:"envelope"`
}

// CollectStdoutRecords builds one StdoutRecord per target from an already-collected store,
// mirroring ExportTargets' per-target loop (same path template and cluster resolution) but
// producing structured records for stdout instead of writing to a backend. A per-target
// marshal failure is collected in errs and does not stop the remaining targets, matching the
// partial-failure contract of the filesystem export path.
func CollectStdoutRecords(
	store *collect.Store,
	targets []kollectdevv1alpha1.KollectTarget,
	sinkSpec kollectdevv1alpha1.KollectSinkSpec,
	contextName string,
) (records []StdoutRecord, errs []error) {
	tmpl := sinkSpec.PathTemplate
	if tmpl == "" {
		tmpl = defaultPathTemplate
	}

	cluster := sinkSpec.Cluster
	if cluster == "" {
		cluster = contextName
	}

	for _, target := range targets {
		payload, err := store.MarshalTargetExport(target.Namespace, target.Name, collect.ExportMetadata{Cluster: cluster})
		if err != nil {
			errs = append(errs, fmt.Errorf("target %s/%s: marshal export: %w", target.Namespace, target.Name, err))

			continue
		}

		var env collect.ExportEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			errs = append(errs, fmt.Errorf("target %s/%s: decode envelope: %w", target.Namespace, target.Name, err))

			continue
		}

		records = append(records, StdoutRecord{
			Context:         contextName,
			TargetNamespace: target.Namespace,
			TargetName:      target.Name,
			Path:            renderPath(tmpl, target.Namespace, target.Name, cluster),
			Envelope:        env,
		})
	}

	return records, errs
}

// WriteStdoutRecords encodes records to w in the requested format. NDJSON and YAML stream
// record-by-record (deterministic caller order preserved); JSON buffers a single array. A
// write or marshal error is returned so the caller can map it to a fatal output failure.
func WriteStdoutRecords(w io.Writer, format StdoutFormat, records []StdoutRecord) error {
	switch format {
	case FormatJSON:
		return writeJSONArray(w, records)
	case FormatYAML:
		return writeYAMLStream(w, records)
	default:
		return writeNDJSON(w, records)
	}
}

func writeNDJSON(w io.Writer, records []StdoutRecord) error {
	for i := range records {
		line, err := json.Marshal(records[i])
		if err != nil {
			return fmt.Errorf("marshal ndjson record: %w", err)
		}

		if _, err := w.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
	}

	return nil
}

func writeYAMLStream(w io.Writer, records []StdoutRecord) error {
	for i := range records {
		doc, err := sigsyaml.Marshal(records[i])
		if err != nil {
			return fmt.Errorf("marshal yaml record: %w", err)
		}

		var buf bytes.Buffer
		buf.WriteString("---\n")
		buf.Write(doc)

		if _, err := w.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
	}

	return nil
}

func writeJSONArray(w io.Writer, records []StdoutRecord) error {
	// A nil slice marshals to "null"; emit an empty array instead so `--format json` always
	// produces a valid, parseable array even when nothing was collected.
	if records == nil {
		records = []StdoutRecord{}
	}

	out, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json array: %w", err)
	}

	if _, err := w.Write(append(out, '\n')); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}

	return nil
}
