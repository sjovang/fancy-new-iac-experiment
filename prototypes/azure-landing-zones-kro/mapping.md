# Archetype → assignment mapping

Source: [`Azure-Landing-Zones-Library/platform/alz/archetype_definitions/`](https://github.com/Azure/Azure-Landing-Zones-Library/tree/main/platform/alz/archetype_definitions).

This file is the matrix the prototype implements. Each row is rendered by `resources/policy-assignments.yaml` as one `ManagementGroupPolicyAssignment` CR, with `spec.forProvider.managementGroupId` pointing at the MG whose archetype is listed in the **Applied at MG** column.

The prototype ships **a representative subset** of assignments per archetype (typically 1–3 per archetype) so the graph is reviewable end-to-end. The full ALZ Library archetypes contain 1–53 assignments each — adding the remainder is a mechanical extension (one entry per assignment, same template).

| Archetype        | MG IDs using this archetype | Assignment (built-in policy definition unless noted) | In prototype? |
|------------------|------------------------------|------------------------------------------------------|---------------|
| `root`           | `alz`                        | (many; representative omitted for prototype clarity) | —             |
| `platform`       | `platform`                   | `Deploy-MDFC-DefSQL-AMA`, `Enable-AUM-CheckUpdates`  | ✅ (1 included) |
| `connectivity`   | `connectivity`               | `Enable-DDoS-VNET`                                   | ✅            |
| `identity`       | `identity`                   | `Deny-MgmtPorts-Internet`, `Deny-Public-IP`, `Deny-Subnet-Without-Nsg`, `Deploy-VM-Backup` | ✅ (2 included) |
| `management`     | `management`                 | *(empty in upstream archetype)*                      | n/a           |
| `security`       | `security`                   | *(empty in upstream archetype)*                      | n/a           |
| `landing_zones`  | `landingzones`               | `Deny-IP-forwarding`, `Deny-Storage-http`, `Enforce-TLS-SSL-Q225` … (53 upstream) | ✅ (2 included) |
| `corp`           | `corp`                       | `Audit-PeDnsZones`, `Deny-HybridNetworking`, `Deny-Public-Endpoints`, `Deny-Public-IP-On-NIC`, `Deploy-Private-DNS-Zones` | ✅ (2 included) |
| `online`         | `online`                     | *(empty in upstream archetype)*                      | n/a           |
| `local`          | `local`                      | `Enforce-ALDO-Services`                              | ✅            |
| `sandbox`        | `sandbox`                    | *(empty in upstream archetype)*                      | n/a           |
| `decommissioned` | `decommissioned`             | `Enforce-ALZ-Decomm`                                 | ✅            |

## Custom policy definitions

Most ALZ assignments target **built-in** Azure policy definitions referenced by their ARM ID (e.g. `/providers/Microsoft.Authorization/policyDefinitions/94de2ad3-...`); these need no CR.

The prototype includes **one** illustrative custom policy definition (`Deny-Public-IP-On-NIC` custom variant) in `resources/policy-definitions.yaml`, both to demonstrate the `PolicyDefinition`-then-`PolicyAssignment` dependency edge and to keep the graph reviewable.

Extending the prototype to cover all ~150 ALZ custom definitions is mechanical: one `PolicyDefinition` CR per definition file, plus a CEL reference from the assignment.
