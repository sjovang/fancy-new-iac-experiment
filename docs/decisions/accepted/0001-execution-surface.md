# 0001. Execution surface: Kubernetes-native vs standalone reconciler

- **Status:** Accepted
- **Date:** 2026-05-19
- **Deciders:** tjs
- **Related traits:** #1 (continuous reconciliation), #3 (thin API-native providers), #5 (declarative-first authoring), #6 (cloud-agnostic)
- **Related ADRs:** —

## Context

The experimental IaC tool needs a runtime that performs continuous reconciliation against a dependency graph of resources. Two well-validated execution surfaces exist in the ecosystem:

1. **Kubernetes-native control plane** — define resources as CRDs, run controllers against the Kubernetes API server, leverage etcd as the state store and controller-runtime for reconciliation primitives. Used by Crossplane and GCP Config Connector.
2. **Standalone reconciler process** — a self-contained binary/service that owns its own state store, scheduler, and API surface. Not bound to Kubernetes.

This decision shapes:

- Whether Kubernetes is a hard runtime dependency for *users* of the tool.
- The state store and identity model.
- The default API surface (kubectl/CRDs vs a custom CLI/API).
- How RBAC, secrets, events, and multi-tenancy are handled (inherit from K8s vs build our own).
- The contributor on-ramp (familiarity with controller-runtime vs a custom framework).

Constraints from `docs/traits-spec.md`:

- Must be cloud-agnostic (invariant #5).
- Must support continuous reconciliation as a first-class behavior, not as a cron over an apply-only tool.
- Must keep providers thin and schema-driven.

## Options considered

### Option A — Kubernetes-native (CRDs + controllers)

- Summary: Model resources as Kubernetes CRDs; run reconcilers as controllers in a cluster (or kind/k3s for local). Reuse etcd, RBAC, events, secrets, leader election.
- Pros:
  - Continuous reconciliation, leader election, work queues, retries come from `controller-runtime` for free.
  - Mature ecosystem (Crossplane, Config Connector, Operator SDK) to learn from.
  - Spec/status separation is idiomatic in K8s and matches trait #1.
  - RBAC, audit logging, secrets, namespacing — all reusable.
- Cons:
  - Forces every consumer to run Kubernetes, even when targeting only non-K8s clouds. That's a heavy floor.
  - YAML authoring tends toward verbosity; pushes us toward composition functions or a code-gen layer to satisfy trait #5.
  - Tight coupling to Kubernetes API semantics may leak into our resource model.
  - Schema-driven authoring (trait #6) layers awkwardly over CRD schemas (which are themselves schemas).
- Trait alignment: very strong on #1; neutral on #3/#6; weaker on #5 and on "lightweight, embeddable" feel.

### Option B — Standalone reconciler (own process, own state)

- Summary: A standalone binary with an embedded reconcile loop, its own state store (pluggable: file, sqlite, postgres, git), and its own API (CLI + optional gRPC/HTTP). Providers are plugins.
- Pros:
  - No Kubernetes dependency for users; runs anywhere.
  - Full control over the resource model — can shape it around schema-driven, action-aware lifecycle without CRD constraints.
  - Easier to embed (local dev, CI, edge).
  - Authoring layer can be designed from scratch to satisfy trait #5 (e.g., Pulumi-style program host + declarative synth).
- Cons:
  - We must build (or borrow) controller-runtime-equivalents: work queues, leader election, retries, backoff, finalizers, observability.
  - State store, locking, and multi-writer semantics are now our problem.
  - Less ecosystem leverage; smaller existing audience.
  - Higher initial implementation cost.
- Trait alignment: strong on #3/#5/#6; #1 requires we rebuild reconciliation primitives.

### Option C — Hybrid: standalone core, optional K8s adapter

- Summary: Build the reconciler and resource model as a standalone core. Provide an optional Kubernetes adapter that exposes resources as CRDs and runs the same core as a controller in-cluster.
- Pros:
  - Doesn't force K8s on anyone, but lets K8s shops use familiar tooling.
  - Keeps the core model cloud-agnostic and language-host-agnostic.
  - Aligns with Pulumi's layered architecture (engine separable from surface).
- Cons:
  - Two surfaces to maintain.
  - Risk of the K8s adapter becoming second-class or, conversely, of K8s assumptions creeping into the core.
  - More design discipline required at the core/adapter boundary.
- Trait alignment: strong on all traits if the boundary is disciplined.

## Decision

**Accepted: Option A — Kubernetes-native control plane (CRDs + controllers).**

Resources are modeled as Kubernetes CRDs and reconciled by controllers built on `controller-runtime`. Kubernetes is a hard runtime dependency. The engine reuses the Kubernetes API server (watch cache, optimistic concurrency, `resourceVersion` semantics), RBAC, events, secrets, finalizers, leader election, and the surrounding cloud-native ecosystem (ArgoCD/Flux, Gatekeeper/Kyverno, Prometheus operator, cert-manager, external-secrets).

Rationale:

- Trait #1 (continuous reconciliation) is exactly what `controller-runtime` plus the API server's watch/optimistic-concurrency substrate provides. Re-implementing this substrate would consume the experiment's runway.
- Trait #6 is *cloud*-agnostic, not *runtime*-agnostic. Crossplane and Config Connector demonstrate K8s as a control-plane substrate is compatible with cross-cloud resource management.
- The CRD verbosity / trait #5 concern is real but addressable in a separate ADR (authoring layer can synthesize CRs from a higher-level language; see ADR-0002).
- The SPAR pressure-test on this decision concluded that Option C's "neutral boundary" was K8s-shaped in disguise, and that Option B required rebuilding controller-runtime semantics from scratch with no reference implementation.

### Lessons taken from Crossplane (what we will *not* repeat)

We adopt the K8s-native substrate but explicitly reject several Crossplane design choices that hurt usability and correctness:

1. **Steep learning curve.** Crossplane requires users to understand Crossplane *and* Kubernetes *and* a layered set of CRDs (Providers, ProviderConfigs, MRs, XRDs, Compositions, composition functions) before doing anything useful. Our authoring surface must hide that ramp by default; advanced users can opt down into the substrate.
2. **Auto-generating providers from Terraform providers** (Upjet-style). This inherits Terraform's CRUD/HTTP-verb assumptions, drags in Terraform-provider quirks, produces low-fidelity CRDs, and undermines trait #3 (thin, API-native providers) and trait #4 (explicit operation contract). Our providers will be generated from upstream API schemas (OpenAPI/Swagger/TypeSpec) directly, not from Terraform providers.
3. **Weak inter-resource dependencies.** Crossplane relies on eventual consistency through controller retries; there is no planned DAG and no preview of ordering. Failures surface as noisy reconciliation churn that is hard to read for users not deep in K8s/Crossplane. We will keep trait #2 (DAG planning with explicit ordering and a `plan`-style preview) as a first-class behavior on top of the K8s substrate — the planner and the reconciler are complementary, not alternatives.

Explicitly out of scope for this ADR:

- Implementation language (likely Go for `controller-runtime` interop; to be confirmed in a separate ADR).
- Authoring layer / CR synthesis approach (ADR-0002).
- Schema ingestion model.
- Action contract shape on top of CRDs.
- How the DAG planner integrates with the reconciler loop.

## Consequences

- **Positive**
  - Continuous reconciliation, watch cache, optimistic concurrency, leader election, finalizers, conditions, and observability come from the K8s ecosystem.
  - Spec/status separation is idiomatic and matches trait #1 directly.
  - Massive ecosystem leverage: ArgoCD/Flux for GitOps delivery of *our* resources, Gatekeeper/Kyverno for policy, Prometheus operator for metrics, cert-manager and external-secrets for credential management.
  - Contributor on-ramp is well-trodden: `controller-runtime`, kubebuilder, Operator SDK.
- **Negative / accepted tradeoffs**
  - Kubernetes becomes a hard install dependency. Mitigation: kind/k3s/k3d for local; document a minimum cluster footprint.
  - CRD authoring is verbose. Mitigation: the authoring layer (ADR-0002) is expected to synthesize CRs, not require hand-written YAML for typical workflows.
  - We must actively resist drifting into Crossplane's failure modes (see "Lessons" above). This is partially an organizational discipline problem and is captured in the new research tracks.
- **Follow-up work**
  - **Research track:** [`docs/research/usability.md`](../../research/usability.md) — how to keep the K8s substrate from leaking into the user experience (authoring, errors, ordering, plan preview, default ergonomics).
  - **Research track:** [`docs/research/implementation.md`](../../research/implementation.md) — concrete implementation details (controller-runtime usage, CRD shape, planner ↔ reconciler integration, schema ingestion, provider generation pipeline).
  - Future ADRs: state store conventions on top of K8s, plan-step semantics, authoring language (ADR-0002), schema ingestion, action model, drift policy, identity rules, multi-tenant scope.

## References

- Research artifact: `artifacts/research/i-want-to-start-a-research-session-and-l.md` (§3 gap matrix, §5 open questions).
- `docs/traits-spec.md` — design invariants.
- Crossplane docs — <https://docs.crossplane.io/latest/>
- GCP Config Connector overview — <https://cloud.google.com/config-connector/docs/overview>
- Pulumi architecture — <https://www.pulumi.com/docs/iac/concepts/how-pulumi-works/>
- Kubernetes controller-runtime — <https://github.com/kubernetes-sigs/controller-runtime>
