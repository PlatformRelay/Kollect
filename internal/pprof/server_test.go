// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pprof

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	return port
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx // test probe against local ephemeral listener
		if err == nil {
			_ = resp.Body.Close()

			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server at %s did not become ready", url)
}

func TestServerNeedLeaderElection(t *testing.T) {
	t.Parallel()

	if (&Server{}).NeedLeaderElection() {
		t.Fatal("pprof server must not require leader election")
	}
}

func TestServerStartShutdown(t *testing.T) {
	port := freeTCPPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- (&Server{Addr: addr}).Start(ctx)
	}()

	waitForHTTP(t, fmt.Sprintf("http://%s/debug/pprof/", addr))

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned error after shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not exit after context cancel")
	}
}

func TestServerStartDefaultAddrWhenTaken(t *testing.T) {
	holder, err := net.Listen("tcp", defaultAddr)
	if err != nil {
		t.Skip("cannot bind default pprof address for conflict test")
	}
	defer func() { _ = holder.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := (&Server{Addr: ""}).Start(ctx); err == nil {
		t.Fatal("expected error when default address is unavailable")
	}
}

func TestServerStartListenError(t *testing.T) {
	port := freeTCPPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ctx1, cancel1 := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- (&Server{Addr: addr}).Start(ctx1)
	}()

	waitForHTTP(t, fmt.Sprintf("http://%s/debug/pprof/", addr))

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()

	if err := (&Server{Addr: addr}).Start(ctx2); err == nil {
		t.Fatal("expected error when address is already bound")
	}

	cancel1()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("first server shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first server did not stop")
	}
}
