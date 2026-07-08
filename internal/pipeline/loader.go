// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package pipeline implements the kubeconfig-based CLI collection mode (ADR-0801):
// reading KollectProfile/KollectTarget/KollectSnapshotSink config from local YAML
// instead of the cluster API, and wiring the one-shot runner to a sink backend.
package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var pipelineScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(pipelineScheme))
	utilruntime.Must(kollectdevv1alpha1.AddToScheme(pipelineScheme))
}

var pipelineCodec = serializer.NewCodecFactory(pipelineScheme).UniversalDeserializer()

// LoadResult holds all typed objects successfully loaded from the config directory.
type LoadResult struct {
	Profiles []kollectdevv1alpha1.KollectProfile
	Targets  []kollectdevv1alpha1.KollectTarget
	Sinks    []kollectdevv1alpha1.KollectSnapshotSink
	Secrets  []corev1.Secret
	Warnings []string
}

// LoadConfig reads all *.yaml / *.yml files from dir (non-recursive) and returns
// typed CRD objects. Returns an error if dir is unreadable, any file is syntactically
// invalid YAML, or any KollectTarget.spec.profileRef is not satisfied within the loaded set.
func LoadConfig(dir string) (LoadResult, error) {
	var result LoadResult

	entries, err := os.ReadDir(dir)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read config dir %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)

		raw, err := os.ReadFile(path) //nolint:gosec // G304: path is built from os.ReadDir(dir) entries, not external input
		if err != nil {
			return LoadResult{}, fmt.Errorf("read %q: %w", path, err)
		}

		if err := decodeDocuments(&result, raw, name); err != nil {
			return LoadResult{}, err
		}
	}

	if err := resolveProfileRefs(&result); err != nil {
		return LoadResult{}, err
	}

	return result, nil
}

// decodeDocuments decodes every YAML document in raw (multi-doc "---" streams supported)
// and dispatches each into result by concrete type. Unknown/unregistered kinds are recorded
// as warnings, not errors (forward-compatibility with newer CRD kinds).
func decodeDocuments(result *LoadResult, raw []byte, filename string) error {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), len(raw))

	for {
		var doc runtime.RawExtension
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("parse %q: %w", filename, err)
		}

		if len(bytes.TrimSpace(doc.Raw)) == 0 {
			continue
		}

		obj, gvk, err := pipelineCodec.Decode(doc.Raw, nil, nil)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("skipped unknown kind %v in %s", gvk, filename))

				continue
			}

			return fmt.Errorf("parse %q: %w", filename, err)
		}

		dispatch(result, obj, filename)
	}
}

func dispatch(result *LoadResult, obj runtime.Object, filename string) {
	switch typed := obj.(type) {
	case *kollectdevv1alpha1.KollectProfile:
		result.Profiles = append(result.Profiles, *typed)
	case *kollectdevv1alpha1.KollectTarget:
		result.Targets = append(result.Targets, *typed)
	case *kollectdevv1alpha1.KollectSnapshotSink:
		result.Sinks = append(result.Sinks, *typed)
	case *corev1.Secret:
		result.Secrets = append(result.Secrets, *typed)
	default:
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("skipped unknown kind %T in %s", obj, filename))
	}
}

func resolveProfileRefs(result *LoadResult) error {
	profileNames := make(map[string]struct{}, len(result.Profiles))
	for _, p := range result.Profiles {
		profileNames[p.Name] = struct{}{}
	}

	for _, t := range result.Targets {
		if _, ok := profileNames[t.Spec.ProfileRef]; !ok {
			return fmt.Errorf("target %q: profileRef %q not found in config directory", t.Name, t.Spec.ProfileRef)
		}
	}

	return nil
}
