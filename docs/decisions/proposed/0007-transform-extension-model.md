# 0007. Transform and extension model

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #4 (operation control), #5 (declarative-first with rich constructs), #6 (schema-driven ergonomics)
- **Related ADRs:** [0003](../accepted/0003-kro-authoring-surface.md), [0004](./0004-schema-ingestion.md)

## Context

kro + CEL provides strong declarative composition, but some scenarios require higher-order transformations beyond inline expressions. We need a bounded extension model that preserves declarative intent and graph observability.

## Options considered

### Option A — CEL-only forever

- Summary: disallow any extension surface beyond CEL and static templates.
- Pros: simplest trust model.
- Cons: likely insufficient for advanced transforms and schema adaptation at scale.

### Option B — Bounded extension layer (recommended direction)

- Summary: define controlled transform hooks with explicit inputs/outputs, deterministic behavior, and visible dependency edges.
- Pros: richer capability without collapsing into imperative scripts.
- Cons: requires security/sandbox and contract governance.

## Decision

**Proposed direction: Option B (bounded extension layer).**

Scope for this ADR:

- Allowed extension points.
- Determinism and side-effect constraints.
- Error/status integration into plan and reconcile flows.

Out of scope:

- Final execution engine choice (process, wasm, etc.).

## Consequences

- **Positive**
  - Supports complex transformations without abandoning declarative-first principles.
  - Enables clearer growth path for advanced users.
- **Negative / accepted tradeoffs**
  - More moving parts in validation/security model.
  - Requires strict guardrails to avoid hidden imperative behavior.

## References

- `docs/declarative-authoring-guidelines.md`
- `docs/provider-operation-model.md`
- `artifacts/research/adr-0004.md`
