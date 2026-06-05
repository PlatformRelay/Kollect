// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
	caPEM []byte,
) error {
	cfg, err := ConfigFromSpec(spec, secretData)
	if err != nil {
		return err
	}
	tlsCfg, err := TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return err
	}
	nc, err := connect(cfg, tlsCfg)
	if err != nil {
		return err
	}
	defer nc.Close()
	if !nc.IsConnected() {
		return fmt.Errorf("nats connect: not connected")
	}
	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("nats jetstream: %w", err)
	}
	if _, err := js.AccountInfo(ctx); err != nil {
		return fmt.Errorf("nats jetstream account info: %w", err)
	}
	return nil
}
