// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"errors"
	"strings"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func resetJetStreamFromConn() {
	jetStreamFromConn = func(nc *natsgo.Conn) (jetstream.JetStream, error) {
		return jetstream.New(nc)
	}
}

func TestJetStream_returnsCachedWhenAlive(t *testing.T) {

	fjs := &fakeJetStream{}
	b := &Backend{
		cfg:              Config{Stream: "s", Subject: "sub"},
		js:               fjs,
		connectionClosed: func() bool { return false },
	}

	got, err := b.jetStream(context.Background())
	if err != nil {
		t.Fatalf("jetStream: %v", err)
	}
	if got != fjs {
		t.Fatal("expected cached JetStream instance")
	}
}

func TestJetStream_dropsStaleCacheAndRedials(t *testing.T) {

	stale := &fakeJetStream{}
	b := &Backend{
		cfg:              Config{URL: "nats://127.0.0.1:1", Stream: "s", Subject: "sub"},
		js:               stale,
		connectionClosed: func() bool { return true },
		connectFn: func(Config, TLSConfig) (*natsgo.Conn, error) {
			return nil, errors.New("dial refused")
		},
	}

	_, err := b.jetStream(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dial refused") {
		t.Fatalf("error = %v, want dial refused after stale cache drop", err)
	}
	if b.js != nil {
		t.Fatal("expected stale js cache to be cleared before redial")
	}
}

func TestJetStream_wrapsJetStreamNewError(t *testing.T) {

	t.Cleanup(resetJetStreamFromConn)
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) {
		return nil, errors.New("js init boom")
	}

	b := &Backend{
		cfg: Config{URL: "nats://broker:4222", Stream: "s", Subject: "sub"},
		connectFn: func(Config, TLSConfig) (*natsgo.Conn, error) {
			return nil, nil
		},
	}

	_, err := b.jetStream(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nats jetstream:") {
		t.Fatalf("error = %v, want wrapped jetstream init failure", err)
	}
}

func TestJetStream_wrapsEnsureStreamError(t *testing.T) {

	t.Cleanup(resetJetStreamFromConn)
	sentinel := errors.New("stream boom")
	fjs := &fakeJetStream{
		createFunc: func(context.Context, jetstream.StreamConfig) (jetstream.Stream, error) {
			return nil, sentinel
		},
	}
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) {
		return fjs, nil
	}

	b := &Backend{
		cfg: Config{URL: "nats://broker:4222", Stream: "s", Subject: "sub"},
		connectFn: func(Config, TLSConfig) (*natsgo.Conn, error) {
			return nil, nil
		},
	}

	_, err := b.jetStream(context.Background())
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want ensure stream failure", err)
	}
}

func TestJetStream_cachesAfterSuccessfulDial(t *testing.T) {

	t.Cleanup(resetJetStreamFromConn)
	fjs := &fakeJetStream{}
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) {
		return fjs, nil
	}
	dials := 0
	b := &Backend{
		cfg: Config{URL: "nats://broker:4222", Stream: "events", Subject: "inventory.events"},
		connectFn: func(Config, TLSConfig) (*natsgo.Conn, error) {
			dials++
			return nil, nil
		},
		connectionClosed: func() bool { return false },
	}

	first, err := b.jetStream(context.Background())
	if err != nil {
		t.Fatalf("first jetStream: %v", err)
	}
	second, err := b.jetStream(context.Background())
	if err != nil {
		t.Fatalf("second jetStream: %v", err)
	}
	if first != second || dials != 1 {
		t.Fatalf("cache miss: dials=%d first=%p second=%p", dials, first, second)
	}
}

func TestCachedConnDead_nilConnIsDead(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	if !b.cachedConnDead() {
		t.Fatal("nil nc should be dead")
	}
}
