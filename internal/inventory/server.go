// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/metrics"
)

// Summary is the JSON payload for GET /v1alpha1/inventory.
type Summary struct {
	ItemCount int            `json:"itemCount"`
	Namespace string         `json:"namespace,omitempty"`
	Inventory string         `json:"inventory,omitempty"`
	Items     []collect.Item `json:"items,omitempty"`
	UpdatedAt string         `json:"updatedAt"`
}

// Server serves read-only inventory HTTP endpoints backed by the collection store.
type Server struct {
	Enabled bool
	Port    int32
	Store   *collect.Store
	Auth    *AuthConfig
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
	inventoryHandler := http.Handler(http.HandlerFunc(s.handleInventory))
	watchHandler := http.Handler(http.HandlerFunc(s.handleWatch))
	if s.Auth != nil {
		s.Auth.InitCache()
		inventoryHandler = s.Auth.Middleware(inventoryHandler)
		watchHandler = s.Auth.Middleware(watchHandler)
	}

	mux.Handle("GET /v1alpha1/inventory", inventoryHandler)
	mux.Handle("GET /v1alpha1/inventory/{namespace}/{name}", inventoryHandler)
	mux.Handle("GET /v1alpha1/inventory/watch", watchHandler)
	// Deprecated paths — retained for one release.
	mux.Handle("GET /inventory", inventoryHandler)
	mux.Handle("GET /inventory/watch", watchHandler)

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

	log.FromContext(ctx).Info("inventory HTTP listening", "port", port, "authMode", s.Auth.Mode)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("inventory HTTP server: %w", err)
	}

	return nil
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	inventoryName := strings.TrimSpace(r.URL.Query().Get("inventory"))

	if ns := r.PathValue("namespace"); ns != "" {
		namespace = ns
	}
	if name := r.PathValue("name"); name != "" {
		inventoryName = name
	}

	summary := s.buildSummary(namespace, inventoryName)

	metrics.InventoryItemsTotal.Set(float64(summary.ItemCount))
	metrics.CollectItemsTotal.Set(float64(summary.ItemCount))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)

		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)

		return
	}

	namespace := r.URL.Query().Get("namespace")
	inventoryName := r.URL.Query().Get("inventory")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.Store.Subscribe()
	defer s.Store.Unsubscribe(ch)

	send := func() bool {
		payload, err := json.Marshal(s.buildSummary(namespace, inventoryName))
		if err != nil {
			return false
		}

		if _, err := fmt.Fprintf(w, "event: inventory\ndata: %s\n\n", payload); err != nil {
			return false
		}

		flusher.Flush()

		return true
	}

	if !send() {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if !send() {
				return
			}
		}
	}
}

func (s *Server) buildSummary(namespace, inventoryName string) Summary {
	itemCount := 0
	var items []collect.Item

	if s.Store != nil {
		nsSummary := s.Store.Summary(namespace)
		itemCount = nsSummary.ItemCount
		items = nsSummary.Items
	}

	return Summary{
		ItemCount: itemCount,
		Namespace: namespace,
		Inventory: inventoryName,
		Items:     items,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
