// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestProvisionTable_existingVerifies(t *testing.T) {
	t.Parallel()

	verifyCalled, ensureCalled := false, false
	err := provisionTable(
		context.Background(),
		kollectdevv1alpha1.ProvisioningModeExisting,
		func(context.Context) error { verifyCalled = true; return nil },
		func(context.Context) error { ensureCalled = true; return nil },
	)
	if err != nil {
		t.Fatalf("provisionTable() error = %v", err)
	}
	if !verifyCalled || ensureCalled {
		t.Fatalf("existing mode: verifyCalled=%v ensureCalled=%v, want verify only", verifyCalled, ensureCalled)
	}
}

func TestProvisionTable_ensureRunsDDL(t *testing.T) {
	t.Parallel()

	verifyCalled, ensureCalled := false, false
	err := provisionTable(
		context.Background(),
		kollectdevv1alpha1.ProvisioningModeEnsure,
		func(context.Context) error { verifyCalled = true; return nil },
		func(context.Context) error { ensureCalled = true; return nil },
	)
	if err != nil {
		t.Fatalf("provisionTable() error = %v", err)
	}
	if verifyCalled || !ensureCalled {
		t.Fatalf("ensure mode: verifyCalled=%v ensureCalled=%v, want ensure only", verifyCalled, ensureCalled)
	}
}

func TestProvisionTable_emptyModeDefaultsToEnsure(t *testing.T) {
	t.Parallel()

	ensureCalled := false
	err := provisionTable(
		context.Background(),
		"",
		func(context.Context) error { return nil },
		func(context.Context) error { ensureCalled = true; return nil },
	)
	if err != nil {
		t.Fatalf("provisionTable() error = %v", err)
	}
	if !ensureCalled {
		t.Fatal("empty mode must run ensure")
	}
}

func TestClassifyTableProbe_absentIsNotFound(t *testing.T) {
	t.Parallel()

	err := classifyTableProbe("public", "inventory_items", false, nil)
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("error = %v, want ErrTableNotFound", err)
	}
	if !strings.Contains(err.Error(), "public.inventory_items does not exist (provisioning.mode=existing)") {
		t.Fatalf("error text = %q, want the provisioning.mode=existing shape", err)
	}
}

func TestClassifyTableProbe_queryErrorIsNotNotFound(t *testing.T) {
	t.Parallel()

	err := classifyTableProbe("public", "inventory_items", false, errors.New("permission denied for schema public"))
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrTableNotFound) {
		t.Fatalf("a failed probe must not be reported as not-found: %v", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error text = %q, want the underlying cause retained", err)
	}
}

func TestClassifyTableProbe_presentIsNil(t *testing.T) {
	t.Parallel()

	if err := classifyTableProbe("public", "inventory_items", true, nil); err != nil {
		t.Fatalf("classifyTableProbe(present) = %v, want nil", err)
	}
}
