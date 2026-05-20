# 0004. Schema ingestion for kro-oriented authoring

- **Status:** Proposed
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #1 (continuous reconciliation), #2 (dependency DAG), #3 (thin API layers), #4 (explicit operation control), #5 (declarative-first authoring), #6 (cloud-agnostic schema consumption)
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

### Option D — Canonical IR + generic APIResource primary surface

- Summary: ingest into canonical IR, but emit a single generic `type@version/body` authoring shape as the main v0 surface.
- Pros: fastest bootstrap and smallest generated type surface.
- Cons: weaker static guarantees and less ergonomic kro-first authoring for common users.

## Decision

**Proposed direction: Option C, refined as _canonical IR -> typed kro-facing artifacts per resource_ (planning Option 1).**

v0 baseline included in this ADR:

- **Source scope:** OpenAPI 3.x primary; Swagger 2.x supported via deterministic normalization.
- **Generation strategy:** typed kro-facing artifacts per resource from canonical IR (generic-only surface is not the v0 primary).
- **Operation mapping:** lifecycle defaults are method-derived, but always materialized as explicit IR operation profiles.
- **Override policy:** explicit overrides for non-standard APIs with precedence:
  1. resource-level override,
  2. provider-profile override,
  3. method-derived default.

### Canonical IR contract (v0 minimum)

The canonical IR must represent behavior and graph semantics explicitly, independent of transport quirks:

- Resource identity metadata (stable logical address, scope/parent context, key fields).
- Desired-state schema and validation metadata normalized from source specs.
- Explicit operation profile for `Observe`, `Create`, `Update`, `Delete`, plus optional `actions`.
- Dependency metadata required for stable DAG construction and planning.
- Generated-artifact projection metadata for kro-facing typed outputs.

This preserves explicit lifecycle semantics while still allowing method-derived defaults as a convenience input.

Scope for this ADR:

- Define supported source formats in v0.
- Define canonical IR contract.
- Define transform/override mechanism.
- Define generated artifact contract consumed by kro-oriented authoring.

Out of scope:

- Transformation extension runtime model (follow-on ADR).
- CLI packaging and distribution details.
- Final execution-engine selection for transforms/extensions.

## Invariant alignment

This proposal preserves the five design invariants in `docs/traits-spec.md`:

1. **Reconciliation-first:** generated artifacts retain explicit desired/observed modeling and operation profiles for repeated convergence loops.
2. **DAG-preserving:** IR carries dependency metadata for deterministic graph planning and cycle-safe ordering.
3. **Explicit operation contracts:** lifecycle behavior is explicit in IR (defaults are inferred then materialized), avoiding verb-only assumptions.
4. **Declarative-first authoring:** users primarily consume typed declarative artifacts; overrides stay declarative and data-driven.
5. **Cloud-agnostic core:** normalization and IR are provider-neutral, with provider-specific behavior handled through declared overrides.

## Explicit unresolved decisions (reason ADR remains Proposed)

1. Exact IR schema versioning and compatibility policy across generator releases.
2. Ambiguous endpoint resolution rules when HTTP method heuristics are insufficient (e.g., multiple POST endpoints with distinct lifecycle meanings).
3. Validation boundary details between authoring-time, planning-time, and reconcile-time feedback for generated artifacts.
4. Packaging/versioning contract for generated artifact bundles consumed by kro-oriented authoring.

## Consequences

- **Positive**
  - Preserves cloud-agnostic architecture while keeping kro-first UX.
  - Allows deterministic, testable schema normalization.
  - Enables stronger static feedback before reconciliation.
  - Keeps non-standard API support explicit without forcing bespoke controllers by default.
- **Negative / accepted tradeoffs**
  - Additional compiler/tooling complexity.
  - Requires long-term maintenance of transform rules.
  - Method-derived defaults can mask bad source schemas unless ambiguity detection is strict.
- **Follow-up work**
  - Identity/stability rules for expanded resources (ADR-0005).
  - Plan/status projection semantics (ADR-0006).
  - Transform extension model (ADR-0007).

## References

- `docs/traits-spec.md`
- `docs/research/implementation.md`
- `docs/research/usability.md`
- kro docs: <https://kro.run/docs/overview/>
