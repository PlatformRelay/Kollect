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
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/platformrelay/kollect/internal/integrationtest"
)

// PostgresServer is a postgres:16-alpine container serving TLS with a test certificate.
type PostgresServer struct {
	Host string
	Port string
}

const (
	postgresUser     = "kollect"
	postgresPassword = "kollect"
	postgresDatabase = "inventory"
)

// DSN returns a keyword DSN for the server with extra appended (for example
// "sslmode=verify-full"); extra may be empty.
func (s PostgresServer) DSN(extra string) string {
	return strings.TrimSpace(fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s",
		s.Host, s.Port, postgresUser, postgresPassword, postgresDatabase, extra))
}

// postgresTLSEntrypoint installs the key and certificate with the owner and mode the
// server demands (uid 70, key 0600), then runs the image entrypoint with TLS on. A key
// copied in as-is is refused (sink-ca-trust probe 0.1).
const postgresTLSEntrypoint = `install -o postgres -m 0600 /certs/server.key /var/lib/postgresql/server.key &&
install -o postgres -m 0644 /certs/server.crt /var/lib/postgresql/server.crt &&
exec docker-entrypoint.sh "$@" -c ssl=on -c ssl_cert_file=/var/lib/postgresql/server.crt -c ssl_key_file=/var/lib/postgresql/server.key`

// StartTLSPostgres starts a TLS Postgres container serving cert and terminates it
// when t ends. A missing Docker daemon skips t (or fails it when Docker is required).
func StartTLSPostgres(t testing.TB, cert tls.Certificate) PostgresServer {
	t.Helper()

	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatalf("marshal server key: %v", err)
	}

	var certPEM strings.Builder
	for _, der := range cert.Certificate {
		certPEM.Write(pem.EncodeToMemory(&pem.Block{Type: certBlockType, Bytes: der}))
	}

	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase(postgresDatabase),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPassword),
		testcontainers.WithFiles(
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))),
				ContainerFilePath: "/certs/server.key",
				FileMode:          0o600,
			},
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(certPEM.String()),
				ContainerFilePath: "/certs/server.crt",
				FileMode:          0o644,
			},
		),
		testcontainers.WithEntrypoint("sh", "-c", postgresTLSEntrypoint, "sh"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
			wait.ForListeningPort("5432/tcp"),
		),
	)
	if ctr != nil {
		t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })
	}
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}

		t.Fatalf("start TLS postgres: %v", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}

	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}

	return PostgresServer{Host: host, Port: port.Port()}
}
