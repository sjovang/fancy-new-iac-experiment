# 0005. Resource identity and stability rules

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #1 (continuous reconciliation), #2 (dependency DAG), #4 (explicit lifecycle behavior), #5 (declarative-first authoring), #6 (cloud-agnostic schema consumption)
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

v0 baseline included in this ADR:

- Identity is derived from a composite address, not object name alone.
- Kubernetes object names are deterministic projections of identity.
- Replace-vs-update behavior is determined from identity and immutable-field rules.
- Rename/move behavior is explicit and plan-visible.

### Composite identity contract (v0 minimum)

Each managed resource identity is computed from canonical components:

- **Graph address:** stable path within the declarative graph expansion.
- **Logical resource id:** stable authoring-level identifier for the resource intent.
- **Scope address:** parent/scope identity (for scoped resources).
- **External key tuple:** upstream provider key fields required to address the real resource.
- **Type discriminator:** resource kind/type and version identity needed for unambiguous matching.

The deterministic Kubernetes name is a projection of these components for operational convenience. It is not the source-of-truth identity.

### Replace vs update rules (v0 minimum)

- **Recreate required** when any identity component changes, or when provider-declared immutable key fields change.
- **In-place update allowed** when only mutable desired-state fields change.
- Planner output must surface this explicitly so replacements are predictable before reconciliation.

### Rename and move semantics (v0 minimum)

- Renames/moves that do not change identity components are treated as non-breaking metadata changes.
- Renames/moves that change identity components are treated as planned replace operations, with dependency-aware ordering from the DAG.
- Compatibility behavior for cross-identity migration is policy-driven and remains explicit in plan output.

Scope for this ADR:

- Define identity components.
- Define replace-in-place vs recreate triggers.
- Define rename/move semantics and compatibility windows.

Out of scope:

- User-facing CLI formatting of identity deltas.
- Full migration-engine implementation details for cross-identity remap workflows.

## Invariant alignment

This proposal preserves the five design invariants in `docs/traits-spec.md`:

1. **Reconciliation-first:** stable identity makes repeated convergence loops deterministic and reduces false drift churn.
2. **DAG-preserving:** identity changes are explicit graph events, enabling safe re-planning and dependency ordering.
3. **Explicit operation contracts:** replace vs update behavior is modeled explicitly, not inferred from transport verbs.
4. **Declarative-first authoring:** users declare intent and identity anchors declaratively; no imperative rename scripts are required by default.
5. **Cloud-agnostic core:** identity components are provider-neutral abstractions, with provider specifics constrained to external key metadata.

## Explicit unresolved decisions (reason ADR remains Proposed)

1. Exact compatibility-window policy for identity remaps (e.g., alias lifetimes and safety checks).
2. Conflict-resolution policy when two resources converge on the same computed identity tuple.
3. Versioning strategy for identity-contract evolution across generator/runtime releases.
4. Operational policy for optional assisted migration vs strict replace-only behavior.

## Consequences

- **Positive**
  - Cleaner plans and lower reconciliation noise.
  - Stronger guarantees for DAG dependency stability.
  - Predictable replace semantics reduce accidental destructive changes.
- **Negative / accepted tradeoffs**
  - Additional complexity in planner/reconciler integration.
  - Requires explicit migration support when identity rules evolve.
  - Deterministic identity projection increases contract-governance burden.

## References

- `docs/traits-spec.md`
- `docs/research/implementation.md`
