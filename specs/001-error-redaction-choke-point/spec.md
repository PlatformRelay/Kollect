# Feature Specification: Error-redaction choke-point

**Feature Branch**: `001-error-redaction-choke-point`

**Created**: 2026-09-24

**Status**: Draft

**Input**: Findings K-23, K-25, K-24 from the six-review unification register (`data/kollect-unify/report.md` section 4). Test lock: shared redaction contract + static parse messages.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Redaction at the status/Event boundary (Priority: P1)

As a cluster operator, a tenant-authored sink with a credential-bearing endpoint must never have that credential written into a `Kollect.dev` status condition message or a Kubernetes Event. Today free-form driver error text is written verbatim by five writers, and seven malformed-endpoint parse sites `%w`-wrap a `*url.ParseError` whose text contains the raw URL (including `user:token@`).

**Why this priority**: credentials persisted in etcd are readable by anyone with status read access, and the leak is one malformed endpoint away.

**Independent Test**: feed each named writer a synthetic error whose message embeds a credential-bearing URL plus a known secret token; assert the condition message and Event message reaching the API object contain neither.

**Acceptance Scenarios**:

1. **Given** an export error whose text is `dial failed for https://user:s3cret@host/repo`, **When** the inventory controller writes the Degraded/Synced condition, **Then** the message contains no `s3cret` and no `user:` userinfo.
2. **Given** the same error reaching a family sink connection test, **When** `ConnectionVerified=False` is written, **Then** the persisted message is redacted and still names the failing reason.
3. **Given** a KollectConnectionTest whose probe fails with a credential-bearing message, **When** the probe status is written, **Then** the message is redacted.

---

### User Story 2 - Static messages at URL parse sites (Priority: P2)

As a cluster operator, a malformed endpoint must produce a parse failure that identifies the failing kind but never echoes the endpoint.

**Why this priority**: this is the concrete leak in the finding.

**Independent Test**: for each named parse site, call it with an endpoint that both fails to parse and embeds userinfo; assert the returned error string matches a static message and contains neither the secret nor the host.

**Acceptance Scenarios**:

1. **Given** an endpoint `nats://user:tok@bad host`, **When** the NATS connect path parses it, **Then** the error text is static and carries no userinfo.
2. **Given** a NATS URL that parses cleanly but embeds userinfo, **When** the config is built, **Then** the config step rejects it with a static message (fail closed before connect).

---

### User Story 3 - Scrubber matches high-risk key stems (Priority: P3)

As a tenant, collected objects that carry bearer material under keys such as `authorization`, `Authorization`, `X-Authorization` or `bearerToken` must be redacted in the exported inventory, not only keys ending in a deny suffix.

**Why this priority**: K-24 is the collection-side twin of the same credential-leak class.

**Independent Test**: table test over `ScrubAttributes` covering the new stems plus a non-sensitive control key.

**Acceptance Scenarios**:

1. **Given** an attribute map with `authorization: Bearer eyJ...`, **When** scrubbed, **Then** the value is replaced by the redacted marker.
2. **Given** `metadata.name: foo`, **When** scrubbed, **Then** the value survives.

---

### Edge Cases

- Empty error, nil error, and errors with no URL must pass through byte-identical.
- Multi-line error text with several credential-bearing URLs in several schemes must be scrubbed at every occurrence.
- A known secret value that also appears as a substring of ordinary text must be masked (documented over-redaction trade-off).
- Unknown keys must never be treated as sensitive just for containing one of the two added stems.
- Redaction is a single pass and must not recurse or loop on adversarial input.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose one shared redaction helper (`internal/redact`) used by every path that turns an error or free-form message into persisted status or Event text.
- **FR-002**: All five writers named by K-23 (`sink_status.go`, `kollectinventory_controller.go`, `kollectclusterinventory_controller.go`, `family_sink_connection.go`, `kollectconnectiontest_controller.go`) MUST route their message text through the shared helper.
- **FR-003**: All seven named `url.Parse` sites MUST return static, endpoint-free messages; none may `%w`-wrap the parse error text.
- **FR-004**: The NATS config step MUST reject a URL carrying userinfo with a static message before any connect attempt.
- **FR-005**: The scrubber MUST redact keys equal to or prefixed by `authorization` or `bearer` (case- and separator-insensitive) in addition to the existing deny list.
- **FR-006**: Redaction MUST preserve the error's identity for classification: `errors.Is`/`errors.As`/`ClassOf`/`IsTerminal` results unchanged.
- **FR-007**: Redaction MUST leave text without credentials unchanged, byte for byte.

### Key Entities

- **Redaction contract**: a fixed set of patterns (URL userinfo for every scheme; known secret values) plus the placeholder, shared by git/postgres/nats/controller paths.
- **Static parse error**: a package-level sentinel per parse site, deliberately carrying no dynamic content.

## Constraints

- **SC-001**: No other batch's findings touched: the change set is confined to the redaction choke-point plus the scrubber stem list.
- **SC-002**: Every changed file under `internal/` is covered by a behavioural test added in the same change.
- **SC-003**: Existing error-taxonomy tests (`internal/errors`) stay green, proving classification is unchanged.

## Assumptions

- The eight policy recommendations taken by the captain (C-1..C-8) do not gate this lane: no operator opt-in flag or webhook change is required here.
- Status/Event text is the leak surface of record; log lines are out of scope for this batch.
