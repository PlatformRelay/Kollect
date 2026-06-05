// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

const ingestReportsPath = "/hub/v1alpha1/reports"

// IngestReportsPath is the hub HTTP ingest endpoint for spoke push (ADR-0028).
func IngestReportsPath() string {
	return ingestReportsPath
}

// IngestServer accepts authenticated spoke inventory reports over HTTP (ADR-0028).
type IngestServer struct {
	Enabled           bool
	Port              int32
	Auth              IngestAuthConfig
	Merger            *Merger
	StatusClient      client.Client
	AllowedClusters   []string
	AllowlistEnforced bool
	Exporter          *Exporter
}

// Start runs the HTTP ingest server until ctx is cancelled.
func (s *IngestServer) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	port := s.Port
	if port == 0 {
		port = 8083
	}

	mux := http.NewServeMux()
	handler := s.Auth.Middleware(http.HandlerFunc(s.handleReports))
	mux.Handle("POST "+ingestReportsPath, handler)

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

	log.FromContext(ctx).Info("hub ingest HTTP listening", "port", port, "authMode", s.Auth.Mode)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("hub ingest HTTP server: %w", err)
	}

	return nil
}

func (s *IngestServer) handleReports(w http.ResponseWriter, r *http.Request) {
	if s.Merger == nil {
		http.Error(w, "merger unavailable", http.StatusServiceUnavailable)

		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)

		return
	}

	headerCluster := strings.TrimSpace(r.Header.Get(kollectdevv1alpha1.HeaderClusterID))
	if headerCluster == "" {
		http.Error(w, "missing cluster id", http.StatusBadRequest)

		return
	}

	report, _, err := ReceiveReport(
		headerCluster,
		body,
		s.Merger,
		s.AllowedClusters,
		s.AllowlistEnforced,
	)
	if err != nil {
		metrics.HubSpokeReportsTotal.WithLabelValues("http-ingest", metrics.ResultFailure).Inc()
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "merge") {
			status = http.StatusInternalServerError
		}

		http.Error(w, err.Error(), status)

		return
	}

	if s.StatusClient != nil {
		_ = MarkRemoteClusterConnected(r.Context(), s.StatusClient, report.Cluster)
	}

	if s.Exporter != nil {
		if err := s.Exporter.ExportAfterMerge(r.Context(), report); err != nil {
			metrics.HubSpokeReportsTotal.WithLabelValues("http-ingest", metrics.ResultFailure).Inc()
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}
	}

	metrics.HubSpokeReportsTotal.WithLabelValues("http-ingest", metrics.ResultSuccess).Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}
