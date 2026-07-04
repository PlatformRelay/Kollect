// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// noopDelete satisfies the ValidateDelete leg of admission.Validator[T] by
// always allowing deletion. Every validator in this package embeds it
// instead of repeating an identical method, since none of our CRDs gate
// deletion.
type noopDelete[T any] struct{}

func (noopDelete[T]) ValidateDelete(_ context.Context, _ T) (admission.Warnings, error) {
	return nil, nil
}
