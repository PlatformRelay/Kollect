// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func newStubBackend(typeName string) Factory {
	return func(spec kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return nil, fmt.Errorf("sink type %q is not implemented yet (ADR-0414 stub)", typeName)
	}
}
