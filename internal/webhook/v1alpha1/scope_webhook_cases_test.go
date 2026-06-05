// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type scopeWebhookCreateCase struct {
	name     string
	ceiling  kollectdevv1alpha1.ScopeCeilingSpec
	sinkRefs []string
	wantErr  bool
}

func scopeWebhookCreateCases() []scopeWebhookCreateCase {
	return []scopeWebhookCreateCase{
		{
			name: "missing kind",
			ceiling: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{{Version: "v1", Kind: ""}},
			},
			wantErr: true,
		},
		{name: "duplicate sinkRefs", sinkRefs: []string{"git-a", "git-a"}, wantErr: true},
		{
			name: "valid scope",
			ceiling: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs:       []kollectdevv1alpha1.GroupVersionKind{{Group: "apps", Version: "v1", Kind: "Deployment"}},
				AllowedNamespaces: []string{"team-a"},
			},
			sinkRefs: []string{"git-inventory"},
		},
	}
}

func TestScopeWebhookCreateValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range scopeWebhookCreateCases() {
		t.Run("KollectScope/"+tc.name, func(t *testing.T) {
			t.Parallel()

			v := &kollectScopeValidator{}
			_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectScope{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: "team-a"},
				Spec:       kollectdevv1alpha1.KollectScopeSpec{ScopeCeilingSpec: tc.ceiling, SinkRefs: tc.sinkRefs},
			})
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected valid scope: %v", err)
			}
		})

		t.Run("KollectClusterScope/"+tc.name, func(t *testing.T) {
			t.Parallel()

			v := &kollectClusterScopeValidator{}
			_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterScope{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       kollectdevv1alpha1.KollectClusterScopeSpec{ScopeCeilingSpec: tc.ceiling, SinkRefs: tc.sinkRefs},
			})
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected valid cluster scope: %v", err)
			}
		})
	}
}
