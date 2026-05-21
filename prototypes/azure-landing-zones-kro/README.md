# Azure Landing Zones — kro prototype

A working, scoped implementation of [Azure Landing Zones](https://github.com/Azure/Azure-Landing-Zones-Library) on top of [kro.run](https://kro.run).

**In scope**

- Management group hierarchy (`Microsoft.Management/managementGroups`)
- Custom Azure policy definitions (`Microsoft.Authorization/policyDefinitions`)
- Azure policy assignments scoped to management groups (`Microsoft.Authorization/policyAssignments`)

**Out of scope** (deliberate — see [`../../docs/research/azure-landing-zones-kro-prototype.md`](../../docs/research/azure-landing-zones-kro-prototype.md))

- Policy set definitions (initiatives), role definitions, identity / networking / management workloads.

## Layout

```
azure-landing-zones-kro/
├── README.md                     ← this file
├── mapping.md                    ← ALZ Library archetypes → prototype assignments
├── resourcegraph.yaml            ← kro ResourceGraphDefinition (top-level AzureLandingZone API)
├── resources/
│   ├── management-groups.yaml    ← MG hierarchy (alz → platform/landingzones/sandbox/decommissioned → children)
│   ├── policy-definitions.yaml   ← custom policy definitions referenced by assignments
│   └── policy-assignments.yaml   ← ALZ guardrail assignments scoped per MG / archetype
└── values/
    └── dev.yaml                  ← example AzureLandingZone instance input
```

## Source mapping

The graph is a mechanical translation of the ALZ Library sources at `platform/alz/`:

| ALZ source                                           | Becomes |
|------------------------------------------------------|---------|
| `architecture_definitions/alz.alz_architecture_definition.json` | The `management-groups.yaml` resource template (one CR per MG, `parent_id` becomes a CEL reference). |
| `archetype_definitions/<archetype>.alz_archetype_definition.json` | Determines which assignments in `policy-assignments.yaml` get a `scope` pointing at the MG that uses that archetype. |
| `policy_assignments/<name>.alz_policy_assignment.json` | One `ManagementGroupPolicyAssignment` entry per `(archetype, assignment)` pair in `policy-assignments.yaml`. |
| `policy_definitions/<name>.alz_policy_definition.json` | One `PolicyDefinition` CR per custom def in `policy-definitions.yaml` (the prototype includes one illustrative custom def — the rest are built-in ARM IDs and need no CR). |

`mapping.md` lists the full archetype → assignment matrix used by the prototype.

## Prerequisites (when applying for real)

The composition graph is engine-agnostic at the schema level, but the resource templates target specific CRDs. To apply this prototype against a real cluster:

1. A Kubernetes cluster with credentials for an Azure tenant where you have **Management Group Contributor** at the tenant root.
2. [`kro`](https://kro.run/docs/getting-started/installation/) installed.
3. The Upbound Crossplane Azure providers installed and configured with Azure credentials:
   - `xpkg.upbound.io/upbound/provider-azure-management` (supplies `ManagementGroup`)
   - `xpkg.upbound.io/upbound/provider-azure-authorization` (supplies `PolicyDefinition`, `PolicyAssignment`, `ManagementGroupPolicyAssignment`)

Apply order is irrelevant — kro and the provider controllers reconcile continuously and respect the inferred DAG.

## Apply workflow

```bash
# 1. install the ResourceGraphDefinition (registers the AzureLandingZone CRD)
kubectl apply -f resourcegraph.yaml
kubectl apply -f resources/

# 2. apply one or more AzureLandingZone instances
kubectl apply -f values/dev.yaml

# 3. observe
kubectl get azurelandingzones
kubectl get managementgroups.management.azure.upbound.io
kubectl get managementgrouppolicyassignments.authorization.azure.upbound.io
```

## Design notes

- The graph models **desired state**. Drift in any MG attribute (display name, parent), any policy assignment parameter, or any custom definition body is reverted by the underlying controllers on their next loop — no `apply`-style ceremony.
- MG hierarchy ordering, assignment→MG scoping, and assignment→definition references are **not encoded as `dependsOn` steps**. They are CEL references in the resource templates; kro builds the DAG from those references.
- Per-environment differences (root parent MG, DDoS plan ID, log analytics workspace, default location) are inputs on the `AzureLandingZone` schema — see `values/dev.yaml`. The graph itself does not change between environments.
- See [`../../docs/research/azure-landing-zones-kro-prototype.md`](../../docs/research/azure-landing-zones-kro-prototype.md) for the trait-by-trait alignment with `docs/traits-spec.md` and the rationale for picking the Upbound providers over ASO for this scope.
