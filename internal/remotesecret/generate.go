// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package remotesecret

import (
	"encoding/base64"
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// Options configures Istio-style remote credential secret generation (ADR-0503).
type Options struct {
	ClusterName string
	Namespace   string
	APIServer   string
	Token       string
	CAData      string
}

// GenerateYAML returns a hub-namespace Secret manifest with a kubeconfig fragment data key.
func GenerateYAML(opts Options) (string, error) {
	cluster := strings.TrimSpace(opts.ClusterName)
	if cluster == "" {
		return "", fmt.Errorf("cluster name is required")
	}

	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		namespace = "platform"
	}

	apiServer := strings.TrimSpace(opts.APIServer)
	if apiServer == "" {
		apiServer = "https://REPLACE_ME:6443"
	}

	token := strings.TrimSpace(opts.Token)
	if token == "" {
		token = "REPLACE_WITH_SPOKE_SA_TOKEN"
	}

	caData := strings.TrimSpace(opts.CAData)
	if caData == "" {
		caData = "REPLACE_WITH_BASE64_CA_DATA"
	}

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    token: %s
`, caData, apiServer, cluster, cluster, cluster, cluster, cluster, cluster, token)

	encoded := base64.StdEncoding.EncodeToString([]byte(kubeconfig))
	secretName := kollectdevv1alpha1.RemoteSecretNamePrefix + cluster

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    %s: "true"
  annotations:
    %s: %s
type: Opaque
data:
  %s: %s
`, secretName, namespace, kollectdevv1alpha1.LabelMultiCluster, kollectdevv1alpha1.AnnotationClusterName,
		cluster, cluster, encoded), nil
}
