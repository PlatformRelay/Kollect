// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func TestShouldTestConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sink kollectdevv1alpha1.KollectSink
		want bool
	}{
		{
			name: "spec connectionTest true",
			sink: kollectdevv1alpha1.KollectSink{
				Spec: kollectdevv1alpha1.KollectSinkSpec{ConnectionTest: true},
			},
			want: true,
		},
		{
			name: "annotation true",
			sink: kollectdevv1alpha1.KollectSink{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kollectdevv1alpha1.AnnotationTestConnection: "true",
					},
				},
			},
			want: true,
		},
		{
			name: "annotation TRUE case insensitive",
			sink: kollectdevv1alpha1.KollectSink{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kollectdevv1alpha1.AnnotationTestConnection: "TRUE",
					},
				},
			},
			want: true,
		},
		{
			name: "default no probe",
			sink: kollectdevv1alpha1.KollectSink{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldTestConnection(&tt.sink); got != tt.want {
				t.Fatalf("shouldTestConnection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunConnectionTest_unsupportedType(t *testing.T) {
	t.Parallel()

	_, err := sink.RunConnectionTest(t.Context(), kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}, sink.BuildContext{})
	if err == nil {
		t.Fatal("expected error for unsupported sink type")
	}
}
