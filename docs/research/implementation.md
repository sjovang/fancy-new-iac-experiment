# Research track: Implementation

Goal: concrete technical investigation of how to build the tool on the Kubernetes substrate decided in [ADR-0001](../decisions/accepted/0001-execution-surface.md), while satisfying the traits in `docs/traits-spec.md` — especially the parts where we explicitly differ from Crossplane.

This document is a living research backlog. Findings here feed into future ADRs (state store conventions, schema ingestion, provider generation, action model, planner integration).

## Guiding constraints

- Kubernetes-native: CRDs + `controller-runtime` reconcilers.
- DAG planning is a first-class behavior (trait #2), not an emergent property of retries.
- Providers are generated from upstream API schemas (OpenAPI/Swagger/TypeSpec), **not** from Terraform providers.
- Lifecycle is modeled by an explicit operation contract, not by HTTP verb mappings (see `docs/provider-operation-model.md`).

## Research questions

### Q1. controller-runtime usage and reconciler shape

- One reconciler per resource kind (Crossplane-like) vs a generic schema-driven reconciler that handles many kinds.
- How do we represent `Observe → PlanCreate/Update/Delete → InvokeAction → GetOperationStatus` (from `docs/provider-operation-model.md`) as steps inside a single `Reconcile` call?
- Finalizers for delete ordering; owner references for GC; `observedGeneration` discipline for status.
- Server-side apply vs client-side apply for our own desired-state writes.
- Rate limiting and per-API-target backoff strategies.

### Q2. Planner ↔ reconciler integration (the novel part)

- Crossplane has no planner. We do. Where does the planner live: a separate controller that produces a "Plan" CR, an admission webhook, a CLI-only artifact, or all three?
- Resource graph derivation: from explicit references in CR specs, from authoring-layer outputs, or both?
- How is a plan "committed" so reconcilers act on it? Generation bump on the root object? A `Plan`/`PlanRun` CR pattern?
- How do we present plan output to users that is meaningful without `kubectl` archaeology?
- Cycle detection, transitive dependency invalidation, partial-graph re-planning.

### Q3. Provider generation pipeline

- Input schemas: OpenAPI 3.x, Swagger 2.x, TypeSpec, AsyncAPI? Start with OpenAPI 3.x as the primary; TypeSpec as a feeder (Microsoft uses it for ARM/Graph).
- Output: per-API-resource CRD + generic reconciler bindings, or a single "API Resource" CRD with a `type@version` discriminator (AzAPI-style)?
- Action endpoints: a separate `Action` CR shape, or an `actions[]` block on the resource CR? Either way, actions must be addressable in the DAG.
- Identity and naming: how a generated CRD models the upstream resource's primary key, parent scope, and immutable fields.
- Long-running operations: the AWS Cloud Control async handle pattern is a strong reference (`CreateResource` → `RequestToken` → `GetResourceRequestStatus`).

### Q4. State, identity, and idempotency

- The K8s API server stores spec/status; the upstream cloud stores actual state. The "two states" pattern: how do we keep them in sync without writing back to spec.
- Drift handling: auto-correct (Crossplane default), surface-only, or per-resource policy.
- Idempotency keys per operation, retry semantics on partial failures, conflict resolution against concurrent external edits.

### Q5. Authoring → CR synthesis

- Whatever ADR-0002 chooses, the output is Kubernetes CRs. What is the synthesis contract?
- Stable identity rules for looped/conditional resources (Pulumi URN ↔ K8s name/namespace).
- Validation: synthesize-time vs admission-time vs reconcile-time.

### Q6. Packaging and distribution

- Operator + CRDs as a Helm chart and/or OLM bundle.
- Provider packages (generated CRDs + reconciler bindings + schema metadata) as a separate distribution unit.
- Versioning: K8s CRD conversion webhooks for spec evolution.

### Q7. Security

- RBAC scoping for our own CRDs.
- Credential handling: lean on external-secrets / IRSA / workload identity rather than baking secret stores into the operator.
- Multi-tenancy via namespaces vs a higher-level workspace abstraction (links back to usability Q4/Q5).

### Q8. Testing strategy

- envtest for controller unit/integration tests.
- Recorded HTTP fixtures for provider tests (no live cloud in CI by default).
- End-to-end test harness with kind + a fake cloud API (or LocalStack-style fakes) per provider.
- Chaos/conformance tests for reconciliation correctness (drift injection, partial failures, conflicting writes).

## Reference implementations to study (and where to diverge)

| Project | Borrow | Diverge |
|---|---|---|
| Crossplane | Spec/status, conditions, package model | No Terraform-derived providers; add a real DAG planner; flatten the user-facing concept count |
| Config Connector | K8s-native cloud control plane validation | Not Google-only; schema-driven across vendors |
| kubebuilder / Operator SDK | Controller scaffolding patterns | Generic schema-driven reconciler, not one-controller-per-kind |
| AWS Cloud Control API | Async resource-request + status-poll | Cross-cloud, not AWS-only |
| Terraform/OpenTofu | DAG semantics, `plan` UX | Continuous loop, not apply-only |
| AzAPI provider | `type@version` + `body` authoring shape | Not Terraform-bound; cross-cloud |

## Open assumptions

- Implementation language: Go (for `controller-runtime` and kubebuilder interop). To be confirmed in a separate ADR.
- Initial schema source: a handful of OpenAPI specs (one Azure resource type, one MS Graph resource type, one AWS resource via Cloud Control) to exercise the generic provider model on heterogeneous inputs.
- v0 ships without a UI — CLI + CRs only.

## References

- [ADR-0001](../decisions/accepted/0001-execution-surface.md)
- `docs/traits-spec.md`
- `docs/provider-operation-model.md`
- `docs/declarative-authoring-guidelines.md`
- Research artifact: `artifacts/research/i-want-to-start-a-research-session-and-l.md`
- Kubernetes controller-runtime — <https://github.com/kubernetes-sigs/controller-runtime>
- kubebuilder — <https://book.kubebuilder.io/>
- AWS Cloud Control API — <https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/what-is-cloudcontrolapi.html>
