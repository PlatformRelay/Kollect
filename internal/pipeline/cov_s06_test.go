// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// --- loader.go: cluster-free error/skip branches (COV-90-S06) ---

// TestLoadConfig_subdirectoriesAreSkipped locks the entry.IsDir() branch: a nested
// directory under the config dir must be skipped, not descended into or read as a file.
func TestLoadConfig_subdirectoriesAreSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	// A file inside the subdir that would fail to parse if it were ever read.
	if err := os.WriteFile(filepath.Join(dir, "nested", "garbage.yaml"), []byte("::not yaml::"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v; subdirectory contents must not be read", err)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings when a subdir is skipped, got %v", result.Warnings)
	}
}

// TestLoadConfig_emptyDocumentsAreSkipped locks the empty-document continue branch:
// a stream of bare "---" separators yields no objects and no warnings.
func TestLoadConfig_emptyDocumentsAreSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte("---\n---\n\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Profiles)+len(result.Targets)+len(result.Sinks)+len(result.Secrets) != 0 {
		t.Errorf("expected zero objects from empty documents, got %+v", result)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("empty documents must not produce warnings, got %v", result.Warnings)
	}
}

// TestLoadConfig_registeredKindWithBadTypedContentErrors locks the decode-error branch
// that is NOT a not-registered error: a known GVK whose body fails typed decoding is a
// hard configuration error (not a forward-compat warning).
func TestLoadConfig_registeredKindWithBadTypedContentErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// ConfigMap is registered, but metadata as an array cannot decode into ObjectMeta.
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata: [1, 2, 3]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected a typed-decode error for a registered kind with a malformed body, got nil")
	}
	if !strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("error should name the offending file, got: %v", err)
	}
}

// TestLoadConfig_registeredButUnhandledKindWarns locks the dispatch default branch: a
// kind that decodes to a concrete Go type the pipeline does not consume (e.g. ConfigMap)
// is recorded as a warning, not silently dropped and not an error.
func TestLoadConfig_registeredButUnhandledKindWarns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cm.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  a: b\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "ConfigMap") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning for the registered-but-unhandled ConfigMap kind, got %v", result.Warnings)
	}
}

// --- wire.go: RunAllContexts / runOneContext fatal + no-panic edges (COV-90-S06) ---

// TestRunAllContexts_emptyContextsIsNoPanicEmptyResult is the EDGE case: an empty context
// list must degrade to an empty result slice without panicking or dialing anything.
func TestRunAllContexts_emptyContextsIsNoPanicEmptyResult(t *testing.T) {
	t.Parallel()

	results := RunAllContexts(context.Background(), nil, "",
		LoadResult{}, kollectdevv1alpha1.KollectSinkSpec{}, nil, sink.NewRegistry(), nil, false)

	if len(results) != 0 {
		t.Fatalf("expected zero results for zero contexts, got %d", len(results))
	}
}

// TestRunAllContexts_nonexistentContextIsFatalNotPanic locks the runOneContext fatal
// branch reached before any client is built: a context name absent from the kubeconfig
// fails at REST-config construction and is recorded as a per-context Fatal error. No
// network dial happens (the failure precedes client creation), so the test stays hermetic.
func TestRunAllContexts_nonexistentContextIsFatalNotPanic(t *testing.T) {
	t.Parallel()

	kubeconfig := writeMultiClusterFixtureKubeconfig(t)

	results := RunAllContexts(context.Background(), []string{"does-not-exist"}, kubeconfig,
		LoadResult{}, kollectdevv1alpha1.KollectSinkSpec{}, nil, sink.NewRegistry(), nil, false)

	if len(results) != 1 {
		t.Fatalf("expected one result per context, got %d", len(results))
	}
	if results[0].Fatal == nil {
		t.Fatalf("expected a Fatal error for a nonexistent context, got %+v", results[0])
	}
	if results[0].Context != "does-not-exist" {
		t.Errorf("Context = %q, want does-not-exist", results[0].Context)
	}
	if !strings.Contains(results[0].Fatal.Error(), "does-not-exist") {
		t.Errorf("Fatal should name the context, got: %v", results[0].Fatal)
	}
}

// TestRunAllContexts_oneFatalDoesNotStopOthers locks ADR-0801: a fatal error in one
// context must not short-circuit the sequential run of the remaining contexts. Both
// named contexts are absent from the kubeconfig, so each fails independently at
// REST-config construction (still hermetic — no dial).
func TestRunAllContexts_oneFatalDoesNotStopOthers(t *testing.T) {
	t.Parallel()

	kubeconfig := writeMultiClusterFixtureKubeconfig(t)

	results := RunAllContexts(context.Background(), []string{"missing-a", "missing-b"}, kubeconfig,
		LoadResult{}, kollectdevv1alpha1.KollectSinkSpec{}, nil, sink.NewRegistry(), nil, false)

	if len(results) != 2 {
		t.Fatalf("expected both contexts to be attempted, got %d results", len(results))
	}
	for i, want := range []string{"missing-a", "missing-b"} {
		if results[i].Context != want {
			t.Errorf("results[%d].Context = %q, want %q", i, results[i].Context, want)
		}
		if results[i].Fatal == nil {
			t.Errorf("results[%d] expected Fatal, got %+v", i, results[i])
		}
	}
}

// --- wire.go: ResolveSinkSecretData namespace-mismatch skip (COV-90-S06) ---

// TestResolveSinkSecretData_namespaceMismatchSkipsSecret locks the continue branch that
// rejects a name-matched Secret whose namespace differs from the SecretRef's: the manifest
// must be treated as not-found, never silently used across namespaces.
func TestResolveSinkSecretData_namespaceMismatchSkipsSecret(t *testing.T) {
	t.Parallel()

	secret := corev1.Secret{}
	secret.Name = "sink-creds"
	secret.Namespace = "other-ns"
	secret.Data = map[string][]byte{"token": []byte("shh")}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "sink-creds", Namespace: "expected-ns"},
	}

	_, err := ResolveSinkSecretData(spec, []corev1.Secret{secret})
	if err == nil {
		t.Fatal("expected not-found error when the only matching Secret is in a different namespace, got nil")
	}
	if !strings.Contains(err.Error(), "sink-creds") {
		t.Errorf("error should name the unresolved secret, got: %v", err)
	}
}

// --- stdout.go: writer output-failure branches degrade gracefully (COV-90-S06) ---

// failWriter fails every Write with a fixed error, simulating a broken stdout pipe.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// TestWriteStdoutRecords_writeErrorSurfacesPerFormat is the EDGE case for the output side:
// when the sink writer fails mid-stream, every format must return a wrapped error rather
// than panic or silently drop the failure. Uses a non-empty record set so each writer
// reaches its w.Write call.
func TestWriteStdoutRecords_writeErrorSurfacesPerFormat(t *testing.T) {
	t.Parallel()

	records, _ := CollectStdoutRecords(storeWithTwoTargets(t), twoTargets(),
		kollectdevv1alpha1.KollectSinkSpec{Type: StdoutSinkType}, "prod")
	if len(records) == 0 {
		t.Fatal("precondition: expected records to write")
	}

	for _, format := range []StdoutFormat{FormatNDJSON, FormatYAML, FormatJSON} {
		err := WriteStdoutRecords(failWriter{}, format, records)
		if err == nil {
			t.Errorf("%s: expected a write error to surface, got nil", format)

			continue
		}
		if !strings.Contains(err.Error(), "write stdout") {
			t.Errorf("%s: error = %q, want it wrapped with %q", format, err.Error(), "write stdout")
		}
	}
}

// TestWriteStdoutRecords_largeRecordSetDoesNotPanic is the EDGE case for oversized input:
// a large record set must stream through every format without panicking (the pipeline sets
// no size cap, so "oversized" reduces to "large input is handled").
func TestWriteStdoutRecords_largeRecordSetDoesNotPanic(t *testing.T) {
	t.Parallel()

	records, _ := CollectStdoutRecords(storeWithTwoTargets(t), twoTargets(),
		kollectdevv1alpha1.KollectSinkSpec{Type: StdoutSinkType}, "prod")

	large := make([]StdoutRecord, 0, 20000)
	for len(large) < 20000 {
		large = append(large, records...)
	}

	for _, format := range []StdoutFormat{FormatNDJSON, FormatYAML, FormatJSON} {
		var buf bytes.Buffer
		if err := WriteStdoutRecords(&buf, format, large); err != nil {
			t.Errorf("%s: large record set returned error: %v", format, err)
		}
		if buf.Len() == 0 {
			t.Errorf("%s: expected non-empty output for a large record set", format)
		}
	}
}
