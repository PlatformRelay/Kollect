// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestCertManagerCertificateSummaryProfile_contract(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("testdata", "cert_manager_certificate.json")
	//nolint:gosec // G304: fixture path is fixed under package testdata
	raw, readErr := os.ReadFile(fixture)
	if readErr != nil {
		t.Fatalf("read fixture: %v", readErr)
	}

	var obj unstructured.Unstructured
	if unmarshalErr := json.Unmarshal(raw, &obj.Object); unmarshalErr != nil {
		t.Fatalf("decode fixture: %v", unmarshalErr)
	}

	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found || len(conditions) < 1 {
		t.Fatalf("status.conditions: found=%v len=%d err=%v", found, len(conditions), err)
	}

	ready, ok := conditions[0].(map[string]any)
	if !ok {
		t.Fatal("conditions[0] is not an object")
	}

	if ready["type"] != "Ready" {
		t.Fatalf("fixture orders Ready first: conditions[0].type=%v", ready["type"])
	}

	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "commonName", Path: "$.spec.commonName", Type: "string", Optional: true},
		{Name: "secretName", Path: "$.spec.secretName", Type: "string", Optional: true},
		{Name: "issuer", Path: "$.spec.issuerRef.name", Type: "string", Optional: true},
		{Name: "issuerKind", Path: "$.spec.issuerRef.kind", Type: "string", Optional: true},
		{Name: "primaryDNS", Path: "$.spec.dnsNames[0]", Type: "string", Optional: true},
		{Name: "notAfter", Path: "$.status.notAfter", Type: "string", Optional: true},
		{Name: "revision", Path: "$.status.revision", Type: "int", Optional: true},
		{Name: "ready", Path: "$.status.conditions[0].status", Type: "string", Optional: true},
	}

	ext, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got, err := ext.Extract(&obj, attrs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got["commonName"] != "example.com" {
		t.Fatalf("commonName = %v, want example.com", got["commonName"])
	}

	if got["secretName"] != "example-tls-secret" {
		t.Fatalf("secretName = %v, want example-tls-secret", got["secretName"])
	}

	if got["issuer"] != "letsencrypt-prod" {
		t.Fatalf("issuer = %v, want letsencrypt-prod", got["issuer"])
	}

	if got["issuerKind"] != "ClusterIssuer" {
		t.Fatalf("issuerKind = %v, want ClusterIssuer", got["issuerKind"])
	}

	if got["primaryDNS"] != "example.com" {
		t.Fatalf("primaryDNS = %v, want example.com", got["primaryDNS"])
	}

	if got["notAfter"] != "2026-07-01T00:00:00Z" {
		t.Fatalf("notAfter = %v, want 2026-07-01T00:00:00Z", got["notAfter"])
	}

	if got["revision"] != int64(2) && got["revision"] != float64(2) && got["revision"] != 2 {
		t.Fatalf("revision = %v (%T), want 2", got["revision"], got["revision"])
	}

	if got["ready"] != "True" {
		t.Fatalf("ready = %v, want True from conditions[0].status", got["ready"])
	}
}
