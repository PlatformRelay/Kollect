// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestIngestHandleReportsMergesAuthenticatedReport(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{Authenticated: true}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled: true,
		Auth: IngestAuthConfig{
			Mode:   IngestAuthModeKubernetes,
			Client: client,
		},
		Merger: NewMerger(store),
	}

	mux := http.NewServeMux()
	mux.Handle("POST "+ingestReportsPath, srv.Auth.Middleware(http.HandlerFunc(srv.handleReports)))

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
		Items: []collect.Item{
			{
				TargetNamespace: "team-a",
				TargetName:      "t",
				Namespace:       "apps",
				Name:            "demo",
				UID:             "uid-1",
				Version:         "v1",
				Kind:            "Deployment",
			},
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if store.TotalCount() != 1 {
		t.Fatalf("hub count = %d, want 1", store.TotalCount())
	}
}

func TestIngestHandleReportsRejectsUnregisteredCluster(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled:         true,
		Auth:            IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:          NewMerger(store),
		AllowedClusters: []string{"spoke-a"},
	}

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "rogue",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "rogue")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
