// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

func TestStaticRefResult(t *testing.T) {
	t.Parallel()

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectprofiles"}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "ok", err: nil, want: metrics.StaticRefResultOK},
		{name: "forbidden", err: apierrors.NewForbidden(gr, "p", errors.New("denied")), want: metrics.StaticRefResultForbidden},
		{name: "not found", err: apierrors.NewNotFound(gr, "p"), want: metrics.StaticRefResultNotFound},
		{name: "wrapped not found", err: errWrap(apierrors.NewNotFound(gr, "p")), want: metrics.StaticRefResultNotFound},
		{name: "transient", err: errors.New("connection refused"), want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := staticRefResult(tc.err); got != tc.want {
				t.Fatalf("staticRefResult = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStaticRefTypeForFamily(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		kollectdevv1alpha1.SinkFamilySnapshot: metrics.StaticRefTypeSnapshot,
		kollectdevv1alpha1.SinkFamilyDatabase: metrics.StaticRefTypeDatabase,
		kollectdevv1alpha1.SinkFamilyEvent:    metrics.StaticRefTypeEvent,
	}
	for family, want := range cases {
		if got := staticRefTypeForFamily(family); got != want {
			t.Fatalf("staticRefTypeForFamily(%q) = %q, want %q", family, got, want)
		}
	}
}

func TestRecordStaticRefResolution_IncrementsForbidden(t *testing.T) {
	t.Parallel()

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectdatabasesinks"}
	counter := metrics.StaticRefResolutionTotal.WithLabelValues(
		"KollectClusterInventory", metrics.StaticRefTypeDatabase, metrics.StaticRefResultForbidden,
	)
	before := testutil.ToFloat64(counter)

	recordStaticRefResolution("KollectClusterInventory", metrics.StaticRefTypeDatabase,
		apierrors.NewForbidden(gr, "warehouse", errors.New("denied")))
	// Transient errors must not move the bounded counter.
	recordStaticRefResolution("KollectClusterInventory", metrics.StaticRefTypeDatabase, errors.New("api down"))

	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Fatalf("forbidden counter delta = %v, want 1", got)
	}
}

func errWrap(err error) error {
	return errWrapped{err}
}

type errWrapped struct{ err error }

func (e errWrapped) Error() string { return "wrapped: " + e.err.Error() }
func (e errWrapped) Unwrap() error { return e.err }
