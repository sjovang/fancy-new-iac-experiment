# Research track: Usability

Goal: keep the Kubernetes substrate (decided in [ADR-0001](../decisions/accepted/0001-execution-surface.md)) from leaking into the user experience. We adopt K8s for its reconciliation, watch/optimistic-concurrency, and ecosystem benefits — but a typical user should not have to learn Kubernetes, Crossplane-style layered CRDs, or YAML idioms to be productive.

This document is a living research backlog, not a decision record. Findings here feed into future ADRs (authoring layer, plan-step semantics, error model, etc.).

## Anti-goals (lessons from Crossplane and adjacent tools)

1. **Don't require users to learn the substrate before doing anything useful.** Crossplane's MR/XRD/Composition/Provider/ProviderConfig ramp is the canonical anti-pattern.
2. **Don't surface raw reconciliation churn as the primary failure mode.** Eventual-consistency retry loops are how K8s controllers work; they are *not* an acceptable user-facing UX for ordering errors.
3. **Don't pretend YAML is the authoring language.** YAML may exist at the substrate, but the authoring surface should be higher-level.
4. **Don't auto-generate providers from Terraform providers.** This drags in CRUD/HTTP-verb assumptions and Terraform-provider quirks. Generate from upstream API schemas directly.

## Research questions

### Q1. Authoring surface (feeds ADR-0002)

- What is the user's primary authoring artifact? Candidates: a higher-level language (CUE/KCL), a program host (Pulumi-style), a declarative-with-functions model (Crossplane composition-functions style), or a new DSL.
- How does the authoring layer synthesize Kubernetes CRs without leaking CRD verbosity into the source?
- Can the same authoring layer target both our own resource types and existing K8s CRDs (interop with cert-manager, external-secrets, etc.)?
- What's the AI/codegen story? An LLM-friendly surface is a hard requirement.

### Q2. Dependency model and plan preview (trait #2)

- Crossplane's "no DAG, eventual consistency" model is explicitly rejected. How do we layer a planned DAG over a controller-runtime reconciler without re-inventing Terraform's apply-only model?
- What does a `plan` preview look like in a continuous control plane? (Diff against current desired state? Dry-run reconcile? "What would the next N reconciles do"?)
- How are cross-resource references modeled at the authoring layer vs the CRD layer? Pulumi-style outputs? Crossplane-style `*Ref`/`*Selector`? Something else?
- How are ordering failures surfaced — as one clear error, not a wall of retry events?

### Q3. Error and status model

- Conditions are powerful but cryptic. What does a user-facing error look like when reconciliation can't progress?
- Aggregated status across a multi-resource graph: how do we present it without making users `kubectl describe` every CR?
- Distinguish authoring errors (validation, schema mismatch), planning errors (cycles, missing refs), and runtime errors (API failure, drift, action precondition).

### Q4. Installation and onboarding

- Time-to-first-resource on a fresh laptop. Target: minutes, not hours. (kind/k3d boot + Helm install + first CR applied + first reconciliation completed.)
- Local-only / no-cloud demo path so contributors can iterate without spending money or holding live credentials.
- How much Kubernetes does the user actually need to know on day 1? Day 30? Day 365?

### Q5. Operations and observability

- Default dashboards / metrics surface so users don't need to assemble Prometheus + Grafana from scratch to debug a reconciliation.
- Audit trail: who changed what, when. Inherit from K8s audit log? Augment?
- Multi-tenancy: namespaces alone, or a higher-level workspace concept?

### Q6. Interop with the cloud-native ecosystem

- ArgoCD/Flux for GitOps delivery of our resources — should "just work" as a non-goal-tested invariant.
- Gatekeeper/Kyverno for policy on top of our CRs.
- External-secrets and cert-manager for credentials referenced by our resources.

## Methodology

- Comparative UX studies of Crossplane, Config Connector, ACK, Terraform/OpenTofu, Pulumi, and SST/Winglang.
- Annotated end-to-end task walkthroughs: "deploy a small multi-resource workload across two providers" measured by steps, concepts, and time.
- Small user studies once a v0 authoring surface exists.

## Open assumptions

- We will *not* require users to write CRDs by hand for typical workflows.
- A higher-level authoring layer is the default; raw CR access is an advanced opt-in.
- AI-assisted authoring is a first-class consideration, not a retrofit.

## References

- [ADR-0001](../decisions/accepted/0001-execution-surface.md)
- `docs/traits-spec.md`
- Research artifact: `artifacts/research/i-want-to-start-a-research-session-and-l.md`

---

## Findings — round 1 (2026-05-19)

First research pass focused on Q1 (authoring surface) and Q2 (dependency model + plan preview), since those most directly inform [ADR-0002](../decisions/superseded/0002-authoring-language.md). Findings on Q3–Q6 are deferred to a later pass.

### Candidate authoring layers, evaluated

#### kro (Kube Resource Orchestrator)

- Model: a `ResourceGraphDefinition` (RGD) declares a schema and a set of resource templates wired together by **CEL expressions** (`${...}`). kro infers a dependency graph from those expressions, generates a CRD, and runs a controller. Source: <https://kro.run/docs/overview/>.
- Strengths:
  - Native-feeling K8s authoring (still YAML, but at a higher level).
  - **Dependency graph is inferred from references** — directly addresses our Crossplane critique #3.
  - Schema validation happens before reconciliation; catches expression and reference errors early.
  - Works with any K8s resource — native or CRD, including ACK/ASO/Config Connector resources.
- Weaknesses vs our traits:
  - Still YAML at the surface; CEL adds a second mini-language users must learn.
  - The "API as RGD" pattern is itself a layered abstraction (RGD → generated CRD → instance), which is the same structural complexity Crossplane is criticized for, just with fewer kinds.
  - Limited general-purpose programming (CEL is intentionally bounded).
- **Verdict:** strong precedent for "infer DAG from references, validate up front" — we should borrow this pattern even if we don't adopt kro itself.

#### cdk8s

- Model: programs in TypeScript/Python/Go/Java that build a tree of constructs and **synthesize plain Kubernetes YAML**. Apply with `kubectl` or GitOps tooling. Source: <https://cdk8s.io/docs/latest/>.
- Strengths:
  - Real programming language for conditionals/loops/abstractions.
  - **Synth boundary** keeps the deployed artifact declarative — directly satisfies trait #5.
  - No new DSL to learn for developers already in a supported language.
  - Mature construct ecosystem.
- Weaknesses:
  - Synth is *write-only* — there is no two-way relationship with cluster state, so identity rules across re-synths are the user's problem.
  - Multiple SDKs to maintain if we go this route.
  - YAML still leaks through when users need to debug.
- **Verdict:** the synth-boundary pattern is the strongest single idea for trait #5. The question is whether we adopt it for us, or just for advanced users.

#### CUE

- Model: typed declarative language where **types are values** in a lattice; configuration, schema, validation, and policy live in one language. Source: <https://cuelang.org/docs/concept/the-logic-of-cue/>.
- Strengths:
  - Best-in-class schema/type story — maps cleanly to OpenAPI ingestion (trait #6).
  - Unification means partial configs compose deterministically; no execution-order surprises.
  - Validation is a first-class operation, not a separate step.
- Weaknesses:
  - Unfamiliar mental model (lattice/unification) — non-trivial learning curve, though contained.
  - Less mainstream than TS/Python; AI codegen quality is lower.
  - Tooling/IDE story weaker than mainstream languages.
- **Verdict:** the strongest fit for trait #6 (schema-driven) and the cleanest "declarative-first" story. Learning curve is real but bounded — and crucially, *single-language* (vs Crossplane's layered ramp).

#### KCL

- Model: CNCF sandbox config language, statically typed, with schemas, lambdas, and policies. Designed for K8s-scale configuration with built-in validation and mutation. Source: <https://www.kcl-lang.io/docs/user_docs/getting-started/intro>.
- Strengths:
  - Imports schemas from K8s CRDs and Terraform provider schemas — schema-driven by design.
  - Modern tooling, package management via OCI registries, multi-language SDKs.
  - More familiar imperative-looking syntax than CUE while remaining declarative.
- Weaknesses:
  - Smaller community than CUE/Pulumi/cdk8s.
  - Some configuration patterns are imperative-feeling (a mild risk for trait #5).
- **Verdict:** a credible middle ground between CUE's purity and a program host's familiarity. Worth a deeper look before deciding.

#### Pulumi-style program host

- Model: covered in the original research artifact (§2.4). Programs run, register resources, engine plans diff.
- Strengths in a K8s context: Pulumi's Kubernetes provider exists and could synthesize CRs.
- Weaknesses for *us* specifically: most of Pulumi's value is the deployment engine — which we are replacing with our own DAG planner + reconciler. We'd be using Pulumi only for the language host, which is a lot of dependency for one capability.
- **Verdict:** lower fit than cdk8s for our context. If we go program-host, cdk8s is the closer analogue.

### Pattern observations across the field

1. **Two distinct authoring strategies are validated in production:** higher-level config languages (kro/CUE/KCL) and synth-boundary program hosts (cdk8s/Pulumi). Both can target K8s CRs cleanly. The choice is mostly about *user persona*, not technical feasibility.
2. **CEL is the de-facto reference-and-expression sub-language inside K8s** (admission, kro, Kyverno). Adopting it for inline references reduces the conceptual surface for K8s-fluent users — but layering CEL on top of an authoring language creates a two-language UX.
3. **DAG inference from explicit references** (kro) is universally a better UX than relying on controller retry-driven eventual consistency (Crossplane). This is a borrow-no-matter-what finding.
4. **Schema-driven typing requires either CUE/KCL natively or codegen from OpenAPI** to per-language SDKs in a cdk8s-style approach. The CUE/KCL path is materially simpler engineering.
5. **"Generated CRD per abstraction" (kro/Crossplane XRD)** doubles the conceptual model — users see *both* the high-level kind and the underlying kinds. This is the root of the Crossplane learning-curve complaint, and any path we choose should consider whether it reproduces this pattern.

### Implications for ADR-0002

Mapping back to the four options in [ADR-0002](../decisions/superseded/0002-authoring-language.md):

- **Option A (new DSL)** — increasingly hard to justify when CUE and KCL exist and are designed for this exact problem space.
- **Option B (embedded declarative DSL — CUE or KCL)** — now the strongest candidate. CUE is purer on traits #5 and #6; KCL is more approachable.
- **Option C (program host, cdk8s-style)** — viable but requires per-language SDK codegen from OpenAPI for every provider we add, and offers a *less* declarative source artifact than B.
- **Option D (declarative + transform functions)** — kro is essentially this with CEL as the function language. Worth considering as a *layer on top of* B rather than as an alternative.

A hybrid worth scoping: **CUE (or KCL) as the authoring language, with kro-style "infer DAG from references, validate up front" semantics, and an optional cdk8s-style program-host on top for users who want a general-purpose language.** This is not committed — just the shape the findings suggest.

### Open follow-ups before deciding ADR-0002

1. Hands-on prototype: author the same small multi-resource example (e.g., a workload + bucket + IAM binding) in kro, CUE+KCL, and cdk8s. Measure concepts introduced and lines of code.
2. AI/codegen evaluation: how well do current LLMs author CUE vs KCL vs cdk8s-TS? This matters disproportionately for adoption.
3. Round 2 of this research, covering Q3 (error/status model) and Q4 (onboarding/time-to-first-resource).

### New citations

- kro overview — <https://kro.run/docs/overview/>
- cdk8s docs — <https://cdk8s.io/docs/latest/>
- CUE logic — <https://cuelang.org/docs/concept/the-logic-of-cue/>
- KCL intro — <https://www.kcl-lang.io/docs/user_docs/getting-started/intro>

---

## Findings — round 2 (2026-05-19)

Second pass focused on Q3 (error / status model) and Q4 (installation and onboarding). These both shape how the K8s substrate is allowed — or not — to leak into the user experience.

### Q3. Error and status model

#### How Kubernetes itself models status

Per the K8s API conventions and SIG-Architecture guidance (<https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md>):

- Every object has a `spec` (desired) and `status` (observed). Conventions explicitly call out that *multiple components* may write status on the same object.
- `status.conditions[]` is the canonical place for "current-state facets." Each condition has `type`, `status` (`True`/`False`/`Unknown`), `reason` (short, CamelCase, machine-readable), `message` (human), `lastTransitionTime`, and `observedGeneration`.
- Conditions are intentionally **non-orthogonal and extensible** — Brian Grant: *"non-orthogonal, extensible conditions"* (cited in <https://maelvls.dev/kubernetes-conditions/>). They are *not* a state machine; they are independent assertions about the object.
- A common pattern is one aggregating condition (often `Ready`) plus several specific conditions that explain *why* the aggregate is what it is.
- **Conditions vs Events:** conditions = current state; events = history / transitions. Both exist and serve different debugging needs.

#### Where this breaks for users today

- Condition arrays grow over time. A typical Crossplane MR can have 4–8 conditions whose `Reason` strings are only meaningful if you know the controller's source.
- "Why isn't this Ready?" requires walking the condition array, then walking dependent objects' condition arrays, then walking events, often across namespaces. This is exactly the "noisy and confusing" UX you flagged in ADR-0001's Crossplane critique.
- `observedGeneration` is *the* mechanism for distinguishing "stale status from a previous spec" from "current status," and it is routinely ignored by tooling — leading to the classic "I changed the spec, kubectl says Ready, but Ready was for the *old* spec" trap.

#### Implications for our design

1. **Adopt the K8s condition shape verbatim** — conformance with K8s conventions is non-negotiable for ecosystem interop (ArgoCD, Flux, dashboards, etc., all key off conditions).
2. **Mandate `observedGeneration` discipline in every reconciler we ship.** Stale Ready is a footgun we will not allow.
3. **One aggregating condition + a small, *bounded* set of specific conditions.** Crossplane's "let every controller invent reasons" is a primary source of cognitive load. We should publish a fixed condition vocabulary (e.g., `Synced`, `Ready`, `Planned`, `Drifted`, `ActionRunning`) and a fixed reason vocabulary per condition.
4. **Separate authoring/planning/runtime error classes (already noted in Q3 of the backlog).** Authoring errors should fail at synthesis time, planning errors at plan time, runtime errors via conditions/events. Mixing all three into condition arrays is what makes K8s controllers feel chaotic.
5. **Provide a first-class CLI/UI view that aggregates conditions + events across the resource graph.** `kubectl describe` is not the UX. Think `terraform plan`-style output: one screen, root cause first, traversal optional.
6. **Distinguish "transient retrying" from "stuck."** Reconciliation retry loops are a normal mode of operation; users should not see them as errors. Surface them as a single `Progressing` condition with backoff metadata, not as a flood of failed events.

#### Borrow-regardless findings

- K8s condition shape is mandatory.
- `observedGeneration` discipline is mandatory.
- A bounded, documented condition + reason vocabulary is a hard differentiator from Crossplane.
- A graph-aware status view is a hard differentiator from `kubectl describe`-driven UX.

### Q4. Installation and onboarding

#### Baseline: what the ecosystem demands today

- **kind** (local K8s) — single binary, `brew install kind` then `kind create cluster`; cluster ready in ~60s on a modern laptop (<https://kind.sigs.k8s.io/docs/user/quick-start/>).
- **Crossplane install** — Helm repo add + Helm install into `crossplane-system` namespace, then *separately* install each Provider and ProviderConfig, then *separately* author RBAC for cloud credentials (<https://docs.crossplane.io/latest/get-started/install/>). The minimum useful state is ~6–10 commands across the operator, one provider, credentials, and a first MR.
- **kro install** — Helm install + apply an RGD + apply a CR instance. Closer to ~4 commands.
- **cdk8s** — `npm install` + write a program + `cdk8s synth` + `kubectl apply`. Onboarding cost is mostly programming-language setup.

#### The cliff users actually fall off

Across all of the above, the friction isn't installing the operator — it's the **conceptual ramp** to a working first resource:

- Crossplane: cluster → Crossplane → Provider → ProviderConfig (often with a Secret) → MR (or XRD + Composition + Claim). Five-to-seven concept introductions before a first useful resource.
- kro: cluster → kro → RGD → instance. Three concepts.
- Config Connector: cluster → CC + IAM → CR. Two concepts but a non-trivial IAM setup.

#### Implications for our design

1. **Single distribution unit for the v0 install.** One Helm chart (or one `kubectl apply -f`) installs the operator, the CRDs, *and* a default provider with an in-cluster fake/local backend so the first reconciliation can succeed offline. This is the credible path to a "fresh laptop → first reconciliation in <5 minutes" target.
2. **Local-only "no cloud" demo path is a hard requirement.** A fake provider (mock HTTP API or in-cluster ConfigMap-backed "cloud") so contributors can iterate without spending money or holding credentials. Crossplane's `provider-nop` is precedent; we should ship something more useful (a fake "compute + storage" provider) as a teaching aid.
3. **Hide the substrate's setup steps.** Credentials, RBAC bindings, namespacing, and provider configuration should be expressible at the authoring layer (which synthesizes the CRs underneath), not as separate manual `kubectl apply` steps. This is the single biggest UX cliff in Crossplane.
4. **Day-1 / day-30 / day-365 progression should be explicit.**
   - Day 1: user touches our CLI/authoring language; never sees a CRD.
   - Day 30: user understands conditions and graph status views.
   - Day 365: power user can drop down to raw CRs / write their own provider.
   The substrate is always *available* but never *required* until the user opts in.
5. **Onboarding success metric.** Time-to-first-successful-reconciliation on a fresh laptop, measured. Target: ≤5 min from `curl | sh` to a green status on a multi-resource example using the local fake provider.

#### Borrow-regardless findings

- Single Helm chart (or equivalent) installs everything needed for first success.
- A local fake provider ships with v0.
- Credentials/RBAC are configured through the authoring layer, not separate `kubectl apply` steps.
- "Time-to-first-successful-reconciliation" is a tracked metric, not a vibes-based goal.

### Implications for ADR-0002 (authoring language)

The error/onboarding findings reinforce the round-1 conclusion:

- **Whatever authoring layer we choose must own credential and RBAC expression**, not delegate to raw `kubectl apply`. This favors authoring languages that can synthesize *multiple* CRs from one source unit (CUE, KCL, kro, cdk8s all qualify; a thin DSL might not).
- **The authoring layer must surface conditions back to the user in its own vocabulary.** A program-host (cdk8s-style) can do this via SDK return types; a config language (CUE/KCL) needs a runner/CLI that joins source to live status.
- **The authoring layer must support a "render to dry-run plan" mode** that integrates with the K8s admission/preview machinery for trait #2's plan preview.

These three requirements bias the decision toward Option B (CUE/KCL) or Option C (program-host) over Options A/D, and toward authoring layers that come with a *runner CLI* — not just an evaluator.

### Q3/Q4 open follow-ups

- Survey 3–5 real Crossplane error reports (e.g., from the project's GitHub issues) and rewrite each in our proposed condition vocabulary. Validates whether the bounded vocabulary actually covers reality.
- Prototype the "single Helm chart with operator + CRDs + fake provider" and time the install on a fresh kind cluster.
- Spec the "graph-aware status view" CLI command (what does `<tool> status <root-resource>` print?).

### New citations (round 2)

- K8s API conventions — <https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md>
- Conditions deep-dive (Mael Valais) — <https://maelvls.dev/kubernetes-conditions/>
- kind quick-start — <https://kind.sigs.k8s.io/docs/user/quick-start/>
- Crossplane install — <https://docs.crossplane.io/latest/get-started/install/>
