# Prototypes

This directory contains parallel mock implementations of the same Azure application topology:

- `cue/` — CUE-based composition with domain modules.
- `kro/` — kro-style resource graph composition.

Both prototypes model the same resources and dependency intent:

- Linux App Service Plan
- Frontend App Service
- PostgreSQL
- Storage Account
- Load Balancer
- 2 Linux VMs in an Availability Set as LB backend pool
- User Assigned Managed Identity
- Full supporting networking (VNet, subnets, NSG, NIC/LB associations, public IP)
