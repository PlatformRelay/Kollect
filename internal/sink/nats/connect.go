// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"fmt"

	natsgo "github.com/nats-io/nats.go"
)

func connect(cfg Config, tlsCfg TLSConfig) (*natsgo.Conn, error) {
	opts := []natsgo.Option{}
	if tlsClient, err := tlsCfg.ClientConfig(); err != nil {
		return nil, err
	} else if tlsClient != nil {
		opts = append(opts, natsgo.Secure(tlsClient))
	}
	if cfg.Token != "" {
		opts = append(opts, natsgo.Token(cfg.Token))
	} else if cfg.Username != "" {
		opts = append(opts, natsgo.UserInfo(cfg.Username, cfg.Password))
	}
	nc, err := natsgo.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return nc, nil
}
