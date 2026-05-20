# 0002. Authoring language / surface

- **Status:** Superseded by 0003
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #5 (declarative-first with real programming constructs), #6 (cloud-agnostic, schema-driven), #2 (DAG planning)
- **Related ADRs:** [0001](../accepted/0001-execution-surface.md)

## Context

`docs/traits-spec.md` requires authoring that supports conditionals, loops, and transforms while keeping intent declarative and graph-resolvable. The execution surface ([ADR-0001](../accepted/0001-execution-surface.md), Accepted) is Kubernetes-native, so the authoring layer's synthesis target is **Kubernetes CRs**. The options below are evaluated with that constraint in mind: each must answer "how does this synthesize to CRs without leaking CRD verbosity into the user's source?"

The authoring surface determines:

- How users express desired state.
- Whether expressions are evaluated by us (DSL) or by an existing runtime (program host).
- How the resource graph is derived (statically from text vs dynamically from program execution + identity rules).
- How tightly OpenAPI/Swagger schemas can drive type-safe authoring (trait #6).
- The contributor and AI codegen experience.
- Tooling effort (parser, type system, LSP, formatter — or none).

Key precedents from the research artifact:

- **HCL / OpenTofu** — purpose-built DSL; great DAG inference; limited expressivity.
- **Pulumi** — general-purpose languages with a synth/engine boundary; very expressive; graph derived from program execution with explicit URN identity.
- **AWS CDK / CDKTF** — program → synth → declarative output (CFN/TF); preserves a declarative artifact.
- **CUE** — typed, declarative config language with unification; strong schema story.
- **KCL / Jsonnet / Starlark** — embedded config DSLs with controlled expressivity.
- **Crossplane Compositions / composition functions** — declarative templates plus pluggable functions for transforms.

## Options considered

### Option A — Purpose-built DSL (HCL-like)

- Summary: Design a new declarative language with first-class resource/dependency syntax and bounded expressions (conditionals, comprehensions, transforms).
- Pros:
  - Maximum control over semantics; can bake in our resource/action/identity model.
  - Static analysis and DAG inference are straightforward.
  - Schema-driven IDE support is natural (we own the type system).
  - Outputs are inherently declarative.
- Cons:
  - Huge tooling effort: parser, type checker, formatter, LSP, error messages, package manager.
  - Users learn yet another language.
  - Risk of recreating HCL's limitations (escape hatches inevitably needed).
- Trait alignment: strong on #2/#5/#6; high implementation cost.

### Option B — Embedded declarative DSL (CUE / KCL / Starlark / Jsonnet)

- Summary: Adopt an existing config language as the authoring layer; bind our resource model to its evaluation output.
- Pros:
  - Skip language-design effort.
  - CUE in particular has a strong type/schema/unification story that maps well to OpenAPI ingestion.
  - Users get a sandboxed, deterministic evaluation model — declarative by construction.
- Cons:
  - Inherit the host language's idioms, errors, and limitations.
  - Schema/type interop with OpenAPI varies (CUE: excellent; Starlark: weak).
  - Less control over IDE/LSP integration depth.
  - Some host languages (Starlark) are imperative under the hood, which conflicts with trait #5 unless we constrain usage.
- Trait alignment: strong on #5/#6 if CUE is chosen; mixed otherwise.

### Option C — Program host with synth boundary (Pulumi/CDK-style)

- Summary: Users write programs in TypeScript/Python/Go/etc. that register resources via an SDK. The program runs once to produce a declarative graph, which the engine plans and reconciles. Identity is explicit (URN/address).
- Pros:
  - Full programming language ergonomics: conditionals, loops, abstractions, testing, package managers — all free.
  - Pulumi/CDK have validated the model at scale.
  - The synth boundary keeps the engine input declarative even though authoring is programmatic.
  - Easy AI/codegen path — LLMs are strongest with mainstream languages.
- Cons:
  - Identity rules are subtle: stable resource names across runs require discipline (Pulumi URN model is non-trivial).
  - Risk of imperative side effects bleeding into authoring (file I/O, network calls during synth) — must be constrained or accepted.
  - Multiple language SDKs to maintain (or pick one and live with it for v0).
  - "Declarative" becomes a property of the synth output, not the source — some reviewers find this less auditable.
- Trait alignment: strong on #5; #2 needs careful identity rules; #6 schema-driven typing requires per-language codegen from OpenAPI.

### Option D — Hybrid: declarative core + pluggable transform functions

- Summary: A minimal declarative artifact (YAML/JSON/CUE) is the canonical authoring surface, but users can plug in transform/composition functions (in any language) that produce or mutate desired state before it enters the engine. Inspired by Crossplane composition functions.
- Pros:
  - Auditable declarative artifact at the boundary.
  - Programmatic power is opt-in and scoped, not pervasive.
  - Functions are independently testable and reusable.
- Cons:
  - Two-layer authoring model can confuse newcomers.
  - Function execution model (sandbox? subprocess? wasm?) is its own design problem.
  - Schema ingestion has to bridge the declarative layer and the function layer.
- Trait alignment: strong on #5 (declarative-first, programmable where needed); good on #2.

## Decision

**Accepted: Option B — embedded declarative DSL, specifically CUE.**

> Historical note: this decision was later superseded by [0003](../accepted/0003-kro-authoring-surface.md) after governance and ecosystem fit were re-evaluated.

We will use **CUE** as the primary authoring surface. Users author desired state and transformations in CUE, and the engine evaluates/synthesizes that into Kubernetes custom resources for reconciliation.

Rationale:

- Best alignment with trait #6 (schema-driven, cloud-agnostic authoring) via CUE's type/unification model.
- Strong alignment with trait #5 while preserving declarative-first semantics; conditionals/loops/transforms are available without defaulting to imperative runtime behavior.
- Lower implementation risk than designing a new DSL (Option A) and lower identity-complexity risk than full program-host execution (Option C).
- Supports the K8s-native substrate from ADR-0001 without forcing CRD-level verbosity into the user's source.

Explicitly out of scope:

- Implementation language of the engine itself (separate concern).
- Module/package distribution mechanics.

## Consequences

- **Positive**
  - Single declarative authoring model with strong validation and composition.
  - Direct path to OpenAPI/Swagger-driven type generation and schema-aware authoring UX.
  - Keeps evaluation deterministic and auditable at the config layer.
- **Negative / accepted tradeoffs**
  - Teams unfamiliar with CUE face an onboarding curve.
  - External API reads must be handled in runner/controller boundaries, then fed into CUE input.
  - We must define strict rules for how CUE evaluation output maps to graph identity and plan steps.
- **Follow-up ADRs (expected)**
  - Schema ingestion and type-generation model.
  - Identity/stability rules for looped/conditional resources in CUE output.
  - Plan-step and status projection semantics from CUE source to reconciler graph.

## References

- Pulumi architecture — <https://www.pulumi.com/docs/iac/concepts/how-pulumi-works/>
- CUE — <https://cuelang.org/>
- KCL — <https://www.kcl-lang.io/>
- AWS CDK — <https://docs.aws.amazon.com/cdk/v2/guide/home.html>
- cdk8s — <https://cdk8s.io/docs/latest/>
- kro — <https://kro.run/docs/overview/>
- Crossplane composition functions — <https://docs.crossplane.io/latest/composition/>
- **Usability research findings (round 1):** [`docs/research/usability.md`](../../research/usability.md#findings--round-1-2026-05-19) — concrete evaluation of kro / cdk8s / CUE / KCL and implications for the options below.
- Research artifact: `artifacts/research/i-want-to-start-a-research-session-and-l.md` (§4 inspiration synthesis, §5.4 open question).
