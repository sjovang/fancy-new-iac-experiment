# CUE prototype

This is a mock CUE implementation of the reference Azure app.

## Layout

- `main.cue` — composition root wiring modules together
- `environments/dev.cue` — environment inputs
- `modules/network` — vnet/subnets/nsg/public ip
- `modules/identity` — user-assigned managed identity
- `modules/lb` — load balancer, backend pool, probe/rule
- `modules/compute` — availability set, NICs, two Linux VMs, backend pool attachments
- `modules/data` — PostgreSQL + storage account
- `modules/app` — Linux App Service Plan + frontend App Service

## Notes

- Intended for architecture exploration, not production deployment.
- Module outputs are shaped to make inter-module dependencies explicit.
