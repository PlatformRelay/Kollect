# KollectProfile

**Scope:** Namespace · **Reconciled:** No (static schema) · **Short name:** —

Defines which Kubernetes GVK to watch and which attributes to extract via JSONPath or CEL
([ADR-0003](../adr/0003-cel-jsonpath-extraction.md), [ADR-0031](../adr/0031-namespaced-profiles.md)).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.targetGVK.group` | string | No | API group (empty for core) |
| `spec.targetGVK.version` | string | Yes | API version (e.g. `v1`) |
| `spec.targetGVK.kind` | string | Yes | Resource kind (e.g. `Deployment`) |
| `spec.attributes[]` | list | No | Extraction rules |
| `spec.attributes[].name` | string | Yes | Attribute key in export rows |
| `spec.attributes[].path` | string | Yes | JSONPath (`$.…`) or `cel:…` expression |
| `spec.attributes[].type` | string | No | Hint: `string`, `int`, `list`, `map`, … |
| `spec.attributes[].optional` | bool | No | Non-fatal when extraction yields no value |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| *(none wired)* | — | Static CR — no controller updates status today |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollectprofiles` | Team users read schemas in their namespace |
| `create`, `update`, `patch`, `delete` | `kollectprofiles` | Profile authors in release namespace |

## Samples

- [`config/samples/kollect_v1alpha1_kollectprofile.yaml`](../../config/samples/kollect_v1alpha1_kollectprofile.yaml) — Deployment
- [`config/samples/kollect_v1alpha1_kollectprofile_argo-application-summary.yaml`](../../config/samples/kollect_v1alpha1_kollectprofile_argo-application-summary.yaml) — Argo CD Application
- Walkthrough: [Deployment inventory](../examples/deployment-inventory.md)

## Failure modes

> **TODO:** Document webhook validation errors (invalid GVK, empty attribute path, CEL compile failures).
