// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sink type values for KollectSink.spec.type (ADR-0406).
const (
	SinkTypeGit      = "git"
	SinkTypeGitLab   = "gitlab"
	SinkTypeS3       = "s3"
	SinkTypeGCS      = "gcs"
	SinkTypePostgres = "postgres"
	SinkTypeKafka    = "kafka"
	SinkTypeNats     = "nats"
)

// KollectSinkSpec defines the desired state of KollectSink.
type KollectSinkSpec struct {
	// type selects the sink backend implementation.
	// +kubebuilder:validation:Enum=git;gitlab;s3;gcs;postgres;kafka;nats
	// +required
	Type string `json:"type"`

	// endpoint is the backend-specific destination (URL, bucket, and so on).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// secretRef references a Secret holding credentials for the sink.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// tls configures TLS verification for HTTPS git and similar endpoints.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// connectionTest enables connectivity checks on create/update (default true).
	// Set to false to skip automatic probes; the annotation kollect.dev/test-connection=true
	// triggers a one-shot re-test without editing spec.
	// +kubebuilder:default=true
	// +optional
	ConnectionTest *bool `json:"connectionTest,omitempty"`

	// cluster labels exported inventory in multi-cluster installs.
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// pathTemplate selects the Git/object-store export path layout (ADR-0407).
	// Placeholders: {cluster}, {namespace}, {name}, {generation}, {extension}.
	// Default: inventory/{namespace}/{name}.json
	// +optional
	PathTemplate string `json:"pathTemplate,omitempty"`

	// postgres configures a PostgreSQL database sink.
	// +optional
	Postgres *PostgresSpec `json:"postgres,omitempty"`

	// kafka configures a Kafka or Redpanda event sink.
	// +optional
	Kafka *KafkaSpec `json:"kafka,omitempty"`

	// nats configures a NATS JetStream event sink.
	// +optional
	Nats *NatsSpec `json:"nats,omitempty"`

	// objectStore configures S3/GCS snapshot export format and layout.
	// +optional
	ObjectStore *ObjectStoreSpec `json:"objectStore,omitempty"`

	// git configures git sink settings when type is git.
	// +optional
	Git *GitSpec `json:"git,omitempty"`

	// gitlab configures GitLab-specific settings when type is gitlab.
	// +optional
	GitLab *GitLabSpec `json:"gitlab,omitempty"`

	// exportMinInterval is the default minimum time between identical exports when an inventory
	// ref omits a per-ref override. Material payload changes bypass the interval.
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`
}

// Git push policy values (ADR-0407).
const (
	GitPushPolicyCommit    = "Commit"
	GitPushPolicyForcePush = "ForcePush"
)

// Git auth type values.
const (
	GitAuthTypeToken = "token"
	GitAuthTypeSSH   = "ssh"
)

// GitSpec configures plain git sink export behavior.
type GitSpec struct {
	// branch is the target branch for clone and push.
	// Overrides the endpoint URL fragment (#branch=) when set.
	// +optional
	Branch string `json:"branch,omitempty"`

	// pushPolicy selects append-only commits (Commit, default) or force-push snapshot mode (ForcePush).
	// +kubebuilder:validation:Enum=Commit;ForcePush
	// +optional
	PushPolicy string `json:"pushPolicy,omitempty"`

	// auth documents credential mode and optional secret override for git push.
	// +optional
	Auth *GitAuthSpec `json:"auth,omitempty"`

	// commitMessage is the git commit message template.
	// Placeholders: {namespace}, {name}, {cluster}, {generation}.
	// +optional
	CommitMessage string `json:"commitMessage,omitempty"`

	// author overrides the default kollect committer identity.
	// +optional
	Author *GitAuthorSpec `json:"author,omitempty"`

	// cloneDepth limits shallow clone depth (default 1).
	// +kubebuilder:validation:Minimum=1
	// +optional
	CloneDepth *int32 `json:"cloneDepth,omitempty"`

	// prune stages deletions in the worktree before commit (git add -A semantics).
	// +optional
	Prune bool `json:"prune,omitempty"`
}

// GitAuthSpec configures git credential mode for HTTPS or SSH remotes.
type GitAuthSpec struct {
	// type selects HTTPS token/basic auth or SSH private-key auth.
	// +kubebuilder:validation:Enum=token;ssh
	// +optional
	Type string `json:"type,omitempty"`

	// secretRef overrides spec.secretRef for git credentials when set.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// GitAuthorSpec overrides the default git commit author.
type GitAuthorSpec struct {
	// name is the commit author display name (default kollect).
	// +optional
	Name string `json:"name,omitempty"`

	// email is the commit author email (default kollect@kollect.dev).
	// +optional
	Email string `json:"email,omitempty"`
}

// ObjectStoreSpec configures S3/GCS snapshot serialization (ADR-0401).
type ObjectStoreSpec struct {
	// format selects the snapshot serialization (default json).
	// +kubebuilder:validation:Enum=json;parquet
	// +optional
	Format string `json:"format,omitempty"`
	// hotAttributes lists profile attribute keys promoted to typed Parquet columns (ADR-0401, Q11).
	// +optional
	// +listType=atomic
	HotAttributes []string `json:"hotAttributes,omitempty"`
}

// GitLabSpec configures GitLab sink settings beyond the shared endpoint and TLS fields.
type GitLabSpec struct {
	// mergeRequest configures optional branch + merge request workflow after git push.
	// +optional
	MergeRequest *MergeRequestSpec `json:"mergeRequest,omitempty"`
}

// MergeRequestSpec configures GitLab REST merge request workflow (ADR-0402 Phase 2).
type MergeRequestSpec struct {
	// mode selects direct push to the default branch or branch + merge request workflow.
	// +kubebuilder:validation:Enum=direct;merge_request
	// +optional
	Mode string `json:"mode,omitempty"`

	// targetBranch is the MR target branch (required when mode is merge_request).
	// +optional
	TargetBranch string `json:"targetBranch,omitempty"`

	// branchPrefix prefixes feature branches (default kollect).
	// +optional
	BranchPrefix string `json:"branchPrefix,omitempty"`

	// titleTemplate is an optional MR title template with {namespace} and {name} placeholders.
	// +optional
	TitleTemplate string `json:"titleTemplate,omitempty"`

	// autoMerge requests auto-merge when the MR pipeline succeeds (not yet implemented).
	// +optional
	AutoMerge bool `json:"autoMerge,omitempty"`
}

// PostgresSpec configures PostgreSQL upsert export.
type PostgresSpec struct {
	// databaseRef references a Secret containing the connection string (key dsn or url).
	// +required
	DatabaseRef *SecretReference `json:"databaseRef"`

	// table is the destination table name.
	// +required
	Table string `json:"table"`

	// schema is the PostgreSQL schema (default public).
	// +optional
	Schema string `json:"schema,omitempty"`
}

// NatsSpec configures NATS JetStream inventory change events.
type NatsSpec struct {
	// url is the NATS server connection URL (nats://host:4222).
	// When empty, spec.endpoint is used.
	// +optional
	URL string `json:"url,omitempty"`

	// subject is the JetStream publish subject for inventory events.
	// +required
	Subject string `json:"subject"`

	// stream is the JetStream stream name (default kollect_events).
	// +optional
	Stream string `json:"stream,omitempty"`

	// secretRef references a Secret with optional auth credentials (token, username/password).
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// KafkaSpec configures Kafka inventory change events.
type KafkaSpec struct {
	// brokers lists Kafka bootstrap addresses (host:port).
	// +required
	// +listType=atomic
	Brokers []string `json:"brokers"`

	// topic is the destination topic for inventory events.
	// +required
	Topic string `json:"topic"`

	// secretRef references a Secret with optional SASL/TLS credentials.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// TLSSpec configures custom CA trust for sink endpoints.
type TLSSpec struct {
	// insecureSkipVerify disables server certificate verification (not recommended).
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// caBundle is an inline PEM-encoded CA certificate bundle.
	// Prefer caSecretRef for production; do not set both caBundle and caSecretRef.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// caSecretRef references a Secret containing a PEM CA bundle (key tls.crt or ca.crt).
	// +optional
	CASecretRef *SecretReference `json:"caSecretRef,omitempty"`
}

// SecretReference points to a Secret by name and optional namespace.
type SecretReference struct {
	// name is the name of the referenced Secret.
	// +required
	Name string `json:"name"`

	// namespace is the namespace of the referenced Secret.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// KollectSinkStatus defines the observed state of KollectSink.
type KollectSinkStatus struct {
	// conditions represent the current state of the KollectSink resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ksink

// KollectSink is the Schema for the kollectsinks API
type KollectSink struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectSink
	// +required
	Spec KollectSinkSpec `json:"spec"`

	// status defines the observed state of KollectSink
	// +optional
	Status KollectSinkStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectSinkList contains a list of KollectSink
type KollectSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectSink `json:"items"`
}

// ConnectionTestEnabled reports whether automatic sink connectivity probes should run.
// Unset spec.connectionTest defaults to enabled; set false to opt out.
func ConnectionTestEnabled(spec *KollectSinkSpec) bool {
	if spec == nil || spec.ConnectionTest == nil {
		return true
	}
	return *spec.ConnectionTest
}

func init() {
	SchemeBuilder.Register(&KollectSink{}, &KollectSinkList{})
}
