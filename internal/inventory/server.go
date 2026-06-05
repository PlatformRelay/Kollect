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

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/metrics"
)

// Server serves read-only inventory HTTP endpoints backed by the collection store.
type Server struct {
	Enabled bool
	Port    int32
	Store   *collect.Store
	Auth    *AuthConfig
	Status  StatusReader
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
	statusInventoriesHandler := http.Handler(http.HandlerFunc(s.handleStatusInventories))
	statusTargetsHandler := http.Handler(http.HandlerFunc(s.handleStatusTargets))

	if s.Auth != nil {
		s.Auth.InitCache()
		inventoryHandler = s.Auth.Middleware(inventoryHandler)
		watchHandler = s.Auth.Middleware(watchHandler)
		statusInventoriesHandler = s.Auth.Middleware(statusInventoriesHandler)
		statusTargetsHandler = s.Auth.Middleware(statusTargetsHandler)
	}

	mux.Handle("GET /v1alpha1/inventory", inventoryHandler)
	mux.Handle("GET /v1alpha1/inventory/{namespace}/{name}", inventoryHandler)
	mux.Handle("GET /v1alpha1/inventory/watch", watchHandler)
	mux.Handle("GET /v1alpha1/status/inventories", statusInventoriesHandler)
	mux.Handle("GET /v1alpha1/status/targets", statusTargetsHandler)
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
	filter := parseListFilter(r)
	summary := s.buildSummary(r.Context(), filter)

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

	filter := parseListFilter(r)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.Store.Subscribe()
	defer s.Store.Unsubscribe(ch)

	send := func() bool {
		payload, err := json.Marshal(s.buildSummary(r.Context(), filter))
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

func (s *Server) buildSummary(ctx context.Context, filter ListFilter) InventorySummary {
	items := s.collectItems(filter)
	items = filterItems(items, filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultPageLimit
	}

	pagedItems, pagination := paginateItems(items, limit, filter.Offset)
	if len(items) > 0 && pagination == nil {
		pagination = &Pagination{Limit: len(items), Offset: 0, Total: len(items), HasMore: false}
	}

	checksum, _ := collect.ItemsFingerprint(pagedItems)

	summary := InventorySummary{
		SchemaVersion: collect.ExportSchemaVersion,
		ItemCount:     len(pagedItems),
		Namespace:     filter.Namespace,
		Inventory:     filter.Inventory,
		Items:         pagedItems,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Pagination:    pagination,
		Checksum:      checksum,
	}

	if s.Status != nil && filter.Namespace != "" && filter.Inventory != "" {
		exportStatus, err := s.Status.GetInventoryExportStatus(ctx, filter.Namespace, filter.Inventory)
		if err == nil && len(exportStatus) > 0 {
			summary.ExportStatus = exportStatus
		}
	}

	if summary.Items == nil {
		summary.Items = []collect.Item{}
	}

	return summary
}

func (s *Server) collectItems(filter ListFilter) []collect.Item {
	if s.Store == nil {
		return nil
	}

	ns := filter.Namespace
	if ns == "" && filter.Inventory == "" {
		return s.Store.Summary("").Items
	}

	return s.Store.Summary(ns).Items
}

// Summary is retained as an alias for backward-compatible tests and docs.
type Summary = InventorySummary
