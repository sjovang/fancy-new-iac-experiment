# Shared Azure topology (applies to CUE and kro prototypes)

## Resource set

1. Network foundation: VNet, frontend/backend/data subnets, NSG, LB public IP.
2. Identity: one user-assigned managed identity (UAMI), attached where needed.
3. Load balancing: one public LB with backend pool, health probe, rule.
4. Compute backend: availability set + two Linux VMs + NICs in backend subnet; NICs join LB backend pool.
5. Data: PostgreSQL flexible server + app database + storage account.
6. Frontend app: Linux App Service Plan + App Service connected to frontend subnet.

## Dependency edges

- `network -> lb` (LB needs public IP)
- `network -> compute` (NIC subnet)
- `lb -> compute` (NIC backend pool association)
- `network -> data` (delegated/private subnet)
- `identity -> data` (managed identity attachment)
- `network -> app` (frontend subnet integration)
- `identity -> app` (managed identity attachment)
- `data -> app` (database/storage connection settings)

## Naming strategy

- Prefix: `${name}` from environment values.
- Examples: `${name}-vnet`, `${name}-lb`, `${name}-vm0`, `${name}-vm1`, `${name}-pg`, `${name}-asp`, `${name}-frontend`.

## Identity attachment points

- Linux VMs: UserAssigned identity on each VM resource.
- PostgreSQL server: UserAssigned identity on server.
- Frontend App Service: UserAssigned identity on site.
