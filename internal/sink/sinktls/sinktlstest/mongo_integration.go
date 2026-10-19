//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sinktlstest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/platformrelay/kollect/internal/integrationtest"
)

// MongoServer is a mongo:7 container that accepts TLS connections only.
type MongoServer struct {
	Host string
	Port string
}

// URI returns a mongodb:// URI for the server with query appended (for example
// "tls=true"); query may be empty.
func (s MongoServer) URI(query string) string {
	uri := fmt.Sprintf("mongodb://%s:%s/", s.Host, s.Port)
	if query != "" {
		uri += "?" + query
	}

	return uri
}

// StartTLSMongo starts a mongo:7 container with --tlsMode requireTLS serving cert,
// issued by the CA in issuerPEM, and terminates it when t ends. mongod needs a CA
// file next to the key file, and the combined certificate and key at mode 0644 (the
// server runs as uid 999; sink-ca-trust probe 0.1). Client certificates are not
// required. A missing Docker daemon skips t (or fails it when Docker is required).
func StartTLSMongo(t testing.TB, cert tls.Certificate, issuerPEM []byte) MongoServer {
	t.Helper()

	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatalf("marshal server key: %v", err)
	}

	var combined strings.Builder
	for _, der := range cert.Certificate {
		combined.Write(pem.EncodeToMemory(&pem.Block{Type: certBlockType, Bytes: der}))
	}
	combined.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))

	ctx := context.Background()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:7",
			ExposedPorts: []string{"27017/tcp"},
			Files: []testcontainers.ContainerFile{
				{Reader: strings.NewReader(combined.String()), ContainerFilePath: "/certs/server.pem", FileMode: 0o644},
				{Reader: strings.NewReader(string(issuerPEM)), ContainerFilePath: "/certs/ca.pem", FileMode: 0o644},
			},
			Cmd: []string{
				"--tlsMode", "requireTLS",
				"--tlsCertificateKeyFile", "/certs/server.pem",
				"--tlsCAFile", "/certs/ca.pem",
				"--tlsAllowConnectionsWithoutCertificates",
				"--bind_ip_all",
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("Waiting for connections").WithStartupTimeout(90*time.Second),
				wait.ForListeningPort("27017/tcp"),
			),
		},
		Started: true,
	})
	if ctr != nil {
		t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })
	}
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}

		t.Fatalf("start TLS mongodb: %v", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}

	port, err := ctr.MappedPort(ctx, "27017/tcp")
	if err != nil {
		t.Fatal(err)
	}

	return MongoServer{Host: host, Port: port.Port()}
}
