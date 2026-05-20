# kro-style prototype

This folder models the same Azure topology as `../cue`, but as a single resource graph definition with domain resource templates.

## Layout

- `resourcegraph.yaml` — graph root and dependency ordering
- `resources/*.yaml` — domain templates (network, identity, compute, lb, app, data)
- `values/dev.yaml` — example input values

## Notes

- This is a mock shaped after kro concepts (graph root + composed resources).
- Template bodies are intentionally schematic and focused on dependency and wiring semantics.
