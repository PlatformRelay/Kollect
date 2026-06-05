// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pprof

import (
	"context"
	"fmt"
	"net/http"
	"time"

	// Registers pprof handlers on DefaultServeMux; exposed only when --enable-pprof is set (ADR-0603).
	//nolint:gosec // G108: intentional opt-in profiling endpoint.
	_ "net/http/pprof"
)

const defaultAddr = ":6060"

// Server exposes Go pprof endpoints on a dedicated listen address (ADR-0603).
type Server struct {
	Addr string
}

// Start implements manager.Runnable.
func (s *Server) Start(ctx context.Context) error {
	addr := s.Addr
	if addr == "" {
		addr = defaultAddr
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("pprof listen %s: %w", addr, err)
	}

	return nil
}

// NeedLeaderElection returns false so profiling is available on any replica when enabled.
func (s *Server) NeedLeaderElection() bool {
	return false
}
