# Architecture → archetype → assignment mapping

Source: [`Azure-Landing-Zones-Library/platform/alz/architecture_definitions/`](https://github.com/Azure/Azure-Landing-Zones-Library/tree/main/platform/alz/architecture_definitions) and [`archetype_definitions/`](https://github.com/Azure/Azure-Landing-Zones-Library/tree/main/platform/alz/archetype_definitions).

The prototype no longer encodes any of this matrix by hand. The renderer walks the merged library set:

- `architecture_definitions/<name>.alz_architecture_definition.json` defines the MG hierarchy and which archetypes each MG carries.
- `archetype_definitions/<archetype>.alz_archetype_definition.json` lists `policy_assignments`, `policy_definitions`, `policy_set_definitions`, `role_definitions`.
- The MG's effective assignment list is the union of every archetype's `policy_assignments`, modulo `archetypeOverrides`, `policyAssignmentsToDisable`, and any custom archetypes you defined in `landingzone.yaml`.

## Default ALZ architecture (as rendered by this prototype)

| MG               | Archetype(s) (upstream)        | Effective assignment count after rendering | Notes |
|------------------|--------------------------------|---------------------------------------------|-------|
| `alz`            | `root`                         | 0 | Empty upstream. |
| `platform`       | `platform`                     | 40 | Mostly `Enforce-GR-*` initiatives — most target custom policy set definitions, which the renderer warns about and passes through unchanged. |
| `connectivity`   | `connectivity`                 | 1 | `Enable-DDoS-VNET`. |
| `identity`       | `identity`                     | 4 | `Deny-MgmtPorts-Internet`, `Deny-Public-IP`, `Deny-Subnet-Without-Nsg`, `Deploy-VM-Backup`. |
| `management`     | `management`                   | 0 | Empty upstream. |
| `security`       | `security`                     | 0 | Empty upstream. |
| `landingzones`   | `landing_zones`                | 53 | Largest archetype. |
| `corp`           | `corp`                         | 5 | `Audit-PeDnsZones`, `Deny-HybridNetworking`, `Deny-Public-Endpoints`, `Deny-Public-IP-On-NIC`, `Deploy-Private-DNS-Zones`. |
| `online`         | `online`                       | 0 | Empty upstream. |
| `local`          | `local`                        | 1 | `Enforce-ALDO-Services`. |
| `sandbox`        | `sandbox`                      | 1 | `Enforce-ALZ-Sandbox` (custom initiative — out of scope). |
| `decommissioned` | `decommissioned`               | 1 | `Enforce-ALZ-Decomm` (custom initiative — out of scope). |

Total: **123** assignments emitted from the default `alz` architecture, plus **4** referenced custom policy definitions and **12** management groups → 139 resources, ~3 500 lines.

## How the renderer resolves an assignment's policy id

The renderer reads `properties.policyDefinitionId` from the merged library assignment JSON and matches one of three patterns:

| Pattern | Action |
|---------|--------|
| `/providers/Microsoft.Management/managementGroups/<placeholder>/providers/Microsoft.Authorization/policyDefinitions/<Name>` and `<Name>` is in the merged `policy_definitions/` set | Emit a CEL ref `${pd<Name>.status.atProvider.id}` and add the custom `PolicyDefinition` CR to the graph. |
| `/providers/Microsoft.Authorization/policy(Set)?Definitions/<id>` | Pass through as a literal ARM ID (built-in). |
| `/providers/Microsoft.Management/managementGroups/<placeholder>/providers/Microsoft.Authorization/policySetDefinitions/<Name>` | Warn (custom initiatives are out of scope); pass through unchanged. |
| Anything else | Warn; pass through. |

## Extending

| Goal | How |
|------|-----|
| Pin to a newer upstream library | Bump `libraries[0].ref` in `landingzone.yaml`, re-render. |
| Add your own guardrails | Add a second `libraries` entry pointing at your repo. Your archetypes/assignments/definitions override the upstream ones by filename. |
| Replace the `corp` archetype | Provide a `corp` entry under `archetypes:`. The renderer treats a same-named entry as a replacement. |
| Tweak `Audit-PeDnsZones` parameters for `corp` only | Use `policyAssignmentsToModify.corp.Audit-PeDnsZones.parameters: {...}`. |
| Drop one assignment from a library archetype | Use `archetypeOverrides.<archetype>.remove: [Assignment]`. |
| Skip one assignment for one MG only | Use `policyAssignmentsToDisable.<mgName>: [Assignment]`. |
| Add new MG with a custom archetype | Append to `managementGroups` and reference your archetype in `archetypes`. |
| Cover initiatives (policy set definitions) | Extend the renderer to walk `policy_set_definitions/` and emit `PolicySetDefinition` CRs, then update the policy-id-resolution logic to recognise custom set-def refs. (Out of scope today.) |
