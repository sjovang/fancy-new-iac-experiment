# Declarative authoring guidelines

This project supports richer expression power (conditionals, loops, transforms) while preserving declarative intent.

## Principles

1. **Desired-state first**
   - Expressions should describe target infrastructure state, not execution steps.

2. **Deterministic evaluation**
   - Authoring expressions should be pure and stable for the same inputs.

3. **Graph-friendly outputs**
   - Conditionals/loops should produce stable resource identities and predictable dependency edges.

## Conditionals

Use conditionals to select desired state, not to script procedural branching.

Good pattern:
- Choose between two desired configurations based on input/environment.

Anti-pattern:
- Execute one branch as a sequence of imperative steps and then continue linearly.

## Loops

Use loops/comprehensions to generate collections of desired resources with stable keys.

Good pattern:
- Generate N resources from input data while preserving deterministic identifiers.

Anti-pattern:
- Build resources in order-dependent loops where behavior depends on iteration side effects.

## Data transformations

Transforms should map input schemas and references into normalized desired-state shapes.

Good pattern:
- Schema-aware mapping from vendor payload fields into provider-agnostic resource spec.

Anti-pattern:
- Runtime mutation pipelines that hide dependency edges or implicit side effects.

## Practical constraints

- Keep expression power bounded and observable.
- Make dependencies explicit when transforms produce cross-resource references.
- Favor typed, schema-aware transformations over opaque script snippets.
