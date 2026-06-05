// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestMarshalEnvelopeAndFingerprint(t *testing.T) {
	t.Parallel()

	items := []collect.Item{{
		Namespace: "apps",
		Name:      "web",
		UID:       "uid-1",
		Version:   "v1",
		Kind:      "Deployment",
	}}
	meta := Metadata{Generation: 2, Cluster: "spoke-a", ExportedAt: time.Unix(1, 0).UTC()}

	payload, err := MarshalEnvelope(items, meta)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ItemsFromPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("items = %#v", got)
	}

	fp1, err := ItemsFingerprint(items)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := ItemsFingerprint(items)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("fingerprint = %q", fp1)
	}
}

func TestValidateEnvelopeSchemaVersion(t *testing.T) {
	t.Parallel()

	if err := ValidateEnvelopeSchemaVersion(SchemaVersion); err != nil {
		t.Fatal(err)
	}

	if err := ValidateEnvelopeSchemaVersion("kollect.dev/v99"); err == nil {
		t.Fatal("expected unsupported schemaVersion error")
	}
}

func TestItemsFromPayloadLegacyArray(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal([]collect.Item{{
		Namespace: "apps",
		Name:      "web",
		UID:       "uid-1",
		Version:   "v1",
		Kind:      "Deployment",
	}})
	if err != nil {
		t.Fatal(err)
	}

	items, err := ItemsFromPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d", len(items))
	}
}
