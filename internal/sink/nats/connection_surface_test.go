// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"errors"
	"strings"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

type accountInfoJetStream struct {
	jetstream.JetStream
	infoErr error
}

func (f *accountInfoJetStream) AccountInfo(context.Context) (*jetstream.AccountInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return &jetstream.AccountInfo{}, nil
}

func TestTestConnection_notConnected(t *testing.T) {

	t.Cleanup(func() { connectionDial = connect })
	connectionDial = func(Config, TLSConfig) (*natsgo.Conn, error) {
		return nil, nil
	}

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events"},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("error = %v, want not connected", err)
	}
}

func TestTestConnection_jetStreamAccountInfoError(t *testing.T) {

	t.Cleanup(func() {
		connectionDial = connect
		connectionIsConnected = func(nc *natsgo.Conn) bool {
			return nc != nil && nc.IsConnected()
		}
		resetJetStreamFromConn()
	})
	sentinel := errors.New("account info boom")
	connectionDial = func(Config, TLSConfig) (*natsgo.Conn, error) {
		return nil, nil
	}
	connectionIsConnected = func(*natsgo.Conn) bool { return true }
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) {
		return &accountInfoJetStream{infoErr: sentinel}, nil
	}

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events"},
	}, nil, nil)
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want account info failure", err)
	}
	if !strings.Contains(err.Error(), "nats jetstream account info:") {
		t.Fatalf("error = %q, want account info prefix", err.Error())
	}
}
func TestTestConnection_success(t *testing.T) {

	t.Cleanup(func() {
		connectionDial = connect
		connectionIsConnected = func(nc *natsgo.Conn) bool { return nc != nil && nc.IsConnected() }
		resetJetStreamFromConn()
	})
	connectionDial = func(Config, TLSConfig) (*natsgo.Conn, error) { return nil, nil }
	connectionIsConnected = func(*natsgo.Conn) bool { return true }
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) {
		return &accountInfoJetStream{}, nil
	}

	if err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events"},
	}, nil, nil); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}
