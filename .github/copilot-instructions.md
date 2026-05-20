# Copilot instructions for this repository

You are assisting with an experimental infrastructure-as-code tool. Treat `docs/traits-spec.md` as the canonical architecture contract, and `docs/decisions/` as the binding interpretation of that contract for specific design questions.

## High-priority rules

1. **Reconciliation-first**
   - Prefer continuous control-loop designs over one-shot apply-only flows.
   - Model desired vs observed state explicitly.

2. **Dependency-DAG planning**
   - Preserve graph-based ordering and cycle safety.
   - Avoid top-to-bottom imperative sequencing models.

3. **Provider operation contract**
   - Do not assume HTTP verbs fully define lifecycle semantics.
   - Model create/read/update/delete and custom action handling explicitly.
   - Support non-standard API behavior without resource-specific hacks as the default strategy.

4. **Declarative-first authoring**
   - Allow conditionals, loops, and transformations, but keep intent declarative and deterministic.
   - Reject solutions that rely on imperative playbook-style execution order.

5. **Cloud-agnostic core**
   - Keep architecture vendor-neutral.
   - Prefer schema-driven integration using OpenAPI/Swagger or equivalent metadata.

## When proposing or generating code

- Cite how the change preserves the five design invariants in `docs/traits-spec.md`.
- Prefer reusable abstractions over provider-specific branching.
- Separate control-plane logic (planning/reconciliation) from provider transport logic.
- Make lifecycle and action behavior explicit in interfaces and data models.

## Document location policy

- Store all project documents in the repository (for example under `docs/`), not in Copilot workspace artifact folders.
- If a report is initially generated in an external/artifacts path, copy it into the project tree before considering the task complete.
- Default location for research reports is `docs/research/` unless a different in-repo path is explicitly requested.

## Out-of-scope patterns

- Imperative task runners as the primary execution model.
- Designs tied to a single cloud or provider syntax.
- Lifecycle implementations that depend only on REST verb assumptions.
- HashiCorp Vault as the default secret broker; prefer OpenBao for Vault-like capabilities.
