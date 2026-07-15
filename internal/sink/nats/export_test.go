// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/platformrelay/kollect/internal/digest"
	"github.com/platformrelay/kollect/internal/export"
)

// fakeJetStream implements only the JetStream methods the Backend calls. Any
// other method panics (nil embedded interface) which is fine for these tests.
type fakeJetStream struct {
	jetstream.JetStream

	publishFunc func(ctx context.Context, subj string, data []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
	createFunc  func(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)

	lastSubject string
	lastData    []byte
	lastOptLen  int
	lastCfg     jetstream.StreamConfig
}

func (f *fakeJetStream) Publish(
	ctx context.Context,
	subj string,
	data []byte,
	opts ...jetstream.PublishOpt,
) (*jetstream.PubAck, error) {
	f.lastSubject = subj
	f.lastData = append([]byte(nil), data...)
	f.lastOptLen = len(opts)
	if f.publishFunc != nil {
		return f.publishFunc(ctx, subj, data, opts...)
	}
	return &jetstream.PubAck{}, nil
}

func (f *fakeJetStream) CreateOrUpdateStream(
	ctx context.Context,
	cfg jetstream.StreamConfig,
) (jetstream.Stream, error) {
	f.lastCfg = cfg
	if f.createFunc != nil {
		return f.createFunc(ctx, cfg)
	}
	return nil, nil
}

func TestExport_emptyPayloadDoesNotCallProvider(t *testing.T) {
	t.Parallel()

	called := false
	b := &Backend{
		cfg: Config{Cluster: "local", Subject: "inventory.events"},
		jsProvider: func(context.Context) (jetstream.JetStream, error) {
			called = true
			return nil, nil
		},
	}

	err := b.Export(context.Background(), nil, "inventory/default/inv.json")
	if err == nil || !strings.Contains(err.Error(), "nats export: empty payload") {
		t.Fatalf("expected empty payload error, got %v", err)
	}
	if called {
		t.Fatal("provider was called for an empty payload")
	}
}

func TestExport_publishesEnvelopeWithSubjectBodyAndMsgID(t *testing.T) {
	t.Parallel()

	fjs := &fakeJetStream{}
	b := &Backend{
		cfg:        Config{Cluster: "prod", Subject: "inventory.events"},
		jsProvider: func(context.Context) (jetstream.JetStream, error) { return fjs, nil },
	}

	payload := []byte(`[{"uid":"u1"}]`)
	if err := b.Export(context.Background(), payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if fjs.lastSubject != "inventory.events" {
		t.Fatalf("subject = %q, want inventory.events", fjs.lastSubject)
	}
	// Exactly one publish option (WithMsgID). PublishOpt is opaque, so we assert
	// the count here and verify msgID determinism via the msgID helper test.
	if fjs.lastOptLen != 1 {
		t.Fatalf("publish opt count = %d, want 1", fjs.lastOptLen)
	}

	var env EventEnvelope
	if err := json.Unmarshal(fjs.lastData, &env); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if env.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", env.SchemaVersion, export.SchemaVersion)
	}
	if env.Cluster != "prod" {
		t.Fatalf("cluster = %q, want prod", env.Cluster)
	}
	if env.Namespace != "apps" {
		t.Fatalf("namespace = %q, want apps", env.Namespace)
	}
	if string(env.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", env.Payload, payload)
	}
}

func TestExport_wrapsPublishError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	fjs := &fakeJetStream{
		publishFunc: func(context.Context, string, []byte, ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
			return nil, sentinel
		},
	}
	b := &Backend{
		cfg:        Config{Cluster: "prod", Subject: "inventory.events"},
		jsProvider: func(context.Context) (jetstream.JetStream, error) { return fjs, nil },
	}

	err := b.Export(context.Background(), []byte(`{}`), "inventory/apps/x.json")
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "nats publish:") {
		t.Fatalf("error %q missing 'nats publish:' prefix", err.Error())
	}
}

func TestExport_propagatesProviderError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("connect failed")
	b := &Backend{
		cfg: Config{Cluster: "prod", Subject: "inventory.events"},
		jsProvider: func(context.Context) (jetstream.JetStream, error) {
			return nil, sentinel
		},
	}

	err := b.Export(context.Background(), []byte(`{}`), "inventory/apps/x.json")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestMsgID_deterministic(t *testing.T) {
	t.Parallel()

	payload := []byte(`[{"uid":"u1"}]`)
	got := msgID("prod", "apps", payload)
	want := digest.ContentHash(append([]byte("prod/apps/"), payload...))
	if got != want {
		t.Fatalf("msgID = %q, want %q", got, want)
	}
	// Deterministic across calls.
	if again := msgID("prod", "apps", payload); again != got {
		t.Fatalf("msgID not deterministic: %q != %q", again, got)
	}
	// Different inputs yield different IDs.
	if msgID("prod", "apps", []byte("other")) == got {
		t.Fatal("msgID collided for different payloads")
	}
}

func TestEnsureStream_usesConfigAndWrapsError(t *testing.T) {
	t.Parallel()

	fjs := &fakeJetStream{}
	cfg := Config{Stream: "kollect_events", Subject: "inventory.events"}

	if err := ensureStream(context.Background(), fjs, cfg); err != nil {
		t.Fatalf("ensureStream: %v", err)
	}
	if fjs.lastCfg.Name != "kollect_events" {
		t.Fatalf("stream name = %q, want kollect_events", fjs.lastCfg.Name)
	}
	if len(fjs.lastCfg.Subjects) != 1 || fjs.lastCfg.Subjects[0] != "inventory.events" {
		t.Fatalf("subjects = %v, want [inventory.events]", fjs.lastCfg.Subjects)
	}

	sentinel := errors.New("create boom")
	failing := &fakeJetStream{
		createFunc: func(context.Context, jetstream.StreamConfig) (jetstream.Stream, error) {
			return nil, sentinel
		},
	}
	err := ensureStream(context.Background(), failing, cfg)
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "nats create stream:") {
		t.Fatalf("error %q missing 'nats create stream:' prefix", err.Error())
	}
}

func TestStreamSubjects_nonEmpty(t *testing.T) {
	t.Parallel()

	got := streamSubjects("  inventory.events  ")
	if len(got) != 1 || got[0] != "inventory.events" {
		t.Fatalf("streamSubjects = %v, want [inventory.events]", got)
	}
}
