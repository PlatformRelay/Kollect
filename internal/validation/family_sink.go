// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var (
	// http and azureblob are intentionally absent: ADR-0414 stubs were removed
	// (EC-P1-04) and each type only re-enters the allowlist together with a real backend.
	validSnapshotSinkTypes = []string{
		kollectdevv1alpha1.SnapshotSinkTypeGit, kollectdevv1alpha1.SnapshotSinkTypeGitLab,
		kollectdevv1alpha1.SnapshotSinkTypeS3, kollectdevv1alpha1.SnapshotSinkTypeGCS,
	}
	validDatabaseSinkTypes = []string{
		kollectdevv1alpha1.DatabaseSinkTypePostgres,
		kollectdevv1alpha1.DatabaseSinkTypeBigQuery,
		kollectdevv1alpha1.DatabaseSinkTypeMongoDB,
	}
	validEventSinkTypes = []string{kollectdevv1alpha1.EventSinkTypeNats, kollectdevv1alpha1.EventSinkTypeKafka}
)

type forbiddenBlock struct {
	path *field.Path
	set  bool
}

// ValidateSnapshotSinkSpec checks KollectSnapshotSink cross-field rules (ADR-0414).
func ValidateSnapshotSinkSpec(spec *kollectdevv1alpha1.KollectSnapshotSinkSpec) field.ErrorList {
	if spec == nil {
		return nil
	}
	allErrs := validateFamilyType(spec.Type, validSnapshotSinkTypes)
	if len(allErrs) > 0 {
		return allErrs
	}
	allErrs = append(allErrs, validateCommonSinkFields(&spec.SinkCommonFields)...)
	allErrs = append(allErrs, validateFormatCapability(spec.Type, spec.Serialization)...)
	layoutPath := field.NewPath("spec").Child("layout")
	switch spec.Type {
	case kollectdevv1alpha1.SnapshotSinkTypeGit:
		allErrs = append(allErrs, requireBlock(spec.Git, field.NewPath("spec").Child("git"), "required when type is git")...)
		allErrs = append(allErrs, forbidBlocks(snapshotForbiddenWhenGit(spec))...)
		allErrs = append(allErrs, validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{Type: spec.Type, Git: spec.Git})...)
		allErrs = append(allErrs, ValidateLayoutSpec(spec.Layout, layoutPath)...)
	case kollectdevv1alpha1.SnapshotSinkTypeGitLab:
		allErrs = append(allErrs, forbidBlocks(snapshotForbiddenWhenGitLab(spec))...)
		allErrs = append(allErrs, ValidateLayoutSpec(spec.Layout, layoutPath)...)
	case kollectdevv1alpha1.SnapshotSinkTypeS3, kollectdevv1alpha1.SnapshotSinkTypeGCS:
		allErrs = append(allErrs, forbidBlocks(snapshotForbiddenWhenObjectStore(spec))...)
		allErrs = append(allErrs, forbidLayout(spec.Layout, layoutPath)...)
	}
	return allErrs
}

func forbidLayout(layout *kollectdevv1alpha1.LayoutSpec, path *field.Path) field.ErrorList {
	if layout == nil {
		return nil
	}
	return field.ErrorList{field.Forbidden(path, "layout is only supported for git and gitlab sinks")}
}

// ValidateDatabaseSinkSpec checks KollectDatabaseSink cross-field rules.
func ValidateDatabaseSinkSpec(spec *kollectdevv1alpha1.KollectDatabaseSinkSpec) field.ErrorList {
	if spec == nil {
		return nil
	}
	allErrs := validateFamilyType(spec.Type, validDatabaseSinkTypes)
	if len(allErrs) > 0 {
		return allErrs
	}
	allErrs = append(allErrs, validateCommonSinkFields(&spec.SinkCommonFields)...)
	allErrs = append(allErrs, validateFormatCapability(spec.Type, spec.Serialization)...)
	allErrs = append(allErrs, forbidLayout(spec.Layout, field.NewPath("spec").Child("layout"))...)
	switch spec.Type {
	case kollectdevv1alpha1.DatabaseSinkTypePostgres:
		allErrs = append(allErrs, requireBlock(spec.Postgres, field.NewPath("spec").Child("postgres"), "required when type is postgres")...)
		allErrs = append(allErrs, forbidBlocks([]forbiddenBlock{
			{field.NewPath("spec").Child("bigquery"), spec.BigQuery != nil},
			{field.NewPath("spec").Child("mongodb"), spec.MongoDB != nil},
		})...)
	case kollectdevv1alpha1.DatabaseSinkTypeMongoDB:
		allErrs = append(allErrs, requireBlock(spec.MongoDB, field.NewPath("spec").Child("mongodb"), "required when type is mongodb")...)
		allErrs = append(allErrs, forbidBlocks([]forbiddenBlock{
			{field.NewPath("spec").Child("postgres"), spec.Postgres != nil},
			{field.NewPath("spec").Child("bigquery"), spec.BigQuery != nil},
		})...)
	case kollectdevv1alpha1.DatabaseSinkTypeBigQuery:
		allErrs = append(allErrs, requireBlock(spec.BigQuery, field.NewPath("spec").Child("bigquery"), "required when type is bigquery")...)
		allErrs = append(allErrs, forbidBlocks([]forbiddenBlock{
			{field.NewPath("spec").Child("postgres"), spec.Postgres != nil},
			{field.NewPath("spec").Child("mongodb"), spec.MongoDB != nil},
		})...)
		allErrs = append(allErrs, validateBigQuerySpec(spec.BigQuery)...)
	}
	return allErrs
}

func validateBigQuerySpec(spec *kollectdevv1alpha1.BigQuerySpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	base := field.NewPath("spec").Child("bigquery")

	if strings.TrimSpace(spec.Project) == "" {
		allErrs = append(allErrs, field.Required(base.Child("project"), "required when type is bigquery"))
	}
	if strings.TrimSpace(spec.Dataset) == "" {
		allErrs = append(allErrs, field.Required(base.Child("dataset"), "required when type is bigquery"))
	}
	if strings.TrimSpace(spec.Table) == "" {
		allErrs = append(allErrs, field.Required(base.Child("table"), "required when type is bigquery"))
	}
	if spec.SecretRef != nil && strings.TrimSpace(spec.SecretRef.Name) == "" {
		allErrs = append(allErrs, field.Required(base.Child("secretRef").Child("name"), "name is required when secretRef is set"))
	}

	return allErrs
}

// ValidateEventSinkSpec checks KollectEventSink cross-field rules.
func ValidateEventSinkSpec(spec *kollectdevv1alpha1.KollectEventSinkSpec) field.ErrorList {
	if spec == nil {
		return nil
	}
	allErrs := validateFamilyType(spec.Type, validEventSinkTypes)
	if len(allErrs) > 0 {
		return allErrs
	}
	allErrs = append(allErrs, validateCommonSinkFields(&spec.SinkCommonFields)...)
	allErrs = append(allErrs, validateFormatCapability(spec.Type, spec.Serialization)...)
	allErrs = append(allErrs, forbidLayout(spec.Layout, field.NewPath("spec").Child("layout"))...)
	switch spec.Type {
	case kollectdevv1alpha1.EventSinkTypeNats:
		allErrs = append(allErrs, requireBlock(spec.Nats, field.NewPath("spec").Child("nats"), "required when type is nats")...)
		allErrs = append(allErrs, forbidBlocks([]forbiddenBlock{{field.NewPath("spec").Child("kafka"), spec.Kafka != nil}})...)
	case kollectdevv1alpha1.EventSinkTypeKafka:
		allErrs = append(allErrs, requireBlock(spec.Kafka, field.NewPath("spec").Child("kafka"), "required when type is kafka")...)
		allErrs = append(allErrs, forbidBlocks([]forbiddenBlock{{field.NewPath("spec").Child("nats"), spec.Nats != nil}})...)
	}
	return allErrs
}

func validateFamilyType(sinkType string, validTypes []string) field.ErrorList {
	typePath := field.NewPath("spec").Child("type")
	sinkType = strings.TrimSpace(sinkType)
	if sinkType == "" {
		return field.ErrorList{field.Required(typePath, "type is required")}
	}
	if !containsString(validTypes, sinkType) {
		return field.ErrorList{field.NotSupported(typePath, sinkType, validTypes)}
	}
	return nil
}

func validateCommonSinkFields(fields *kollectdevv1alpha1.SinkCommonFields) field.ErrorList {
	if fields == nil {
		return nil
	}
	var allErrs field.ErrorList
	if err := ValidatePathTemplate(fields.PathTemplate); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("pathTemplate"), fields.PathTemplate, err.Error()))
	}
	allErrs = append(allErrs, ValidateOptionalDurationInterval(fields.ExportMinInterval, field.NewPath("spec").Child("exportMinInterval"))...)
	allErrs = append(allErrs, ValidateSinkCommonConfig(fields)...)
	allErrs = append(allErrs, ValidateOptionsMap(fields.Options, field.NewPath("spec").Child("options"))...)
	return allErrs
}

func validateFormatCapability(sinkType string, serialization *kollectdevv1alpha1.SerializationSpec) field.ErrorList {
	if serialization == nil || serialization.Format == "" {
		return nil
	}
	return ValidateSinkFormatCapability(sinkType, serialization.Format,
		field.NewPath("spec").Child("serialization").Child("format"))
}

func forbidBlocks(blocks []forbiddenBlock) field.ErrorList {
	var allErrs field.ErrorList
	for _, b := range blocks {
		if b.set {
			allErrs = append(allErrs, field.Forbidden(b.path, "not allowed for this sink type"))
		}
	}
	return allErrs
}

func requireBlock[T any](block *T, path *field.Path, detail string) field.ErrorList {
	if block == nil {
		return field.ErrorList{field.Required(path, detail)}
	}
	return nil
}

func snapshotForbiddenWhenGit(spec *kollectdevv1alpha1.KollectSnapshotSinkSpec) []forbiddenBlock {
	return []forbiddenBlock{
		{field.NewPath("spec").Child("gitlab"), spec.GitLab != nil},
		{field.NewPath("spec").Child("objectStore"), spec.ObjectStore != nil},
		{field.NewPath("spec").Child("http"), spec.HTTP != nil},
	}
}

func snapshotForbiddenWhenGitLab(spec *kollectdevv1alpha1.KollectSnapshotSinkSpec) []forbiddenBlock {
	return []forbiddenBlock{
		{field.NewPath("spec").Child("git"), spec.Git != nil},
		{field.NewPath("spec").Child("objectStore"), spec.ObjectStore != nil},
		{field.NewPath("spec").Child("http"), spec.HTTP != nil},
	}
}

func snapshotForbiddenWhenObjectStore(spec *kollectdevv1alpha1.KollectSnapshotSinkSpec) []forbiddenBlock {
	return []forbiddenBlock{
		{field.NewPath("spec").Child("git"), spec.Git != nil},
		{field.NewPath("spec").Child("gitlab"), spec.GitLab != nil},
		{field.NewPath("spec").Child("http"), spec.HTTP != nil},
	}
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func SnapshotSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectSnapshotSink %q is invalid: %s", name, formatErrors(errs))
}

func DatabaseSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectDatabaseSink %q is invalid: %s", name, formatErrors(errs))
}

func EventSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectEventSink %q is invalid: %s", name, formatErrors(errs))
}

func ClusterSnapshotSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterSnapshotSink %q is invalid: %s", name, formatErrors(errs))
}

func ClusterDatabaseSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterDatabaseSink %q is invalid: %s", name, formatErrors(errs))
}

func ClusterEventSinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterEventSink %q is invalid: %s", name, formatErrors(errs))
}

// ValidateConnectionTestSinkRef requires exactly one family ref field.
func ValidateConnectionTestSinkRef(ref kollectdevv1alpha1.ConnectionTestSinkRef) field.ErrorList {
	base := field.NewPath("spec").Child("sinkRef")
	set := 0
	if ref.SnapshotSinkRef != "" {
		set++
	}
	if ref.DatabaseSinkRef != "" {
		set++
	}
	if ref.EventSinkRef != "" {
		set++
	}
	if set == 0 {
		return field.ErrorList{field.Required(base, "exactly one of snapshotSinkRef, databaseSinkRef, or eventSinkRef is required")}
	}
	if set > 1 {
		return field.ErrorList{field.Invalid(base, ref, "exactly one of snapshotSinkRef, databaseSinkRef, or eventSinkRef may be set")}
	}
	if ref.SnapshotSinkRef != "" {
		return validateSameNamespaceRef(ref.SnapshotSinkRef, base.Child("snapshotSinkRef"), "snapshotSinkRef")
	}
	if ref.DatabaseSinkRef != "" {
		return validateSameNamespaceRef(ref.DatabaseSinkRef, base.Child("databaseSinkRef"), "databaseSinkRef")
	}
	return validateSameNamespaceRef(ref.EventSinkRef, base.Child("eventSinkRef"), "eventSinkRef")
}
