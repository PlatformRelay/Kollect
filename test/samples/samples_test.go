// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package samples_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

func TestSampleProfilesValidate(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read samples dir: %v", err)
	}

	s := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	for _, ent := range entries {
		if ent.IsDir() || !strings.HasPrefix(ent.Name(), "kollect_v1alpha1_kollectprofile") {
			continue
		}

		path := filepath.Join(root, ent.Name())
		//nolint:gosec // G304: path is under repo config/samples only
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		var profile kollectdevv1alpha1.KollectProfile
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
		if err := decoder.Decode(&profile); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}

		if profile.Kind != "KollectProfile" {
			t.Fatalf("%s: expected KollectProfile, got %s", path, profile.Kind)
		}

		if errs := validation.ValidateProfile(&profile); len(errs) > 0 {
			t.Fatalf("%s: validation failed: %v", path, errs)
		}
	}
}

func TestSampleClusterTargetValidates(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	path := filepath.Join(root, "kollect_v1alpha1_kollectclustertarget.yaml")

	var target kollectdevv1alpha1.KollectClusterTarget
	decodeSample(t, path, &target)

	if errs := validation.ValidateClusterTargetSpec(&target.Spec); len(errs) > 0 {
		t.Fatalf("%s: validation failed: %v", path, errs)
	}
	if target.Spec.ProfileRef.Namespace == "" {
		t.Fatalf("%s: cluster target profileRef.namespace must be set", path)
	}
}

func TestSampleClusterInventoryValidates(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	path := filepath.Join(root, "kollect_v1alpha1_kollectclusterinventory.yaml")

	var inv kollectdevv1alpha1.KollectClusterInventory
	decodeSample(t, path, &inv)

	if errs := validation.ValidateClusterInventorySpec(&inv.Spec); len(errs) > 0 {
		t.Fatalf("%s: validation failed: %v", path, errs)
	}
}

func TestSampleInventoryExportPartitioningValidates(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	path := filepath.Join(root, "kollect_v1alpha1_kollectinventory_export-partitioning.yaml")

	var inv kollectdevv1alpha1.KollectInventory
	decodeSample(t, path, &inv)

	if errs := validation.ValidateInventorySpec(&inv.Spec); len(errs) > 0 {
		t.Fatalf("%s: validation failed: %v", path, errs)
	}

	if inv.Spec.MaxExportBytes == nil {
		t.Fatalf("%s: expected an inventory-wide spec.maxExportBytes ceiling", path)
	}

	var override *int64
	for _, ref := range inv.Spec.SnapshotSinkRefs {
		if ref.MaxExportBytes != nil {
			override = ref.MaxExportBytes
		}
	}
	if override == nil {
		t.Fatalf("%s: expected a snapshot sink ref with a maxExportBytes override", path)
	}

	if got := validation.ResolveBindingMaxExportBytes(override, *inv.Spec.MaxExportBytes); got != *override {
		t.Fatalf("%s: override %d not effective, resolved %d", path, *override, got)
	}
}

func TestSampleSinksValidate(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")

	snapshots := []string{
		"kollect_v1alpha1_kollectsnapshotsink.yaml",
		"kollect_v1alpha1_kollectsnapshotsink_s3.yaml",
	}
	for _, name := range snapshots {
		var sink kollectdevv1alpha1.KollectSnapshotSink
		decodeSample(t, filepath.Join(root, name), &sink)
		if errs := validation.ValidateSnapshotSinkSpec(&sink.Spec); len(errs) > 0 {
			t.Fatalf("%s: validation failed: %v", name, errs)
		}
	}

	var db kollectdevv1alpha1.KollectDatabaseSink
	decodeSample(t, filepath.Join(root, "kollect_v1alpha1_kollectdatabasesink.yaml"), &db)
	if errs := validation.ValidateDatabaseSinkSpec(&db.Spec); len(errs) > 0 {
		t.Fatalf("database sink sample validation failed: %v", errs)
	}

	var bq kollectdevv1alpha1.KollectDatabaseSink
	decodeSample(t, filepath.Join(root, "kollect_v1alpha1_kollectdatabasesink_bigquery.yaml"), &bq)
	if errs := validation.ValidateDatabaseSinkSpec(&bq.Spec); len(errs) > 0 {
		t.Fatalf("bigquery sink sample validation failed: %v", errs)
	}

	for _, name := range []string{
		"kollect_v1alpha1_kollecteventsink_kafka.yaml",
		"kollect_v1alpha1_kollecteventsink_nats.yaml",
	} {
		var ev kollectdevv1alpha1.KollectEventSink
		decodeSample(t, filepath.Join(root, name), &ev)
		if errs := validation.ValidateEventSinkSpec(&ev.Spec); len(errs) > 0 {
			t.Fatalf("%s: validation failed: %v", name, errs)
		}
	}
}

func decodeSample(t *testing.T, path string, into any) {
	t.Helper()
	//nolint:gosec // G304: path is under repo config/samples only
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
	if err := decoder.Decode(into); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestSampleKindsDecode(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	patterns := []string{

		"kollect_v1alpha1_kollecttarget.yaml",
		"kollect_v1alpha1_kollectinventory.yaml",
		"kollect_v1alpha1_kollectdatabasesink.yaml",
		"kollect_v1alpha1_kollectdatabasesink_bigquery.yaml",
		"kollect_v1alpha1_kollectsnapshotsink.yaml",
		"kollect_v1alpha1_kollecteventsink_kafka.yaml",
		"kollect_v1alpha1_kollecteventsink_nats.yaml",
		"kollect_v1alpha1_kollectclustertarget.yaml",
		"kollect_v1alpha1_kollectclusterinventory.yaml",
	}

	for _, name := range patterns {
		//nolint:gosec // G304: path is under repo config/samples only
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}

		kind, _ := raw["kind"].(string)
		if kind == "" {
			t.Fatalf("%s: missing kind", name)
		}
	}
}
