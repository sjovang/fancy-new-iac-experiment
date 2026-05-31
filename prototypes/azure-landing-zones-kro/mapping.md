# Archetype → assignment mapping

Source: [`Azure-Landing-Zones-Library/platform/alz/archetype_definitions/`](https://github.com/Azure/Azure-Landing-Zones-Library/tree/main/platform/alz/archetype_definitions).

In the previous iteration of this prototype, this file enumerated a hand-curated subset of assignments. With the renderer-driven design, that information lives in two places:

- **The upstream library** at `libraryRef` in `landingzone.yaml` defines the full archetype matrix (each archetype JSON lists which policy assignments apply to MGs of that archetype).
- **`landingzone.yaml`** declares which of those assignments this deployment actually enables, by listing them under each MG's `policyAssignments:` field.

To extend the deployment you list more assignment names; you don't write code.

## Currently enabled assignments

The matrix below is whatever `landingzone.yaml` declares — see the source for the live list. The values shown here are the defaults committed in the input file at the time this doc was last refreshed.

| MG               | Archetype (upstream)          | Assignments enabled in this deployment |
|------------------|--------------------------------|----------------------------------------|
| `alz`            | `root`                         | *(none in this deployment)*            |
| `platform`       | `platform`                     | *(none in this deployment — extend with assignments from `policy_assignments/` that the upstream `platform` archetype lists)* |
| `connectivity`   | `connectivity`                 | `Enable-DDoS-VNET`                     |
| `identity`       | `identity`                     | `Deny-MgmtPorts-Internet`, `Deny-Public-IP` |
| `management`     | `management`                   | *(empty in upstream archetype)*        |
| `security`       | `security`                     | *(empty in upstream archetype)*        |
| `landingzones`   | `landing_zones`                | `Deny-IP-forwarding`, `Deny-Storage-http` |
| `corp`           | `corp`                         | `Audit-PeDnsZones`, `Deny-Public-IP-On-NIC` |
| `online`         | `online`                       | *(empty in upstream archetype)*        |
| `local`          | `local`                        | `Enforce-ALDO-Services`                |
| `sandbox`        | `sandbox`                      | *(empty in upstream archetype)*        |
| `decommissioned` | `decommissioned`               | *(empty — the upstream assignment targets a policy set definition, which is out of scope)* |

## Custom vs built-in policy definitions

The renderer treats them differently:

- **Custom definitions** (everything in `policy_definitions/*.alz_policy_definition.json` upstream — 149 files at the pinned ref) → one `PolicyDefinition` CR per file, scoped at the intermediate-root MG. Emitted regardless of whether any assignment uses them, so the upstream library is reflected verbatim and re-rendering after a library bump is a clean diff.
- **Built-in definitions** referenced by an assignment's `policyDefinitionId` (e.g. `/providers/Microsoft.Authorization/policyDefinitions/<guid>`) → passed through as ARM ID strings; no CR needed.
- **Built-in policy set definitions** (`policySetDefinitions/<guid>`) → also pass through as ARM ID strings, but custom policy set definitions (initiatives) shipped by the library are **not** vendored — that is out of scope.

The renderer detects which case applies by parsing the upstream assignment's `policyDefinitionId` against three patterns:

- `…/managementGroups/<placeholder>/providers/Microsoft.Authorization/policyDefinitions/<Name>` and `<Name>` is in the vendored set ⇒ CEL ref `${pd<Name>.status.atProvider.id}`.
- `/providers/Microsoft.Authorization/policy(Set)?Definitions/<id>` ⇒ pass through.
- Otherwise ⇒ warn and pass through (typically means the library wants a custom policy set definition that this prototype does not vendor).

## Extending

- New assignment on an existing MG → add one line to that MG's `policyAssignments` list in `landingzone.yaml`.
- New MG → add a block under `managementGroups`.
- Upstream library bump → bump `libraryRef`, re-render, review the diff.
- Cover policy set definitions (initiatives) → extend the renderer to also walk `policy_set_definitions/*.json` and emit a `PolicySetDefinition` CR for each, then update the assignment id-resolution logic to recognise custom set-definition refs.
