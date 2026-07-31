// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	sigsyaml "sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

func TestParseStdoutFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		want    StdoutFormat
		wantErr bool
	}{
		"":       {want: FormatNDJSON},
		"ndjson": {want: FormatNDJSON},
		"yaml":   {want: FormatYAML},
		"json":   {want: FormatJSON},
		"toml":   {wantErr: true},
		"NDJSON": {wantErr: true}, // case-sensitive: the flag help lists lowercase tokens
	}

	for in, want := range cases {
		got, err := ParseStdoutFormat(in)
		if want.wantErr {
			if err == nil {
				t.Errorf("ParseStdoutFormat(%q): expected error, got nil", in)
			}

			continue
		}

		if err != nil {
			t.Errorf("ParseStdoutFormat(%q): unexpected error %v", in, err)
		}

		if got != want.want {
			t.Errorf("ParseStdoutFormat(%q) = %q, want %q", in, got, want.want)
		}
	}
}

func TestResolveSink_stdoutSentinel(t *testing.T) {
	t.Parallel()

	got, err := ResolveSink(LoadResult{}, StdoutSentinel)
	if err != nil {
		t.Fatalf("ResolveSink(-): unexpected error %v", err)
	}

	if !IsStdoutSink(got) {
		t.Fatalf("ResolveSink(-): expected stdout sink, got type %q", got.Type)
	}
}

func TestResolveSink_stdoutWithSinkYAMLIsAmbiguous(t *testing.T) {
	t.Parallel()

	loaded := LoadResult{Sinks: []kollectdevv1alpha1.KollectSnapshotSink{{}}}
	if _, err := ResolveSink(loaded, StdoutSentinel); err == nil {
		t.Fatal("ResolveSink(-) with a Sink YAML: expected ambiguity error, got nil")
	}
}

func TestResolveSink_plainOutputDirUnchanged(t *testing.T) {
	t.Parallel()

	got, err := ResolveSink(LoadResult{}, "/tmp/out")
	if err != nil {
		t.Fatalf("ResolveSink(dir): unexpected error %v", err)
	}

	if got.Type != LocalSinkType || got.Endpoint != "/tmp/out" {
		t.Fatalf("ResolveSink(dir) = %+v, want local sink to /tmp/out", got)
	}
}

// storeWithTwoTargets builds a store holding one item under each of two targets, so record
// collection and encoding can be exercised without a cluster.
func storeWithTwoTargets(t *testing.T) *collect.Store {
	t.Helper()

	s := collect.NewStore()
	s.Upsert(collect.Item{
		TargetNamespace: "default", TargetName: "t1",
		Namespace: "default", Name: "cm-a", Version: "v1", Kind: "ConfigMap", UID: "uid-a",
		Attributes: map[string]any{"k": "v1"},
	})
	s.Upsert(collect.Item{
		TargetNamespace: "team", TargetName: "t2",
		Namespace: "team", Name: "cm-b", Version: "v1", Kind: "ConfigMap", UID: "uid-b",
		Attributes: map[string]any{"k": "v2"},
	})

	return s
}

func twoTargets() []kollectdevv1alpha1.KollectTarget {
	return []kollectdevv1alpha1.KollectTarget{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "t1"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "t2"}},
	}
}

func TestCollectStdoutRecords_carriesIdentityAndEnvelope(t *testing.T) {
	t.Parallel()

	records, errs := CollectStdoutRecords(storeWithTwoTargets(t), twoTargets(),
		kollectdevv1alpha1.KollectSinkSpec{Type: StdoutSinkType}, "prod")
	if len(errs) != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	// Deterministic target order preserved.
	if records[0].TargetName != "t1" || records[1].TargetName != "t2" {
		t.Fatalf("target order not preserved: %q, %q", records[0].TargetName, records[1].TargetName)
	}

	r0 := records[0]
	if r0.Context != "prod" {
		t.Errorf("context = %q, want prod", r0.Context)
	}

	if r0.Envelope.SchemaVersion != collect.ExportSchemaVersion {
		t.Errorf("envelope schemaVersion = %q, want %q", r0.Envelope.SchemaVersion, collect.ExportSchemaVersion)
	}

	if r0.Envelope.ItemCount != 1 || len(r0.Envelope.Items) != 1 {
		t.Errorf("envelope itemCount/items = %d/%d, want 1/1", r0.Envelope.ItemCount, len(r0.Envelope.Items))
	}

	if r0.Envelope.Cluster != "prod" {
		t.Errorf("envelope cluster = %q, want prod (defaults to context)", r0.Envelope.Cluster)
	}

	if r0.Path == "" {
		t.Error("expected a rendered export path, got empty")
	}
}

// decodeRecords parses the three encodings back into records so a round-trip can be asserted.
func decodeRecords(t *testing.T, format StdoutFormat, data []byte) []StdoutRecord {
	t.Helper()

	switch format {
	case FormatNDJSON:
		var out []StdoutRecord
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}

			var rec StdoutRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Fatalf("ndjson line %q: %v", line, err)
			}

			out = append(out, rec)
		}

		return out
	case FormatYAML:
		var out []StdoutRecord
		for _, doc := range strings.Split(string(data), "---\n") {
			if strings.TrimSpace(doc) == "" {
				continue
			}

			var rec StdoutRecord
			if err := sigsyaml.Unmarshal([]byte(doc), &rec); err != nil {
				t.Fatalf("yaml doc %q: %v", doc, err)
			}

			out = append(out, rec)
		}

		return out
	default:
		var out []StdoutRecord
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("json array: %v", err)
		}

		return out
	}
}

func TestWriteStdoutRecords_allFormatsRoundTripToSameRecords(t *testing.T) {
	t.Parallel()

	records, _ := CollectStdoutRecords(storeWithTwoTargets(t), twoTargets(),
		kollectdevv1alpha1.KollectSinkSpec{Type: StdoutSinkType}, "prod")

	// Canonical reference: NDJSON decode.
	var ndjsonBuf bytes.Buffer
	if err := WriteStdoutRecords(&ndjsonBuf, FormatNDJSON, records); err != nil {
		t.Fatal(err)
	}

	nd := decodeRecords(t, FormatNDJSON, ndjsonBuf.Bytes())

	// NDJSON: one object per line (2 records -> 2 non-empty lines).
	if got := strings.Count(strings.TrimRight(ndjsonBuf.String(), "\n"), "\n") + 1; got != 2 {
		t.Errorf("ndjson line count = %d, want 2", got)
	}

	reference, err := json.Marshal(nd)
	if err != nil {
		t.Fatal(err)
	}

	for _, format := range []StdoutFormat{FormatYAML, FormatJSON} {
		var buf bytes.Buffer
		if err := WriteStdoutRecords(&buf, format, records); err != nil {
			t.Fatalf("%s: %v", format, err)
		}

		got := decodeRecords(t, format, buf.Bytes())

		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}

		if string(gotJSON) != string(reference) {
			t.Errorf("%s did not round-trip to the same records:\n got %s\nwant %s", format, gotJSON, reference)
		}
	}
}

func TestWriteStdoutRecords_jsonEmptyIsArrayNotNull(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteStdoutRecords(&buf, FormatJSON, nil); err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("empty json = %q, want []", strings.TrimSpace(buf.String()))
	}
}
