# 0003. Authoring surface pivot to kro

- **Status:** Accepted
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #1 (continuous reconciliation), #2 (DAG planning), #5 (declarative-first authoring), #6 (cloud-agnostic schema awareness)
- **Related ADRs:** [0001](./0001-execution-surface.md), [0002](../superseded/0002-authoring-language.md)

## Context

ADR-0002 selected CUE as the authoring surface. After deeper review of ecosystem/governance posture and fit with the Kubernetes-native runtime from ADR-0001, we are revisiting that choice.

The project needs an authoring surface that:

- stays declarative-first for broad platform-team usability;
- maps naturally into Kubernetes resources and reconciliation behavior;
- preserves explicit dependency graph semantics;
- can evolve with strong ecosystem signals and low governance risk.

## Options considered

### Option A — Keep CUE (ADR-0002)

- Summary: Retain CUE as the primary authoring language and synthesis boundary.
- Pros:
  - Strong schema/type system and validation.
  - Good support for transforms and composition.
- Cons:
  - Requires adoption of a less familiar language for many users.
  - Adds a language/runtime boundary that is separate from Kubernetes-native composition UX.
- Trait alignment: strong on #5/#6, moderate ergonomics risk for adoption.

### Option B — Pivot to kro-oriented authoring

- Summary: Use kro resource graph concepts as the primary authoring surface for app-level composition and dependency declarations.
- Pros:
  - Strong alignment with Kubernetes-native control-plane direction (ADR-0001).
  - Declarative graph shape is explicit and easier to inspect.
  - Better ecosystem confidence from CNCF linkage and cloud-native community alignment.
- Cons:
  - We still need to define how provider schema ingestion maps into kro-facing authoring UX.
  - Some advanced transformation needs may require additional design layers.
- Trait alignment: strong on #1/#2/#5, with follow-up work needed to keep #6 first-class.

## Decision

**Accepted: Option B — pivot to kro-oriented authoring.**

We will use a kro-style resource graph authoring surface as the primary direction. ADR-0002 is superseded. CUE is no longer the default authoring decision.

Explicitly out of scope:

- Final schema-ingestion mechanics from OpenAPI/Swagger into kro-facing authoring.
- Exact extension model for complex transforms.
- CLI and packaging details.

## Consequences

- **Positive**
  - Tighter conceptual alignment with Kubernetes-native reconciliation and graph composition.
  - Simpler mental model for teams already operating in cloud-native/K8s ecosystems.
  - Decision aligns with governance confidence preferences (CNCF-linked ecosystem).
- **Negative / accepted tradeoffs**
  - Reduced built-in type/unification capability versus a CUE-first model unless we add equivalent validation layers.
  - We must deliberately design transform ergonomics so complex use cases do not regress.
- **Follow-up work or future ADRs**
  - ADR for schema-ingestion and type-safety strategy on top of kro.
  - ADR for transformation/templating extension model.
  - ADR for plan/status projection from graph definitions to runtime conditions.

## References

- [0001. Execution surface](./0001-execution-surface.md)
- [0002. Authoring language / surface](../superseded/0002-authoring-language.md)
- kro overview — <https://kro.run/docs/overview/>
- CNCF landscape/profile references (governance signal)
