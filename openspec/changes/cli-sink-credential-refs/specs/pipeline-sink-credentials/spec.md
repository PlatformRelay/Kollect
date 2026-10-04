# Spec Delta

## Purpose

Defines how kollect-pipeline resolves the credential and CA references of its sink from Secret
manifests in the config directory, so that a sink manifest gives the backend the same material in
the CLI as in the operator, and a missing reference fails instead of being dropped.

## ADDED Requirements

### Requirement: CSC-1 Type-specific credentials override the default secret

For a git sink, the CLI SHALL give the backend the data of `spec.git.auth.secretRef` when it is
set, and the data of `spec.secretRef` otherwise, matching the operator.

#### Scenario: Only git.auth.secretRef is set

- **WHEN** a git sink sets `git.auth.secretRef` to Secret `git-auth` with key `token` and sets no `secretRef`
- **THEN** the backend is built with the `git-auth` data

#### Scenario: Both references are set

- **WHEN** a git sink sets `secretRef: default-creds` and `git.auth.secretRef: git-auth`
- **THEN** the backend is built with the `git-auth` data and SHALL NOT receive the `default-creds` data

### Requirement: CSC-2 The CA secret reaches the backend

The CLI SHALL give the backend the CA from `tls.caBundle` when set, else from the Secret named by
`tls.caSecretRef` (first of keys `tls.crt`, `ca.crt`, `ca.pem`).

#### Scenario: CA from a Secret manifest

- **WHEN** a git sink sets `tls.caSecretRef: private-ca` and the config directory holds Secret `private-ca` with key `ca.crt`
- **THEN** the constructed git backend's CA bundle is that PEM

#### Scenario: Inline bundle wins

- **WHEN** a sink sets both `tls.caBundle` and `tls.caSecretRef`
- **THEN** the backend receives the inline bundle

### Requirement: CSC-3 Missing references fail before the backend is built

When `spec.secretRef`, `git.auth.secretRef` or `tls.caSecretRef` names a Secret that is not in the
config directory, the CLI SHALL fail the run with an error naming the reference, and SHALL NOT
build the backend or export.

#### Scenario: Missing CA secret

- **WHEN** `tls.caSecretRef` names a Secret that is not in the config directory
- **THEN** the run fails naming that Secret, and no backend is built

#### Scenario: Missing git auth secret

- **WHEN** `git.auth.secretRef` names a Secret that is not in the config directory
- **THEN** the run fails naming that Secret

### Requirement: CSC-4 Environment placeholders apply to every referenced Secret

The `${env:VAR}` substitution SHALL apply to the git auth Secret and the CA Secret as it does to
`spec.secretRef`, and an unset variable SHALL fail the run.

#### Scenario: Token from the environment through git.auth.secretRef

- **WHEN** Secret `git-auth` has `token: ${env:KOLLECT_GIT_TOKEN}` and the variable is set
- **THEN** the backend receives the variable's value, not the placeholder

### Requirement: CSC-5 The CLI resolves what the operator resolves

For the same snapshot sink spec and the same Secrets in the sink's namespace, referenced without an
explicit namespace or with the sink's namespace, the credential data and CA the CLI gives the backend
SHALL equal what the operator's resolver produces, and the CLI SHALL fail when the operator's
resolver fails. Two differences are intended and out of this scope: an unqualified reference in the
CLI matches a config-dir Secret of that name in any namespace (unchanged CLI matching), and a
reference with an empty name fails in the CLI where the operator treats it as unset.

#### Scenario: Parity over reference combinations

- **WHEN** the CLI and the operator resolver are given each combination of `secretRef`, `git.auth.secretRef`, `tls.caBundle` and `tls.caSecretRef`, present or missing
- **THEN** both produce the same data and CA, or both fail

### Requirement: CSC-6 The backend is released on every export outcome

The CLI SHALL close the backend after export, whether the export succeeded or failed.

#### Scenario: Export fails

- **WHEN** the export returns errors
- **THEN** the backend is still closed
