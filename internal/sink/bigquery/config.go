// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/bigquery"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TypeName is the KollectSink.spec.type value for BigQuery sinks.
const TypeName = "bigquery"

const typeName = TypeName

// CredentialsJSONKey is the expected Secret key when spec.bigquery.secretRef is used.
const CredentialsJSONKey = "credentials.json"

// Config holds resolved BigQuery sink settings.
type Config struct {
	Project          string
	Dataset          string
	Table            string
	Location         string
	Cluster          string
	ProvisioningMode string
	CredentialsJSON  []byte
}

// ConfigFromSpec validates spec and optional secret data for a bigquery sink.
func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (Config, error) {
	if spec.Type != typeName {
		return Config{}, fmt.Errorf("expected bigquery sink, got %q", spec.Type)
	}

	if spec.BigQuery == nil {
		return Config{}, fmt.Errorf("bigquery sink requires spec.bigquery")
	}

	bq := spec.BigQuery
	project := strings.TrimSpace(bq.Project)
	if project == "" {
		return Config{}, fmt.Errorf("bigquery sink requires spec.bigquery.project")
	}

	dataset := strings.TrimSpace(bq.Dataset)
	if dataset == "" {
		return Config{}, fmt.Errorf("bigquery sink requires spec.bigquery.dataset")
	}

	table := strings.TrimSpace(bq.Table)
	if table == "" {
		return Config{}, fmt.Errorf("bigquery sink requires spec.bigquery.table")
	}

	cfg := Config{
		Project:          project,
		Dataset:          dataset,
		Table:            table,
		Location:         strings.TrimSpace(bq.Location),
		Cluster:          strings.TrimSpace(spec.Cluster),
		ProvisioningMode: kollectdevv1alpha1.EffectiveProvisioningMode(&spec),
	}

	if bq.SecretRef != nil {
		creds, err := credentialsJSONFromSecret(databaseSecret)
		if err != nil {
			return Config{}, err
		}
		cfg.CredentialsJSON = creds
	}

	return cfg, nil
}

func credentialsJSONFromSecret(data map[string][]byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("bigquery secretRef secret is empty")
	}

	raw, ok := data[CredentialsJSONKey]
	if !ok || len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("bigquery secret must contain credentials.json")
	}

	return raw, nil
}

func (c Config) clientOptions(ctx context.Context) ([]option.ClientOption, error) {
	opts := make([]option.ClientOption, 0, 2)

	if emulatorHost := strings.TrimSpace(os.Getenv("BIGQUERY_EMULATOR_HOST")); emulatorHost != "" {
		if !strings.HasPrefix(emulatorHost, "http://") && !strings.HasPrefix(emulatorHost, "https://") {
			emulatorHost = "http://" + emulatorHost
		}
		opts = append(opts, option.WithEndpoint(emulatorHost), option.WithoutAuthentication())
	}

	if len(c.CredentialsJSON) > 0 {
		creds, err := google.CredentialsFromJSONWithTypeAndParams(
			ctx,
			c.CredentialsJSON,
			google.ServiceAccount,
			google.CredentialsParams{Scopes: []string{bigquery.Scope}},
		)
		if err != nil {
			return nil, fmt.Errorf("parse bigquery credentials.json: %w", err)
		}

		opts = append(opts, option.WithCredentials(creds))
	}

	return opts, nil
}
