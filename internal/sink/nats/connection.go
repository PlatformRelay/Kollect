// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"fmt"

	natsgo "github.com/nats-io/nats.go"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

var connectionDial = connect

var connectionIsConnected = func(nc *natsgo.Conn) bool {
	return nc != nil && nc.IsConnected()
}

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
	nc, err := connectionDial(cfg, tlsCfg)
	if err != nil {
		return err
	}
	defer func() {
		if nc != nil {
			nc.Close()
		}
	}()
	if !connectionIsConnected(nc) {
		return fmt.Errorf("nats connect: not connected")
	}
	js, err := jetStreamFromConn(nc)
	if err != nil {
		return fmt.Errorf("nats jetstream: %w", err)
	}
	if _, err := js.AccountInfo(ctx); err != nil {
		return fmt.Errorf("nats jetstream account info: %w", err)
	}
	return nil
}
