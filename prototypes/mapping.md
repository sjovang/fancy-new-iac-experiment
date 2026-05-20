# CUE ↔ kro mapping

| Infrastructure domain | CUE location | kro location |
| --- | --- | --- |
| Composition root | `cue/main.cue` | `kro/resourcegraph.yaml` |
| Environment values | `cue/environments/dev.cue` | `kro/values/dev.yaml` |
| Network | `cue/modules/network/network.cue` | `kro/resources/network.yaml` |
| Identity | `cue/modules/identity/identity.cue` | `kro/resources/identity.yaml` |
| Load balancer | `cue/modules/lb/lb.cue` | `kro/resources/lb.yaml` |
| Compute (availability set + 2 VMs) | `cue/modules/compute/compute.cue` | `kro/resources/compute.yaml` |
| Data (PostgreSQL + storage) | `cue/modules/data/data.cue` | `kro/resources/data.yaml` |
| Frontend app (ASP + App Service) | `cue/modules/app/app.cue` | `kro/resources/app.yaml` |

## Consistency checkpoints

1. Both prototypes model the same seven primary resource groups (network, identity, lb, compute, data, app, root composition).
2. Both enforce backend compute as two explicit VMs in an availability set (no VMSS).
3. Both wire managed identity into compute, data, and app components.
4. Both preserve dependency ordering from `prototypes/topology.md`.
