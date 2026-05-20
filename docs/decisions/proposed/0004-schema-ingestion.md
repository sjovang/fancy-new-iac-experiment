# 0004. Schema ingestion for kro-oriented authoring

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #3 (thin API layers), #4 (explicit operation control), #6 (cloud-agnostic schema consumption)
- **Related ADRs:** [0001](../accepted/0001-execution-surface.md), [0003](../accepted/0003-kro-authoring-surface.md)

## Context

With kro chosen as the authoring surface ([ADR-0003](../accepted/0003-kro-authoring-surface.md)), we need a concrete schema-ingestion model that turns upstream API descriptions (OpenAPI/Swagger and equivalent sources) into kro-friendly, typed authoring and validation artifacts.

The decision must preserve cloud-agnostic architecture while avoiding hand-maintained, per-resource logic as the default path.

## Options considered

### Option A — Runtime-only schema checks (no ingestion pipeline)

- Summary: rely only on schemas already present in the cluster and kro validation.
- Pros: lowest initial implementation effort.
- Cons: weak authoring ergonomics, no consistent cross-vendor normalization, poor trait #6 alignment.

### Option B — Provider-specific monolithic generators

- Summary: build provider-specific generators that emit all types/artifacts directly.
- Pros: strong per-provider control.
- Cons: high maintenance cost, drifts toward bespoke babysitting, weaker cloud-neutral architecture.

### Option C — Canonical IR + generated kro artifacts (hybrid)

- Summary: ingest upstream schemas into a provider-agnostic intermediate model (IR), apply deterministic transforms/overrides, then generate kro-facing typed artifacts and validation metadata.
- Pros: strongest trait #6 alignment, scalable cross-vendor model, explicit transform layer for imperfect source specs.
- Cons: requires IR/compiler design and transform governance.

## Decision

**Proposed direction: Option C (canonical IR + generated kro artifacts).**

Scope for this ADR:

- Define supported source formats in v0.
- Define canonical IR contract.
- Define transform/override mechanism.
- Define generated artifact contract consumed by kro-oriented authoring.

Out of scope:

- Transformation extension runtime model (follow-on ADR).
- CLI packaging and distribution details.

## Consequences

- **Positive**
  - Preserves cloud-agnostic architecture while keeping kro-first UX.
  - Allows deterministic, testable schema normalization.
  - Enables stronger static feedback before reconciliation.
- **Negative / accepted tradeoffs**
  - Additional compiler/tooling complexity.
  - Requires long-term maintenance of transform rules.
- **Follow-up work**
  - Identity/stability rules for expanded resources (ADR-0005).
  - Plan/status projection semantics (ADR-0006).
  - Transform extension model (ADR-0007).

## References

- `docs/traits-spec.md`
- `docs/research/implementation.md`
- `docs/research/usability.md`
- kro docs: <https://kro.run/docs/overview/>
- ADR-0004 research report: `artifacts/research/adr-0004.md`
