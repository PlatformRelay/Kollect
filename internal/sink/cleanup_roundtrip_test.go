// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/objectstore"
)

// exportedObjectPaths runs the real export path derivation for one inventory:
// the controller's canonical identity, export.PartitionObjectPath per part,
// then RunExportEnvelope against an object-store stub. It returns every path
// the export actually wrote, so cleanup tests assert against the real layout
// rather than a hand-written one.
func exportedObjectPaths(t *testing.T, spec kollectdevv1alpha1.KollectSinkSpec, sinkName, ns, name string, parts int) []string {
	t.Helper()

	stub := &stubBackend{caps: ObjectStoreSnapshotCapabilities()}
	reg := NewRegistry()
	reg.Register(spec.Type, func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) {
		return stub, nil
	})
	t.Cleanup(func() { EvictBackendPool("team-a", sinkName) })

	canonical := "inventory/" + ns + "/" + name + ".json"

	var written []string
	for part := 1; part <= parts; part++ {
		envelope, err := export.MarshalEnvelope(
			[]collect.Item{{Name: "item", Attributes: map[string]any{"part": part}}},
			export.Metadata{Generation: 7, PartIndex: part, PartTotal: parts},
		)
		if err != nil {
			t.Fatal(err)
		}

		paths, err := RunExportEnvelope(ExportEnvelopeRequest{
			Ctx:           t.Context(),
			Registry:      reg,
			SinkNamespace: "team-a",
			SinkName:      sinkName,
			ObjectPath:    export.PartitionObjectPath(canonical, part, parts),
			Envelope:      envelope,
			SinkSpec:      spec,
		})
		if err != nil {
			t.Fatalf("RunExportEnvelope part %d: %v", part, err)
		}
		written = append(written, paths...)
	}
	if len(written) == 0 {
		t.Fatal("export wrote nothing")
	}

	return written
}

func cleanupMatchesAny(spec kollectdevv1alpha1.KollectSinkSpec, ns, name, key string) bool {
	for _, candidate := range cleanupCandidatePaths(spec, ns, name, 7) {
		for _, m := range objectstore.CleanupMatchers(candidate) {
			if m.Matches(key) {
				return true
			}
		}
	}

	return false
}

// MR-01 round trip: every object a single- or multipart object-store export
// writes must be addressed by the deletion-time cleanup of that inventory, and
// a sibling inventory whose name merely extends it must never be.
func TestCleanupMatchers_RoundTripObjectStoreExports(t *testing.T) {
	t.Parallel()

	parquet := kollectdevv1alpha1.KollectSinkSpec{
		Type:        kollectdevv1alpha1.SnapshotSinkTypeS3,
		Cluster:     "prod",
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{Format: objectstore.FormatParquet},
	}
	jsonDefault := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeS3}
	jsonStemTemplate := kollectdevv1alpha1.KollectSinkSpec{
		Type:         kollectdevv1alpha1.SnapshotSinkTypeS3,
		PathTemplate: "snapshots/{cluster}/{namespace}/{name}.json",
	}

	cases := map[string]struct {
		spec  kollectdevv1alpha1.KollectSinkSpec
		parts int
	}{
		"parquet single part":     {parquet, 1},
		"parquet multipart":       {parquet, 3},
		"json default multipart":  {jsonDefault, 3},
		"json template multipart": {jsonStemTemplate, 2},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sinkName := "roundtrip-" + name
			written := exportedObjectPaths(t, tc.spec, sinkName, "team-a", "apps", tc.parts)
			for _, key := range written {
				if !cleanupMatchesAny(tc.spec, "team-a", "apps", key) {
					t.Errorf("exported %q is not addressed by the cleanup of team-a/apps", key)
				}
			}

			sibling := exportedObjectPaths(t, tc.spec, sinkName+"-sibling", "team-a", "appsX", tc.parts)
			for _, key := range sibling {
				if cleanupMatchesAny(tc.spec, "team-a", "apps", key) {
					t.Errorf("sibling inventory object %q must not be addressed by the cleanup of team-a/apps", key)
				}
			}
		})
	}
}
