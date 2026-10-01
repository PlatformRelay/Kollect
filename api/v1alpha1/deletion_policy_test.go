// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "testing"

func TestEffectiveDeletionPolicy_defaultsRetain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *KollectSinkSpec
		want string
	}{
		{"nil spec", nil, DeletionPolicyRetain},
		{"unset field", &KollectSinkSpec{Type: SinkTypeGit}, DeletionPolicyRetain},
		{"explicit retain", &KollectSinkSpec{DeletionPolicy: DeletionPolicyRetain}, DeletionPolicyRetain},
		{"explicit delete", &KollectSinkSpec{DeletionPolicy: DeletionPolicyDelete}, DeletionPolicyDelete},
		// Admission rejects anything else; an unexpected value read from an
		// old object must never widen to the destructive policy.
		{"unknown value", &KollectSinkSpec{DeletionPolicy: "delete"}, DeletionPolicyRetain},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := EffectiveDeletionPolicy(tc.spec); got != tc.want {
				t.Fatalf("EffectiveDeletionPolicy() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSnapshotSinkToKollectSinkSpec_carriesDeletionPolicy(t *testing.T) {
	t.Parallel()

	spec := KollectSnapshotSinkSpec{Type: SnapshotSinkTypeS3, DeletionPolicy: DeletionPolicyDelete}
	if got := spec.ToKollectSinkSpec().DeletionPolicy; got != DeletionPolicyDelete {
		t.Fatalf("normalized deletionPolicy = %q, want %q", got, DeletionPolicyDelete)
	}
}
