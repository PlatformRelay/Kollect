// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konih/kollect/internal/collect"
)

func TestIngestHandleReports_errors(t *testing.T) {
	t.Parallel()

	srv := &IngestServer{Enabled: true, Auth: IngestAuthConfig{Mode: IngestAuthModeDisabled}}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil merger status = %d", rec.Code)
	}

	store := collect.NewStore()
	srv.Merger = NewMerger(store)
	req = httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader([]byte(`{}`)))
	rec = httptest.NewRecorder()
	srv.handleReports(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing cluster status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader([]byte(`not-json`)))
	req.Header.Set("X-Kollect-Cluster-Id", "spoke-a")
	rec = httptest.NewRecorder()
	srv.handleReports(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d", rec.Code)
	}
}
