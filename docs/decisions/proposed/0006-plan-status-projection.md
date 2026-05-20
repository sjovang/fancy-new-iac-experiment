# 0006. Plan and status projection model

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #1 (continuous reconciliation), #2 (dependency DAG), #5 (declarative-first UX)
- **Related ADRs:** [0001](../accepted/0001-execution-surface.md), [0003](../accepted/0003-kro-authoring-surface.md), [0005](./0005-resource-identity-rules.md)

## Context

The project requires both planner-style previews and reconciliation-style runtime convergence. We need one projection model that maps graph intent into:

- plan output (ordering, deltas, blockers),
- runtime conditions/status,
- consistent diagnostics across authoring/planning/runtime phases.

## Options considered

### Option A — Reconciler-only status, no explicit plan projection

- Summary: depend on runtime status/events without first-class plan semantics.
- Pros: simpler implementation.
- Cons: reintroduces noisy, hard-to-debug behavior this project is explicitly trying to avoid.

### Option B — Unified projection contract (recommended direction)

- Summary: define explicit plan-step model and map each step/result into bounded condition/reason vocabulary at runtime.
- Pros: clearer user mental model, better debuggability, consistent UX across phases.
- Cons: requires upfront schema and strict discipline on status taxonomy.

## Decision

**Proposed direction: Option B (unified projection contract).**

Scope for this ADR:

- Plan-step taxonomy and ordering semantics.
- Condition/reason vocabulary and observed-generation discipline.
- Mapping between plan outcomes and runtime status.

Out of scope:

- UI implementation details (CLI/table/rendering specifics).

## Consequences

- **Positive**
  - Stronger explainability and lower operational ambiguity.
  - Better compatibility with GitOps/K8s condition-based tooling.
- **Negative / accepted tradeoffs**
  - Higher upfront design effort.
  - Requires compatibility/versioning policy for status schema evolution.

## References

- `docs/research/usability.md`
- `docs/research/implementation.md`
- `docs/provider-operation-model.md`
