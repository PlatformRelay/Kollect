//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// mongoVerificationText is in the driver's error when the server certificate does
// not chain to a trusted root.
const mongoVerificationText = "certificate signed by unknown authority"

// mongoExportAndProbe runs the connection probe, builds a backend for uri with caPEM
// and exports one item. It returns the number of exported documents found (-1 when
// the build or export failed), the probe error and the build-or-export error.
func mongoExportAndProbe(t *testing.T, uri string, caPEM []byte) (docs int64, probeErr, exportErr error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// A short server selection keeps the refused case bounded; the driver retries
	// a failed handshake until then.
	uri += "&serverSelectionTimeoutMS=5000"
	spec := defaultMongoSpec("inventory", "ca_trust_items", "test-cluster")
	secret := map[string][]byte{"uri": []byte(uri)}

	probeErr = TestConnection(ctx, spec, secret, caPEM)

	backend, err := NewBackend(ctx, spec, secret, caPEM)
	if err != nil {
		return -1, probeErr, err
	}
	defer backend.Close()

	payload, err := json.Marshal([]collect.Item{{
		TargetNamespace: "apps", TargetName: "web", Namespace: "apps", Name: "demo",
		Version: "v1", Kind: "Deployment", UID: "uid-ca",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if exportErr = backend.Export(ctx, payload, "inventory/apps/demo.json"); exportErr != nil {
		return -1, probeErr, exportErr
	}

	docs, err = backend.client.Database("inventory").Collection("ca_trust_items").
		CountDocuments(ctx, bson.M{"source_uid": "uid-ca"})
	if err != nil {
		t.Fatalf("count exported documents: %v", err)
	}

	return docs, probeErr, nil
}

// MI1: a server signed only by private CA X is trusted with CA X and tls=true.
func TestMongoCATrust_MI1_privateCATrusted(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	srv := sinktlstest.StartTLSMongo(t, caX.Server(t), caX.PEM)

	docs, probeErr, exportErr := mongoExportAndProbe(t, srv.URI("tls=true"), caX.PEM)
	if probeErr != nil || exportErr != nil || docs != 1 {
		t.Fatalf("server signed by X, CA X: probe %v, export %v, documents %d; want success and 1", probeErr, exportErr, docs)
	}
}

// MI2: a server signed by Y is refused with CA X on probe and build.
func TestMongoCATrust_MI2_unknownCARefused(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	srv := sinktlstest.StartTLSMongo(t, caY.Server(t), caY.PEM)

	_, probeErr, exportErr := mongoExportAndProbe(t, srv.URI("tls=true"), caX.PEM)
	for name, err := range map[string]error{"probe": probeErr, "build": exportErr} {
		if err == nil {
			t.Errorf("server signed by Y, CA X: %s succeeded; want a certificate verification error", name)
			continue
		}

		if !strings.Contains(err.Error(), mongoVerificationText) {
			t.Errorf("server signed by Y, CA X: %s error %q does not say %q", name, err, mongoVerificationText)
		}
	}
}

// MI3: the CA is appended to the system roots. The child runs with root P as the
// only system root, against a server signed by P, with CA X configured.
func TestMongoCATrust_MI3_keepsSystemRoots(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	rootP := sinktlstest.NewCA(t, "P")
	srv := sinktlstest.StartTLSMongo(t, rootP.Server(t), rootP.PEM)

	sinktlstest.RunWithSystemRoots(t, "TestMongoCATrustChild_MI3", rootP.PEM,
		"MONGO_CA_TRUST_URI="+srv.URI("tls=true"),
		"MONGO_CA_TRUST_CA="+string(caX.PEM),
	)
}

func TestMongoCATrustChild_MI3(t *testing.T) {
	if !sinktlstest.InChild() {
		t.Skip("child of TestMongoCATrust_MI3_keepsSystemRoots only")
	}

	docs, probeErr, exportErr := mongoExportAndProbe(t, os.Getenv("MONGO_CA_TRUST_URI"), []byte(os.Getenv("MONGO_CA_TRUST_CA")))
	if probeErr != nil || exportErr != nil || docs != 1 {
		t.Fatalf("server signed by system root P, CA X: probe %v, export %v, documents %d; want success and 1",
			probeErr, exportErr, docs)
	}
}
