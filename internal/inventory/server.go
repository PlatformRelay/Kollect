// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/konih/kollect/internal/metrics"
)

// Summary is the JSON payload for GET /inventory (stub until aggregation is wired).
type Summary struct {
	ItemCount int            `json:"itemCount"`
	Clusters  map[string]any `json:"clusters,omitempty"`
	UpdatedAt string         `json:"updatedAt"`
	Note      string         `json:"note,omitempty"`
}

// Server serves read-only inventory HTTP endpoints.
type Server struct {
	Enabled bool
	Port    int32
}

// Start runs the HTTP server until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	port := s.Port
	if port == 0 {
		port = 8082
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /inventory", s.handleInventory)
	// TODO: SSE or watch endpoint for async inventory updates (see ADR-0014).

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.FromContext(ctx).Info("inventory HTTP listening", "port", port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("inventory HTTP server: %w", err)
	}

	return nil
}

func (s *Server) handleInventory(w http.ResponseWriter, _ *http.Request) {
	summary := Summary{
		ItemCount: 0,
		Clusters:  map[string]any{},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Note:      "stub summary until KollectInventory aggregation is implemented",
	}
	metrics.InventoryItemsTotal.Set(float64(summary.ItemCount))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}
