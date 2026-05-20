# Experimental IaC Tool Traits (Canonical Spec)

This document is the source of truth for architecture and implementation decisions in this repository.

## Mission

Build an experimental infrastructure-as-code system that combines:

- **continuous reconciliation** of desired vs observed state (Crossplane-like control-loop behavior),
- a **dependency graph** for planning and ordering (Terraform/OpenTofu-like DAG behavior),
- **thin, API-native resource providers** that do not hardcode cloud-specific orchestration,
- richer authoring power (conditionals, loops, transforms) while remaining **declarative-first**,
- and **cloud-agnostic schema awareness** by consuming OpenAPI/Swagger and equivalent schemas.

## Required capabilities

1. **Continuous reconciliation**
   - Reconcile resources repeatedly, not only during one-shot apply runs.
   - Detect and correct drift.
   - Separate desired state from observed state.

2. **Dependency graph semantics**
   - Build a DAG from references and explicit dependencies.
   - Respect stable ordering and cycle detection.
   - Re-plan only affected subgraphs when inputs change.

3. **Provider model as thin API layers**
   - Providers should map resource intent to API behavior with minimal vendor-specific orchestration.
   - Provider implementations must be cloud-neutral in architecture, even when endpoints differ.

4. **Operation control beyond HTTP verb assumptions**
   - Resource lifecycle handling cannot assume API semantics map cleanly to HTTP methods.
   - Create/read/update/delete and custom actions must be modeled explicitly.
   - Support endpoints with non-standard behavior without fragile per-resource hacks.

5. **Declarative authoring with programming constructs**
   - Support conditionals, loops, and data transformations.
   - Keep execution intent declarative (desired state), not imperative step execution.
   - Prefer deterministic expressions over procedural scripts.

6. **Cloud-agnostic schema consumption**
   - Ingest OpenAPI/Swagger and similar vendor schema metadata.
   - Use schemas for validation, type generation, and authoring ergonomics.
   - Avoid cloud-locked assumptions in the core model.

## Explicit non-goals

- A top-to-bottom imperative playbook execution model (Ansible-like task orchestration).
- Tight coupling to one cloud vendor or one provider ecosystem.
- Assuming provider lifecycle behavior is fully represented by HTTP method names.
- Requiring hand-crafted per-resource babysitting as the default approach.

## Core terminology

- **Desired state**: user-declared target configuration.
- **Observed state**: current external system state discovered by providers.
- **Reconciliation**: loop that converges observed state toward desired state.
- **Dependency DAG**: directed acyclic graph describing ordering constraints.
- **Provider operation contract**: explicit lifecycle/action handlers independent of transport verbs.
- **Action endpoint**: API behavior that changes state but does not map to strict CRUD semantics.

## Decisions

Architectural decisions that interpret or operationalize these traits are recorded as ADRs under [`docs/decisions/`](./decisions/README.md).

## Design invariants

Every proposal and implementation should satisfy all invariants:

1. Maintains reconciliation-first behavior.
2. Preserves DAG-based dependency ordering.
3. Uses explicit operation contracts instead of verb assumptions.
4. Keeps authoring declarative-first, even with richer expressions.
5. Remains cloud-agnostic in core architecture.
