# ADR-0004 Decision 1 research: API versioning compatibility matrix

## Goal

Unblock ADR-0004 unresolved decision #1 (canonical IR versioning + compatibility policy) with a concise comparison across:

- Azure ARM
- Microsoft Graph
- AWS Cloud Control

## Comparison matrix

| API family | Versioning behavior | Breaking-change signal | Operational implications for IR |
|---|---|---|---|
| Azure ARM | Resource definitions use explicit `apiVersion`, and available properties vary by that version. Resource providers publish new API versions over time. | New API versions are introduced by providers; templates can stay pinned to older versions for consistent behavior. | IR must preserve per-resource source API version metadata. A generic “latest” assumption is unsafe across providers/types. |
| Microsoft Graph | Two active surfaces: `v1.0` (GA) and `beta` (preview). Beta can break without notice; non-backward-compatible GA changes drive major version increments. | Explicit major version increments for non-backward-compatible API changes; version deprecation is announced in advance. | IR/runtime contract should treat major-version boundaries as hard compatibility boundaries. Beta-like sources need stricter guardrails. |
| AWS Cloud Control | Resource types are versioned in the CloudFormation registry; accounts can manage versions and set a default type version. Schemas and handler contracts are version-scoped. | Version changes are surfaced via type-version lifecycle and default-version selection in the registry. | IR must capture source type version identity, not only type name, because behavior/schema may differ by activated default version. |

## Evidence notes

1. **Azure ARM**
   - ARM resources require `apiVersion`, and properties vary with API version.
   - Resource Explorer exposes valid API versions per resource type.
2. **Microsoft Graph**
   - `beta` is explicitly non-production and subject to breaking changes.
   - Non-backward-compatible changes are handled via API version increments.
3. **AWS Cloud Control**
   - CloudFormation registry supports listing type versions and setting default type versions.
   - Resource schemas and operation handlers are tied to resource type metadata.

## Recommendation for ADR-0004 decision #1

Adopt **strict major compatibility** for canonical IR/runtime compatibility in v0:

- Runtime accepts only IR documents with the same `irSchemaVersion.major`.
- Cross-major support is not implicit; it requires explicit adapters/migration tooling.
- IR should always carry source version identity (`apiVersion` / type version metadata) per resource to keep planning/reconciliation deterministic.

This is the lowest-risk policy across the three API ecosystems, which all show independent version churn and schema drift patterns.

## References

- Azure resource providers and types: <https://learn.microsoft.com/azure/azure-resource-manager/management/resource-providers-and-types>
- ARM template resource properties (`apiVersion` behavior): <https://learn.microsoft.com/azure/azure-resource-manager/templates/template-tutorial-add-resource#resource-properties>
- Microsoft Graph versioning and support policy: <https://learn.microsoft.com/graph/versioning-and-support>
- AWS Cloud Control resource types and version management: <https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/resource-types.html>
