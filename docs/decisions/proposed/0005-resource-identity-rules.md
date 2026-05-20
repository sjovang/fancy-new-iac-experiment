# 0005. Resource identity and stability rules

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #2 (dependency DAG), #4 (explicit lifecycle behavior), #5 (declarative-first authoring)
- **Related ADRs:** [0001](../accepted/0001-execution-surface.md), [0003](../accepted/0003-kro-authoring-surface.md), [0004](./0004-schema-ingestion.md)

## Context

We need stable identity semantics for resources created from graph expansion, references, and templating. Without explicit rules, rename/move scenarios produce noisy churn, broken dependencies, and unclear drift behavior.

## Options considered

### Option A — Name-only identity

- Summary: identity is inferred only from generated Kubernetes object names.
- Pros: simple to implement.
- Cons: fragile under refactors, poor rename semantics, high accidental replacement risk.

### Option B — Composite identity contract (recommended direction)

- Summary: identity is derived from explicit address components (graph path, logical resource id, key tuple, scope) with deterministic name projection.
- Pros: stable across non-breaking refactors, better diff/drift behavior, clearer plan semantics.
- Cons: needs explicit design and migration rules.

## Decision

**Proposed direction: Option B (composite identity contract).**

Scope for this ADR:

- Define identity components.
- Define replace-in-place vs recreate triggers.
- Define rename/move semantics and compatibility windows.

Out of scope:

- User-facing CLI formatting of identity deltas.

## Consequences

- **Positive**
  - Cleaner plans and lower reconciliation noise.
  - Stronger guarantees for DAG dependency stability.
- **Negative / accepted tradeoffs**
  - Additional complexity in planner/reconciler integration.
  - Requires explicit migration support when identity rules evolve.

## References

- `docs/traits-spec.md`
- `docs/research/implementation.md`
- `artifacts/research/adr-0004.md`
