// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr/funcr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// leakSecretValue is the field value every leak probe plants in the selected object. An
// extraction error may name the attribute and the failure class, never this value.
const leakSecretValue = "SUPERSECRETVALUE"

func leakSecretObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name": "db-creds", "namespace": "team-a", "uid": "uid-secret",
		},
		"data": map[string]any{"pw": leakSecretValue},
		"spec": map[string]any{"items": []any{leakSecretValue}},
	}}
}

// leakProbes are attribute paths whose evaluation fails with an error that, unredacted, echoes
// a selected field value. The first three are the independently reported reproductions.
var leakProbes = []struct {
	name      string
	path      string
	wantClass string
}{
	{name: "cel timestamp parse", path: "cel:timestamp(object.data.pw)", wantClass: "invalid timestamp"},
	{name: "cel map key from data", path: "cel:object.data[object.data.pw]", wantClass: "no such key"},
	{name: "jsonpath filter on map", path: "{.data[?(@.x>1)]}", wantClass: "filter applied to a non-list value"},
	{name: "cel regex from data", path: "cel:'x'.matches(object.data.pw + '(')", wantClass: "invalid regular expression"},
	{name: "cel duration parse", path: "cel:duration(object.data.pw)", wantClass: "type conversion error"},
	{name: "cel index from data", path: "cel:object.spec.items[size(object.data.pw)]", wantClass: "index out of bounds"},
}

// Reproduces the LastExtractionError leak at its source: the error returned by Extract feeds
// status, conditions, events and logs, so it must never carry object-derived text.
func TestExtractEvaluationErrorsNeverEchoObjectValues(t *testing.T) {
	t.Parallel()

	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range leakProbes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, extractErr := ext.Extract(leakSecretObject(), []kollectdevv1alpha1.AttributeSpec{
				{Name: "leaky", Path: tc.path},
			})
			if extractErr == nil {
				t.Fatalf("Extract(%q) error = nil, want an evaluation error", tc.path)
			}

			msg := extractErr.Error()
			if strings.Contains(msg, leakSecretValue) {
				t.Fatalf("Extract(%q) error %q echoes the selected field value", tc.path, msg)
			}
			if !strings.Contains(msg, `attribute "leaky"`) {
				t.Fatalf("Extract(%q) error %q, want it to name the attribute", tc.path, msg)
			}
			if !strings.Contains(msg, tc.wantClass) {
				t.Fatalf("Extract(%q) error %q, want error class %q", tc.path, msg, tc.wantClass)
			}

			// The raw cause must not be recoverable by unwrapping either: anything that
			// formats a wrapped error chain would otherwise re-expose it.
			for e := errors.Unwrap(extractErr); e != nil; e = errors.Unwrap(e) {
				if strings.Contains(e.Error(), leakSecretValue) {
					t.Fatalf("unwrapped error %q echoes the selected field value", e.Error())
				}
			}
		})
	}
}

// Helm release payloads carry chart values; a traversal through a scalar must not echo it.
func TestExtractHelmAccessorErrorNeverEchoesReleaseValues(t *testing.T) {
	t.Parallel()

	release := sampleHelmReleaseJSON()
	release["config"] = map[string]any{"password": leakSecretValue}
	obj, err := helmReleaseSecretObject(release)
	if err != nil {
		t.Fatalf("helmReleaseSecretObject: %v", err)
	}

	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	_, extractErr := ext.Extract(obj, []kollectdevv1alpha1.AttributeSpec{
		{Name: "leaky", Path: "helm:release.config.password.sub"},
	})
	if extractErr == nil {
		t.Fatal("Extract() error = nil, want an accessor error")
	}
	if strings.Contains(extractErr.Error(), leakSecretValue) {
		t.Fatalf("Extract() error %q echoes a Helm release value", extractErr.Error())
	}
}

// Compile and parse errors are derived from the expression alone, never from object data, so
// they keep their full detail: users need it to fix their profile.
func TestExtractCompileErrorsKeepExpressionDetail(t *testing.T) {
	t.Parallel()

	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		want string
	}{
		{path: "cel:1 +", want: "Syntax error"},
		{path: "{.data[", want: "unterminated array"},
	}

	for _, tc := range cases {
		_, extractErr := ext.Extract(leakSecretObject(), []kollectdevv1alpha1.AttributeSpec{
			{Name: "broken", Path: tc.path},
		})
		if extractErr == nil || !strings.Contains(extractErr.Error(), tc.want) {
			t.Fatalf("Extract(%q) error = %v, want detail %q", tc.path, extractErr, tc.want)
		}
	}
}

// syncBuffer is a goroutine-safe log sink for the engine leak test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) write(prefix, args string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.WriteString(prefix + " " + args + "\n")
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// Engine path: the status-feeding ExtractFailures message and the dispatch log line must both
// stay free of selected field values.
func TestEngineDispatchExtractionErrorDoesNotLeakValues(t *testing.T) {
	t.Parallel()

	for _, tc := range leakProbes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ext, err := NewExtractor()
			if err != nil {
				t.Fatal(err)
			}
			rules, err := CompileResourceRules(nil, ext.celEnv)
			if err != nil {
				t.Fatal(err)
			}

			gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
			key := targetKey("team-a", "secrets")
			e := &Engine{
				store:        NewStore(),
				extractor:    ext,
				access:       NewAccessChecker(allowAllAccessClient()),
				forbidden:    make(map[string]struct{}),
				accessErr:    make(map[string]struct{}),
				extractErr:   make(map[string]*extractFailureState),
				nsMeta:       map[string]namespaceMeta{"team-a": {}},
				targets:      make(map[string]targetState),
				targetsByGVR: make(map[schema.GroupVersionResource][]string),
			}
			e.targets[key] = targetState{
				target: kollectdevv1alpha1.KollectTarget{
					ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "secrets"},
				},
				profile: kollectdevv1alpha1.KollectProfile{
					Spec: kollectdevv1alpha1.KollectProfileSpec{
						TargetGVK:  kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"},
						Attributes: []kollectdevv1alpha1.AttributeSpec{{Name: "leaky", Path: tc.path}},
					},
				},
				effectiveNamespaces: map[string]struct{}{"team-a": {}},
				compiledRules:       rules,
			}
			e.targetsByGVR[gvr] = []string{key}

			logs := &syncBuffer{}
			logger := funcr.New(logs.write, funcr.Options{Verbosity: 10})
			ctx := log.IntoContext(context.Background(), logger)

			e.processDispatch(ctx, gvr, leakSecretObject(), false)

			count, lastErr := e.ExtractFailures("team-a", "secrets")
			if count != 1 {
				t.Fatalf("extract failure count = %d, want 1", count)
			}
			if strings.Contains(lastErr, leakSecretValue) {
				t.Fatalf("ExtractFailures message %q echoes the selected field value", lastErr)
			}
			if !strings.Contains(lastErr, `attribute "leaky"`) || !strings.Contains(lastErr, tc.wantClass) {
				t.Fatalf("ExtractFailures message %q, want attribute name and class %q", lastErr, tc.wantClass)
			}

			out := logs.String()
			if !strings.Contains(out, "extract attributes") {
				t.Fatalf("log output %q, want the extraction failure logged", out)
			}
			if strings.Contains(out, leakSecretValue) {
				t.Fatalf("log output echoes the selected field value: %q", out)
			}
		})
	}
}

// One-shot runner path: the ExtractionFailure reason is logged by the pipeline and joined
// into the process exit error.
func TestRunnerExtractionFailureReasonDoesNotLeakValues(t *testing.T) {
	t.Parallel()

	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range leakProbes {
		_, extractErr := ext.Extract(leakSecretObject(), []kollectdevv1alpha1.AttributeSpec{
			{Name: "leaky", Path: tc.path},
		})
		if reason := redactExtractionReason(extractErr); strings.Contains(reason, leakSecretValue) {
			t.Fatalf("%s: ExtractionFailure reason %q echoes the selected field value", tc.name, reason)
		}
	}
}
