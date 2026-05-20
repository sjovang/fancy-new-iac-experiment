## Summary

Describe the change and why it is needed.

## Traits alignment checklist

- [ ] Preserves reconciliation-first behavior (desired vs observed state remains explicit).
- [ ] Preserves dependency-DAG semantics (ordering and dependency modeling remain graph-based).
- [ ] Uses explicit provider operation contracts (not HTTP verb assumptions alone).
- [ ] Keeps authoring declarative-first, even when using conditionals/loops/transforms.
- [ ] Keeps architecture cloud-agnostic and schema-driven where relevant.
- [ ] References `docs/traits-spec.md` in design rationale.

## Validation notes

Describe how behavior was verified for this change.
