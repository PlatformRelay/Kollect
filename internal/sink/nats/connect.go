// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"fmt"

	natsgo "github.com/nats-io/nats.go"

	"github.com/platformrelay/kollect/internal/redact"
	"github.com/platformrelay/kollect/internal/sink/netguard"
)

// natsDialer is natsgo.Connect's shape, a parameter so tests can make the
// driver return credential-bearing error text.
type natsDialer func(url string, options ...natsgo.Option) (*natsgo.Conn, error)

func connect(cfg Config, tlsCfg TLSConfig) (*natsgo.Conn, error) {
	return connectWith(natsgo.Connect, cfg, tlsCfg)
}

func connectWith(dial natsDialer, cfg Config, tlsCfg TLSConfig) (*natsgo.Conn, error) {
	opts := []natsgo.Option{natsgo.SetCustomDialer(netguard.DefaultDialer)}
	if tlsClient := tlsCfg.ClientConfig(); tlsClient != nil {
		opts = append(opts, natsgo.Secure(tlsClient))
	}
	if cfg.Token != "" {
		opts = append(opts, natsgo.Token(cfg.Token))
	} else if cfg.Username != "" {
		opts = append(opts, natsgo.UserInfo(cfg.Username, cfg.Password))
	}
	nc, err := dial(cfg.URL, opts...)
	if err != nil {
		// K-23/K-25: nats.go error text can echo the server URL; mask
		// userinfo and the resolved credential values before the error
		// reaches a condition message or Event.
		return nil, redact.Error(fmt.Errorf("nats connect: %w", err), cfg.Token, cfg.Password)
	}

	return nc, nil
}
