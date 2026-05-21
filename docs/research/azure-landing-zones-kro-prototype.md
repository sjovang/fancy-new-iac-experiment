# Research report: Azure Landing Zones (scoped) on kro.run

**Date:** 2026-05-21
**Scope of prototype:** management groups, Azure policy definitions, Azure policy assignments
**Reference source:** [`Azure/Azure-Landing-Zones-Library`](https://github.com/Azure/Azure-Landing-Zones-Library) — `platform/alz`
**Composition engine:** [kro.run](https://kro.run)
**Prototype location:** [`prototypes/azure-landing-zones-kro/`](../../prototypes/azure-landing-zones-kro/)

## Executive answer

A useful **scoped** implementation of Azure Landing Zones (ALZ) on kro is achievable today by:

1. Translating the ALZ Library’s **architecture definition** (management group hierarchy) and **archetype definitions** (the lists of policy assignments / definitions attached to each MG) from JSON into a single kro `ResourceGraphDefinition` that exposes an `AzureLandingZone` higher-level API.
2. Backing each leaf resource with a Kubernetes CRD that maps to the corresponding ARM type (`Microsoft.Management/managementGroups`, `Microsoft.Authorization/policyDefinitions`, `Microsoft.Authorization/policyAssignments`).
3. Letting kro perform **DAG inference and continuous reconciliation** across the leaves so MG hierarchy ordering, assignment-to-definition references, and parent→child MG dependencies are honored automatically.

This satisfies the canonical traits in [`docs/traits-spec.md`](../traits-spec.md): reconciliation-first, dependency-DAG planning, thin provider model, declarative-first authoring, and cloud-agnostic core (kro is cloud-neutral; provider CRDs are the thin schema-driven layer).

## Why this scope first

The ALZ Library splits cleanly into independent concern areas. `platform/alz` contains:

| Concern                                  | Path                              | In prototype? |
|------------------------------------------|-----------------------------------|---------------|
| Management group hierarchy               | `architecture_definitions/`       | ✅            |
| Archetype → assignment mapping           | `archetype_definitions/`          | ✅            |
| Policy assignments                       | `policy_assignments/`             | ✅            |
| Policy definitions (custom)              | `policy_definitions/`             | ✅            |
| Policy set definitions (initiatives)     | `policy_set_definitions/`         | out of scope  |
| Role definitions                         | `role_definitions/`               | out of scope  |
| Default values / overrides               | `alz_policy_default_values.json`  | partial (example overrides only) |

The three included concerns form the **minimum viable ALZ control surface**: once you can declaratively create MGs and apply guardrail policies, you have the Landing Zone "skeleton" without yet wiring identity, networking, or platform workloads.

## Mapping ALZ source → kro graph

### Source shape

`architecture_definitions/alz.alz_architecture_definition.json` defines a flat list of MG entries with `id`, `parent_id`, `display_name`, and an `archetypes` array. The reference hierarchy (verified against the library `main`) is:

```
alz (root)
├── platform
│   ├── connectivity
│   ├── identity
│   ├── management
│   └── security
├── landingzones
│   ├── corp
│   ├── online
│   └── local
├── sandbox
└── decommissioned
```

Each archetype JSON (e.g. `connectivity.alz_archetype_definition.json`) is shaped like:

```json
{
  "name": "connectivity",
  "policy_assignments": ["Enable-DDoS-VNET"],
  "policy_definitions": [],
  "policy_set_definitions": [],
  "role_definitions": []
}
```

Each `policy_assignment` JSON (e.g. `Enable-DDoS-VNET.alz_policy_assignment.json`) is an ARM `Microsoft.Authorization/policyAssignments@2024-04-01` resource carrying `properties.policyDefinitionId`, `properties.parameters`, `properties.scope`, `identity`, etc.

### Target shape

The prototype expresses the same intent as one kro `ResourceGraphDefinition`:

- **Schema:** `AzureLandingZone` — top-level API with `rootParentId`, `prefix`, `location`, and per-archetype overrides (e.g. `ddosPlanId`, `logAnalyticsWorkspaceId`).
- **Resources:** one entry per ALZ management group plus one entry per policy assignment / custom definition referenced by that MG’s archetype.
- **Edges:** kro infers them from CEL references. Examples:
  - Child MG references parent MG → ensures hierarchy ordering.
  - Policy assignment references the MG it is scoped to → ensures the MG exists first.
  - Policy assignment references a `PolicyDefinition` CR (for custom defs) → ensures the definition exists first.

This is a direct, mechanical translation of the JSON sources into a declarative graph. No imperative “run order” is encoded; the engine derives it.

## Provider / CRD selection (the hard part)

The prototype needs Kubernetes CRDs that actually reconcile the three ARM types. The candidates and current state:

| Need                                              | ASO (`azure-service-operator`) | Upbound `provider-azure-management` / `provider-azure-authorization` | Notes |
|---------------------------------------------------|--------------------------------|----------------------------------------------------------------------|-------|
| `Microsoft.Management/managementGroups`           | **Not supported** (no `management.azure.com` group in ASO v2 reference index) | `ManagementGroup` (`management.azure.upbound.io/v1beta1`) | Verified by searching ASO’s supported-resources index. |
| `Microsoft.Authorization/policyDefinitions`       | **Not supported** as of current `main`              | `PolicyDefinition` (`authorization.azure.upbound.io/v1beta1`) | The only `PolicyAssignment` in ASO is `RedisAccessPolicyAssignment` and similar cache/service-specific types — not Azure Policy. |
| `Microsoft.Authorization/policyAssignments`       | **Not supported**              | `PolicyAssignment`, `ManagementGroupPolicyAssignment`, `SubscriptionPolicyAssignment` (same API group) | `ManagementGroupPolicyAssignment` is exactly what we need — scope is the MG. |

**Decision for the prototype:** back the kro graph with the **Upbound Crossplane Azure providers** (`provider-azure-management` and `provider-azure-authorization`). Rationale:

- They are the only mainstream operator family that currently exposes the three ARM types as first-class CRDs.
- They are **upjet-generated from the Terraform AzureRM provider schemas** — i.e. schema-driven integration, which aligns with the spec invariant on consuming vendor schemas (OpenAPI / equivalent metadata).
- They are reconciler-based, satisfying the reconciliation-first invariant.
- They run alongside kro inside the same cluster — kro composes any CRD, so this is a supported pairing.

This intentionally diverges from the prior research note (`can-kro-https-azure-github-io-azure-serv.md`), which proposed kro + ASO. For an **Azure-only workload-resource** prototype ASO is still the better fit, but for **ALZ control-plane resources** (MGs and Azure Policy) ASO is currently a non-starter. Once ASO adds the `management.azure.com` and `authorization.azure.com` policy resources, the prototype graph could be retargeted by swapping the resource templates without altering the schema or graph shape — a clean demonstration of why thin, declarative resource templates beat provider-coupled orchestration.

## How this preserves the five design invariants

| Invariant (from `docs/traits-spec.md`)                          | How the prototype preserves it |
|-----------------------------------------------------------------|--------------------------------|
| **1. Continuous reconciliation**                                | All resources are Kubernetes CRDs reconciled by their respective controllers; kro reconciles the graph itself. Drift in any MG attribute or policy parameter is corrected on the next loop. |
| **2. Dependency-DAG planning**                                  | kro infers edges from CEL references in the resource templates (`${resources.mg_platform.status.id}`, `${resources.def_xxx.status.id}`). No imperative ordering. |
| **3. Provider operation contract (thin, API-native)**           | Resource templates are pure CRD bodies — no provider-specific orchestration code in the prototype. Lifecycle (create/update/delete) is owned by the provider controller, not by the composition. |
| **4. Declarative-first authoring (loops/conditionals allowed)** | The graph uses kro’s declarative composition; per-archetype variation is expressed as separate template entries rather than imperative steps. Per-environment differences ride on the `AzureLandingZone` schema, not procedural overrides. |
| **5. Cloud-agnostic core**                                      | kro itself is cloud-neutral. Azure specificity is confined to the CRD types referenced in resource templates. The graph schema (`AzureLandingZone`) is a vendor-neutral surface that could be retargeted to another provider by substituting resource templates. |

## Out-of-scope (deliberate)

- **Policy set definitions (initiatives)** — same shape as policy definitions; trivial to add later but adds noise.
- **Role definitions / RBAC** — separate concern area, not requested.
- **Identity, networking, management workloads** — the Landing Zones platform stack is large; this prototype is the control-plane skeleton only.
- **State backend / drift remediation strategy** — provided by the underlying controllers; not re-implemented at the composition layer.
- **Live `terraform apply`-style validation** — the prototype is a working set of manifests; verifying behavior against a real Azure tenant is outside this report.

## References

- Azure Landing Zones Library (`platform/alz`): <https://github.com/Azure/Azure-Landing-Zones-Library/tree/main/platform/alz>
- ALZ Library README (architecture + archetype overview): <https://github.com/Azure/Azure-Landing-Zones-Library/blob/main/platform/alz/README.md>
- kro overview and `ResourceGraphDefinition`: <https://kro.run/docs/overview/>
- Azure Service Operator supported resources index: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/reference/_index.md>
- Upbound `provider-azure-management` (ManagementGroup CRD)
- Upbound `provider-azure-authorization` (PolicyDefinition, PolicyAssignment, ManagementGroupPolicyAssignment CRDs)
- Prior research: [`can-kro-https-azure-github-io-azure-serv.md`](./can-kro-https-azure-github-io-azure-serv.md)
- Canonical spec: [`../traits-spec.md`](../traits-spec.md)
