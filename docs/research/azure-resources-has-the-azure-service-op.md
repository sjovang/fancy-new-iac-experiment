# Feasibility Report: A Kubernetes Operator for Entra ID / Microsoft Graph

*Modeled after `Azure/terraform-provider-azapi` and `microsoft/terraform-provider-msgraph` — a thin, generic layer over the Graph API rather than a fully-typed, per-resource operator.*

**Date:** 2026-05-21
**Scope:** Assess effort, risks, and architecture for building an Azure Service Operator (ASO)-style controller for Entra/Graph resources, using a generic resource model.

---

## Executive Summary

- **Gap is real.** ASO covers ARM resources thoroughly, but its Entra coverage is a single resource (`EntraSecurityGroup`, hand-written, no async/LRO handling, no general framework). No production-grade native Kubernetes operator exists for Microsoft Graph today.
- **The thin-layer approach is the right one.** Microsoft Graph's v1.0 OpenAPI is ~37 MB and beta is ~67 MB, with ~900–1,000 v1.0 entity types and ~10,000+ paths. A typed-per-resource model in the style of ASO's ARM generator is *possible* but would require a huge codegen investment. Microsoft itself shipped `terraform-provider-msgraph` (July 2025) as a generic 4-resource thin layer, validating the pattern.
- **The Graph API breaks naive CRUD in several well-defined ways** that the operator MUST handle as first-class concepts: action endpoints (POST RPCs), OData functions, `$ref` reference collections, polymorphic types via `@odata.type`, singleton resources (no POST/DELETE — only PATCH), the Application/ServicePrincipal split, soft-delete + restore, eventual consistency requiring `ConsistencyLevel: eventual`, and throttling with `Retry-After`.
- **Recommended architecture:** reuse ASO's generic reconciler shell + credential provider; replace the ARM client with a raw Graph HTTP client; expose 4–5 CRD archetypes mirroring `terraform-provider-msgraph` (`GraphResource`, `GraphUpdateResource`, `GraphReferenceCollection`, `GraphAction`, optional typed CRDs for the top ~20 high-value Entra resources).
- **MVP effort estimate (one experienced Go/controller-runtime engineer):**
  - Generic thin-layer CRDs + reconcilers + auth + LRO + eventual-consistency wait: **2–3 months**.
  - First 5 typed CRDs (Group, Application, ServicePrincipal, AppRoleAssignment, FederatedIdentityCredential): **1–2 additional months**.
  - Production hardening (throttling, sovereign clouds, multi-tenant, soft-delete handling, conformance tests): **2–3 additional months**.
  - Total to "ASO-equivalent quality for the Entra core": **~6–9 person-months**.

---

## 1. Why This Gap Exists

### 1.1 ASO covers ARM, not Graph

Azure resources are described by ARM Swagger/OpenAPI specs published in `Azure/azure-rest-api-specs`. ASO's code generator (`v2/tools/generator/`) consumes these and emits hundreds of typed CRDs. Entra/Graph resources are *not* in the ARM control plane — they live on `https://graph.microsoft.com`, described by **OData CSDL** at `https://graph.microsoft.com/v1.0/$metadata` and a derived OpenAPI in `microsoftgraph/msgraph-metadata`. ASO's pipeline cannot consume this directly without significant re-tooling.

ASO has *started* to grow Entra coverage in `v2/internal/reconcilers/entra/`, but it currently contains only `EntraSecurityGroupReconciler` — hand-written, single-resource, no LRO support, no shared framework, with explicit `// TODO: Factor out common code shared with other Entra resources` comments (`Azure/azure-service-operator:v2/internal/reconcilers/entra/entra_securitygroup_reconciler.go`).

### 1.2 Existing Graph-related options today

| Option | Type | Coverage | Verdict |
|---|---|---|---|
| ASO Entra reconciler | K8s operator | 1 resource (Security Group) | Embryonic; not a framework |
| `crossplane-contrib/provider-upjet-azuread` | K8s operator (Crossplane) | 32 managed resources, generated from `terraform-provider-azuread` via Upjet | Works, but Terraform-embedded (heavyweight), Entra-only subset, can't go generic |
| `hashicorp/terraform-provider-azuread` | Terraform provider (typed) | ~50 resources, ~20 data sources, hand-written | Not a K8s operator; Entra subset only |
| `microsoft/terraform-provider-msgraph` (July 2025) | Terraform provider (generic) | All of Graph (user-driven) | Closest design analogue, but not a K8s operator |
| `terraprovider/terraform-provider-microsoft365wp` | Terraform provider | Intune + Entra config | Typed; has interesting "singleton" handling code we can borrow |

**No native, generic, Graph-API-driven Kubernetes operator exists.** This is greenfield.

---

## 2. The Graph API Surface

### 2.1 Machine-readable spec is available and complete

The `microsoftgraph/msgraph-metadata` repo is the source of truth:

| Artifact | File | Size |
|---|---|---|
| v1.0 CSDL (raw, prod) | `schemas/v1.0-Prod.csdl` | 3.15 MB |
| beta CSDL (raw, prod) | `schemas/beta-Prod.csdl` | 8.13 MB |
| v1.0 OpenAPI | `openapi/v1.0/openapi.yaml` | 37.1 MB |
| beta OpenAPI | `openapi/beta/openapi.yaml` | 67.1 MB |
| Cleaned + annotated v1.0 CSDL | `clean_v10_metadata/cleanMetadataWithDescriptionsAndAnnotationsAndErrorsv1.0.xml` | 7.86 MB |
| OData→OpenAPI conversion settings | `conversion-settings/openapi.json` | — |
| Sovereign cloud variants | `schemas/v1.0-{Fairfax,Mooncake,Bleu,GovSG,USNat,USSec}.csdl` | varies |

Also live at `https://graph.microsoft.com/v1.0/$metadata` (CSDL XML). Aliased by `https://aka.ms/graph/metadata`.

**Scale estimate (from file sizes + the 38 PowerShell SDK module groups):** ~900–1,000 v1.0 entity types, ~1,800–2,200 beta entity types, ~10,000+ API paths in v1.0.

This means the OpenAPI/CSDL is rich enough to drive code generation *if* we want — but the surface is too large to hand-write per-resource controllers for everything. The thin-layer approach skips the codegen problem for the long tail and only generates types for the resources where typed UX really matters.

### 2.2 Non-CRUD patterns the operator MUST model

These are the patterns that drove `terraform-provider-msgraph` to ship multiple resource types instead of one. The operator needs equivalents.

**(a) Singleton resources** — no create, no delete, only `GET`/`PATCH`:
- `/organization/{tenantId}`
- `/policies/authorizationPolicy`
- `/policies/authenticationMethodsPolicy`
- `/policies/adminConsentRequestPolicy`
- `/policies/identitySecurityDefaultsEnforcementPolicy`

Docs: <https://learn.microsoft.com/en-us/graph/api/resources/authenticationmethodspolicy>

`terraprovider/terraform-provider-microsoft365wp:workplace/generic/resource.go` shows the pattern:
```go
if skipDelete {
    diags.AddWarning("Skipping deletion of entity from MS Graph, just removing resource from state",
        "Cannot delete entities that are singletons and/or have been created by MS Graph itself.")
}
if r.AccessParams.WriteOptions.UpdateInsteadOfCreate {
    updateExisting = true
}
```

**(b) Action endpoints (RPC over POST)** — not resources, side-effecting, often `202 Accepted`:
- `POST /users/{id}/sendMail`  → 202, no body
- `POST /users/{id}/revokeSignInSessions` → `{value: true}`
- `POST /users/{id}/assignLicense`
- `POST /applications/{id}/addPassword` / `removePassword` / `addKey` / `removeKey` — returns secret material on response, **plaintext only available once**
- `POST /groups/{id}/validateProperties`
- `POST /directoryObjects/getByIds`

Docs: <https://learn.microsoft.com/en-us/graph/api/user-sendmail>, <https://learn.microsoft.com/en-us/graph/api/user-revokesigninsessions>

**(c) OData function endpoints** — computed/derived GETs:
- `GET /users/delta` → returns deltaLink token for change tracking
- `GET /users/{id}/memberOf/microsoft.graph.group/$count`

Docs: <https://learn.microsoft.com/en-us/graph/delta-query-overview>

**(d) `$ref` reference navigation** — relationship edges, not normal child resources:
- `POST /groups/{id}/members/$ref` with `{"@odata.id": "https://graph.microsoft.com/v1.0/directoryObjects/{id}"}`
- `DELETE /groups/{id}/members/{id}/$ref`
- Batch alternative: `PATCH /groups/{id}` with `"members@odata.bind": ["...", "..."]` (max 20)

Docs: <https://learn.microsoft.com/en-us/graph/api/group-post-members>

**(e) Polymorphism via `@odata.type`** — heterogeneous collections require a discriminator:
- `directoryObject` base: `user`, `group`, `device`, `servicePrincipal`, `application`, `orgContact`, `administrativeUnit`
- `/users/{id}/authentication/methods`: `fido2AuthenticationMethod`, `phoneAuthenticationMethod`, `microsoftAuthenticatorAuthenticationMethod`, ...
- Conditional Access policy: nested structured types each with their own `@odata.type`

Docs: <https://learn.microsoft.com/en-us/graph/api/resources/directoryobject>, <https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy>

**(f) Application / ServicePrincipal split** — two separate objects, separate lifecycles:
- `POST /applications` creates the app registration (generates `appId`)
- `POST /servicePrincipals {appId: "..."}` creates the tenant instance
- Soft-delete: `GET /directory/deletedItems/microsoft.graph.application`, `POST /directory/deletedItems/{id}/restore`, `DELETE` for permanent
- Upsert by alternate key: `PATCH /applications(appId='{appId}')`

Docs: <https://learn.microsoft.com/en-us/graph/api/resources/application>

**(g) Eventual consistency** — many directory queries need `ConsistencyLevel: eventual` + `$count=true` to use the secondary, more up-to-date index:
```
GET /users?$filter=accountEnabled ne true&$count=true
ConsistencyLevel: eventual
```
Without these, `ne`, `not`, `endsWith`, `startsWith`, `$search`, certain `$orderby` calls fail or return stale data. Even with them, POST-then-immediate-GET frequently returns 404 for a few seconds.

Docs: <https://learn.microsoft.com/en-us/graph/aad-advanced-queries>

**(h) Throttling** — token-bucket per app+tenant; respect `Retry-After`:

| Limit | Quota |
|---|---|
| App+tenant (S tier) | 3,500 RU / 10s |
| App+tenant write | 3,000 requests / 2.5 min |
| Per app across tenants | 150,000 RU / 20s |
| Per app write | 35,000 requests / 5 min |
| Global | 130,000 requests / 10s |

429 response includes `Retry-After: <seconds>`. Docs: <https://learn.microsoft.com/en-us/graph/throttling>, <https://learn.microsoft.com/en-us/graph/throttling-limits>

**(i) JSON batching** — `POST $batch` allows up to 20 requests in one call; useful for reducing throttling pressure. Docs: <https://learn.microsoft.com/en-us/graph/json-batching>.

---

## 3. Lessons from `terraform-provider-azapi`

`Azure/terraform-provider-azapi` is the "thin layer over ARM" that the user wants to mirror for Graph.

- **Generic resource shape:** `azapi_resource` takes `type` (e.g. `Microsoft.Storage/storageAccounts@2023-01-01`), `parent_id`, `body` (raw JSON), plus optional outputs via JMESPath (`response_export_values`).
- **Schema validation without per-type Go structs:** ships ARM type definitions as embedded static files in `internal/azure/generated/`, loaded lazily by `internal/azure/index.go`:
  ```go
  type ResourceDefinition struct {
      Definition *types.ResourceType
      Location   TypeLocation
      ApiVersion string
      mutex      sync.Mutex
  }
  func (o *ResourceDefinition) GetDefinition() (*types.ResourceType, error) {
      data, _ := StaticFiles.ReadFile("generated/" + o.Location)
      // lazy-load and cache
  }
  ```
  This is a middle ground between fully typed and fully schema-less — feasible for Graph using the CSDL.
- **LRO handling:** azapi uses the Azure SDK poller within a Terraform apply context. That works for a CLI but is unsafe for a Kubernetes operator across pod restarts. **Do not copy this.** Use ASO's annotation-based poller resume token instead (see §4.2).

---

## 4. Lessons from `microsoft/terraform-provider-msgraph`

This is the closest analogue to what the user wants to build. Released by Microsoft in July 2025. It identified **four (sometimes five) resource archetypes** that are sufficient to express almost all Graph operations:

| Terraform resource | Pattern | Operator CRD analogue |
|---|---|---|
| `msgraph_resource` | Full CRUD over an entity | `GraphResource` |
| `msgraph_update_resource` | PATCH a subset of an existing resource; delete is a no-op | `GraphUpdateResource` (overlay) |
| `msgraph_resource_collection` | `$ref` set reconciliation | `GraphReferenceCollection` |
| `msgraph_resource_action` | POST action endpoints | `GraphAction` (one-shot, status-only) |
| Data sources | Read-only | Could be CRD with `Reconciliation: read-only` or skipped |

Key model (`microsoft/terraform-provider-msgraph:internal/services/msgraph_resource.go`):
```go
type MSGraphResourceModel struct {
    Id                    types.String   // Graph object ID
    ResourceUrl           types.String   // computed full URL
    Url                   types.String   // collection endpoint (e.g. /groups)
    ApiVersion            types.String   // "v1.0" or "beta"
    Body                  types.Dynamic  // schema-less JSON
    IgnoreMissingProperty types.Bool
    UpdateMethod          types.String   // "PATCH" or "PUT"
    ResponseExportValues  map[string]string  // JMESPath selectors
    Output                types.Dynamic
}
```

- **Create:** POST collection URL, then `consistency.WaitForUpdate()` polls until the resource is GETtable (handles eventual consistency).
- **Update:** diff against last-known state, send only changed fields via PATCH.
- **`$ref` URLs:** if URL ends in `/$ref`, extract UUID from `@odata.id` and construct resource URL as `{base}/{uuid}`.
- **Collection sync (`msgraph_resource_collection`):**
  ```go
  func syncCollection(ctx, client, collectionUrl, desired []string) error {
      actual := fetchCurrentMembers(ctx, client, collectionUrl)
      toAdd    := set.Difference(desired, actual)
      toRemove := set.Difference(actual, desired)
      applyCollection(ctx, client, collectionUrl, toAdd, toRemove)
  }
  ```
- **Critical gap:** `internal/clients/msgraph_client.go` has explicit `// TODO: Handle long-running operations if needed` in Create/Update/Delete. **An operator must implement this** — silently succeeding on 202 Accepted will leave resources half-provisioned.

---

## 5. Lessons from ASO v2 to inherit directly

These pieces of ASO are framework-level and largely reusable for any cloud-API-backed operator.

### 5.1 Generic reconciler shell

`Azure/azure-service-operator:v2/internal/reconcilers/generic/generic_reconciler.go`:
```go
type GenericReconciler struct {
    Reconciler                genruntime.Reconciler // per-type implementation
    KubeClient                kubeclient.Client
    Recorder                  record.EventRecorder
    Config                    config.Values
    GVK                       schema.GroupVersionKind
    PositiveConditions        *conditions.PositiveConditionBuilder
    RequeueIntervalCalculator interval.Calculator
    PanicHandler              func()
}
```
`Reconcile()` does fetch → deep-copy → ownership stamp → CreateOrUpdate or Delete → `CommitUpdate`. All cloud-specifics live behind the `genruntime.Reconciler` interface:
```go
type Reconciler interface {
    CreateOrUpdate(ctx, log, eventRecorder, MetaObject) (ctrl.Result, error)
    Delete(ctx, log, eventRecorder, MetaObject) (ctrl.Result, error)
    Claim(ctx, log, eventRecorder, MetaObject) error
    UpdateStatus(ctx, log, eventRecorder, MetaObject) error
}
```
A Graph operator implements this interface and gets condition management, finalizers, pause/detach annotations, and rate-limited requeue for free.

### 5.2 Annotation-based LRO state machine

ASO's solution for ARM long-running operations is the right model for Kubernetes (stateless between reconciles):
```go
// azure_generic_arm_reconciler_instance.go:338–345
resumeToken, err := pollerResp.Poller.ResumeToken()
SetPollerResumeToken(r.Obj, pollerResp.ID, resumeToken)
return ctrl.Result{Requeue: true}, nil
```
On the next reconcile:
```go
poller := r.ARMConnection.Client().ResumeDeletePoller(pollerID)
err := poller.Resume(ctx, r.ARMConnection.Client(), pollerResumeToken)
if poller.Poller.Done() { return ctrl.Result{}, nil }
retryAfter := genericarmclient.GetRetryAfter(poller.RawResponse)
return ctrl.Result{Requeue: true, RequeueAfter: retryAfter}, nil
```
For Graph: same pattern, but the poller polls the `Location` header from the original 202 response (Graph uses `Location` rather than ARM's `Azure-AsyncOperation`).

### 5.3 Three-level credential hierarchy

`v2/internal/identity/credential_provider.go` resolves credentials per-resource → per-namespace → cluster-default and supports:

| Mode | Secret key |
|---|---|
| Client secret | `AZURE_CLIENT_SECRET` |
| Client certificate | `AZURE_CLIENT_CERTIFICATE` |
| User-assigned managed identity | `AZURE_USER_ASSIGNED_IDENTITY_CLIENT_ID` |
| Workload identity (default) | projected token at `/var/run/secrets/tokens/azure-identity` |

Graph reuses this `azcore.TokenCredential` with `scope = https://graph.microsoft.com/.default`. AKS Workload Identity Federation is the recommended in-cluster path: ServiceAccount annotated with `azure.workload.identity/client-id`, federated identity credential trusts the cluster OIDC issuer + SA subject. See <https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview>.

### 5.4 Adopt-or-create semantics (already in ASO Entra)

`v2/internal/reconcilers/entra/entra_securitygroup_reconciler.go:75–110`:
```go
if id, ok := getEntraID(obj); ok { return r.update(ctx, id, group, log) }
if r.canAdopt(group) {
    id, err := r.tryAdopt(ctx, group, log)
    if id != "" { setEntraID(obj, id); return r.update(...) }
}
if r.canCreate(group) { return r.create(ctx, group, log) }
```
Essential for Entra: pre-existing objects are the norm. Store the Graph object ID as a Kubernetes annotation, like ASO does for ARM resource IDs.

### 5.5 CEL-based secret/configmap export

`v2/internal/reconcilers/arm/kubernetes_resource_exporter.go` evaluates CEL expressions against the cloud response body to populate Kubernetes Secrets and ConfigMaps. This is exactly what's needed for Graph too (especially for `addPassword` which returns plaintext only once). The simpler `OperatorSpec.Secrets/ConfigMaps` model works for static field extraction.

---

## 6. Recommended Architecture

```
┌──────────────────────────────────────────────────┐
│            GenericReconciler  (ASO shell)         │
│  finalizers, ownership, reconcile-policy,         │
│  CommitUpdate, conditions, requeue calculator     │
└──────────────────────┬────────────────────────────┘
                       │  genruntime.Reconciler
            ┌──────────┴─────────────────────────┐
            │      GraphGenericReconciler         │
            │  routes by CRD kind to handler      │
            │  owns GraphClient + CredentialProv. │
            └──┬────────┬─────────┬─────────┬─────┘
               │        │         │         │
               ▼        ▼         ▼         ▼
        FullCRUD   Overlay   RefCollection  Action
        (Graph-    (Update-  (membership/   (one-shot
         Resource) Resource)  ownership)     POST)
               │        │         │         │
               └────────┴────┬────┴─────────┘
                             ▼
                    ┌────────────────────┐
                    │   GraphHTTPClient  │
                    │  azcore pipeline   │
                    │  bearer token      │
                    │  @odata.nextLink   │
                    │  202+Location LRO  │
                    │  Retry-After 429   │
                    │  ConsistencyLevel  │
                    │  $batch support    │
                    └────────────────────┘
```

### 6.1 CRD archetypes (MVP)

| CRD | Purpose | Notes |
|---|---|---|
| `GraphResource` | Generic CRUD on any Graph entity | `apiVersion: v1.0|beta`, `url`, `body`, `updateMethod`, `responseExportValues` |
| `GraphUpdateResource` | Manage a subset of fields on a singleton or pre-existing object; no-op on delete | For `authenticationMethodsPolicy`, organization settings, etc. |
| `GraphReferenceCollection` | Reconcile a `$ref` set (group members, app owners, role assignments) | Diff-based add/remove, optional `@odata.bind` PATCH batching |
| `GraphAction` | One-shot POST action; result captured in `.status` | For `addPassword`, `assignLicense`, etc. |

### 6.2 Optional typed CRDs for MVP

Generate or hand-write typed CRDs for the top ~5 high-value Entra resources where the UX win is large:

1. `EntraGroup` — exercises adopt-or-create + member set reconciliation
2. `EntraApplication` — prerequisite for workload identity
3. `EntraServicePrincipal` — created from Application
4. `EntraAppRoleAssignment` — simple child
5. `EntraFederatedIdentityCredential` — high demand for K8s workload identity

These five cover the 80% workload-identity use case, are all synchronous (no LRO needed), and exercise every reconciler archetype except `GraphAction`.

### 6.3 Cross-cutting machinery the operator must implement

| Capability | Implementation note |
|---|---|
| LRO (202 + Location) | Annotation-based resume token; polling with `Retry-After` |
| Eventual consistency | Post-write requeue-with-backoff until GET succeeds; auto-inject `ConsistencyLevel: eventual` for known advanced queries |
| Throttling | `Retry-After`-aware rate limiter wrapping `controller-runtime` workqueue; per-tenant token bucket to stay under app+tenant quotas |
| `$ref` set reconciliation | Diff-then-apply; batch up to 20 with `@odata.bind` |
| Polymorphism | Pass `@odata.type` through `body`; for typed CRDs, generate discriminated unions |
| Singletons | Detect via known list or via spec annotation (`graph.operator/singleton: true`); switch CREATE→PATCH; no-op DELETE with warning |
| Adopt-or-create | Per-CRD annotation `graph.operator/adopt: true`; store object ID after adoption |
| Soft-delete | On 404, check `/directory/deletedItems` before declaring resource fully gone; optional restore policy |
| App/SP split | Either separate CRDs (recommended) or composite CRD with sub-reconciler |
| Multi-tenant | Reuse ASO's `additionalTenants` credential model |
| Sovereign clouds | Pick base URL from credential's `cloudName`; CSDL also varies per cloud |
| OData `$batch` | Optional; pays off under heavy load |

### 6.4 What to validate `body` against

Three options, in order of effort:

1. **Schema-less** (`map[string]any`) — fastest to ship, weakest UX. Mirrors what `msgraph_resource` does.
2. **CSDL-driven dynamic validation** — load `clean_v10_metadata/cleanMetadataWith…v1.0.xml` (~7.86 MB) at startup, validate `body` against the entity type derived from `url`. Mirrors `terraform-provider-azapi`'s `internal/azure/generated/` approach.
3. **Generated typed structs per entity** — like ASO, but the source spec is CSDL/OpenAPI instead of ARM Swagger. Highest UX, highest investment.

**Recommendation:** ship (1) for the generic CRDs in MVP. Add (2) post-MVP. Use (3) only for the small typed-CRD subset.

---

## 7. Effort Breakdown

### 7.1 What you get from ASO for free

| Component | Effort saved |
|---|---|
| `GenericReconciler` skeleton | ~1 month |
| Finalizer / ownership / reconcile-policy handling | ~2 weeks |
| Condition management + `PositiveConditionBuilder` | ~1 week |
| Credential provider (3-level, all auth modes) | ~3 weeks |
| Rate-limited requeue calculator | ~1 week |
| CEL secret/configmap exporter (if vendored) | ~2 weeks |
| Adopt-or-create pattern (Entra reference impl) | ~1 week |

### 7.2 What you must build

| Item | Effort |
|---|---|
| Graph HTTP client (pipeline, paging, error mapping) | 2 weeks |
| Graph LRO (202 + Location, annotation-based resume) | 2 weeks |
| Eventual-consistency wait-after-write | 1 week |
| Throttling + Retry-After + per-tenant bucket | 1–2 weeks |
| 4 generic CRDs + reconcilers | 4 weeks |
| `$ref` collection diff + batch | 1–2 weeks |
| Singleton + soft-delete handling | 1 week |
| Test framework (envtest + recorded Graph fixtures) | 3 weeks |
| Docs + permission matrix per CRD | 2 weeks |
| Helm chart / install bundle | 1 week |
| First 5 typed CRDs | 4–6 weeks |
| CSDL-driven body validation (post-MVP) | 4 weeks |
| Sovereign cloud support | 1 week |
| `$batch` optimization (post-MVP) | 2 weeks |

**Totals:**
- MVP (generic only): ~3 person-months
- MVP + first 5 typed CRDs: ~5 person-months
- Production-ready (testing, hardening, sovereign clouds, $batch, docs, releases): ~6–9 person-months

### 7.3 Permission burden

The operator's app registration needs the union of application permissions for all enabled CRD types, e.g.:

| Permission | Needed for |
|---|---|
| `User.ReadWrite.All` | EntraUser-style CRDs |
| `Group.ReadWrite.All` | Groups |
| `GroupMember.ReadWrite.All` | Membership collection CRD |
| `Application.ReadWrite.All` | Applications, ServicePrincipals, FederatedIdentityCredentials |
| `AppRoleAssignment.ReadWrite.All` | App role assignment CRD |
| `Policy.ReadWrite.ConditionalAccess` | CA policies |
| `RoleManagement.ReadWrite.Directory` | Directory role assignments |
| `PrivilegedAccess.ReadWrite.AzureAD` | PIM activations |

Admin consent is required (Global Administrator or Privileged Role Administrator). Per-CRD documentation of minimum required permissions is a non-trivial doc effort.

---

## 8. Risks and Open Questions

| Risk | Mitigation |
|---|---|
| Graph LRO patterns are under-documented; few examples in SDKs | Start with synchronous resources; add LRO when the first async-only resource lands; verify against live API |
| Eventual consistency may cause flapping conditions | Differentiate "Reconciling" from "Failed"; use observedGeneration; backoff requeue |
| Throttling can cause cascading failures across many CRDs | Per-tenant bucket sized below 80% of S-tier quotas; surface throttle status in conditions |
| `terraform-provider-msgraph` source code may not be public — it's a partner provider on the Terraform Registry; the GitHub repo `microsoft/terraform-provider-msgraph` URL was not verified accessible at research time | Plan to study via Registry artifacts and reverse-engineer from the provided docs; or rely on `terraprovider/terraform-provider-microsoft365wp` as a substitute reference (open source) |
| Sovereign clouds have different CSDL surface | Operator must accept cloud env in credential; load matching CSDL if doing validation |
| App permission consent is a human bottleneck | Document minimal-permission profiles for common CRD subsets |
| Soft-delete behavior asymmetric across object types | Per-CRD flag; explicit deletion policy in CRD spec |
| Existing operator-like work in Crossplane via Upjet may make this redundant for some users | Position as native, lightweight alternative to Terraform-embedded Crossplane; emphasize generic surface over typed |

---

## 9. Conclusion

Building a thin-layer, Graph-API-backed Kubernetes operator is feasible and there is no comparable native operator today. The right model is *not* to fork ASO and try to extend its ARM codegen — it is to **reuse ASO's runtime shell** (generic reconciler, credentials, conditions, finalizers, CEL exporter, annotation-based LRO state) and **port the four-archetype design from `terraform-provider-msgraph`** to four generic CRDs, then layer a small number of typed CRDs on top for the highest-value Entra resources.

The hardest non-obvious work is in:

1. LRO handling for Graph (no existing Go example handles this end-to-end), and
2. Eventual-consistency / throttling discipline at the controller queue layer.

A realistic timeline for a single experienced engineer to reach production quality on the Entra core is 6–9 months. A working MVP (generic CRDs only) is achievable in ~3 months.

---

## Citations

### Repositories
- `Azure/azure-service-operator` — <https://github.com/Azure/azure-service-operator>
- `Azure/terraform-provider-azapi` — <https://github.com/Azure/terraform-provider-azapi>
- `microsoft/terraform-provider-msgraph` — (Terraform Registry; GitHub URL unverified)
- `hashicorp/terraform-provider-azuread` — <https://github.com/hashicorp/terraform-provider-azuread>
- `crossplane-contrib/provider-upjet-azuread` — <https://github.com/crossplane-contrib/provider-upjet-azuread>
- `terraprovider/terraform-provider-microsoft365wp` — <https://github.com/terraprovider/terraform-provider-microsoft365wp>
- `microsoftgraph/msgraph-metadata` — <https://github.com/microsoftgraph/msgraph-metadata>
- `microsoftgraph/msgraph-sdk-powershell` — <https://github.com/microsoftgraph/msgraph-sdk-powershell>

### Specific source citations
- ASO codegen pipeline stages — `Azure/azure-service-operator:v2/tools/generator/internal/codegen/code_generator.go:80–200`
- `genruntime.Reconciler` interface — `Azure/azure-service-operator:v2/pkg/genruntime/reconciler.go`
- `GenericReconciler.Reconcile()` — `Azure/azure-service-operator:v2/internal/reconcilers/generic/generic_reconciler.go:73–165`
- Reconcile policy (object+namespace) — `…/generic/generic_reconciler.go:270–330`
- `DetermineCreateOrUpdateAction` (LRO state machine) — `…/arm/azure_generic_arm_reconciler_instance.go:144–157`
- LRO resume-token storage — `…/arm/azure_generic_arm_reconciler_instance.go:255–346`
- LRO monitor + Retry-After — `…/arm/azure_generic_arm_reconciler_instance.go:181–214`
- `PollerResponse[T].Resume()` — `…/internal/genericarmclient/poller.go`
- Credential provider (3-level) — `…/v2/internal/identity/credential_provider.go`
- Schema-less ARM client — `…/v2/internal/genericarmclient/generic_client.go`
- Entra adopt-or-create — `…/internal/reconcilers/entra/entra_securitygroup_reconciler.go:75–110`
- Entra ConfigMap export — `…/internal/reconcilers/entra/entra_securitygroup_reconciler.go:330–380`
- CEL secret exporter — `…/internal/reconcilers/arm/kubernetes_resource_exporter.go`
- azapi schema lazy-load — `Azure/terraform-provider-azapi:internal/azure/index.go`
- msgraph generic resource model + CRUD — `microsoft/terraform-provider-msgraph:internal/services/msgraph_resource.go:58–385`
- msgraph LRO TODO — `microsoft/terraform-provider-msgraph:internal/clients/msgraph_client.go` (Create/Update/Delete)
- msgraph $ref collection sync — `…/internal/services/msgraph_resource_collection.go`
- msgraph overlay/no-op delete — `…/internal/services/msgraph_update_resource.go`
- msgraph action endpoints — `…/internal/services/msgraph_resource_action.go`
- Singleton handling — `terraprovider/terraform-provider-microsoft365wp:workplace/generic/resource.go`
- Crossplane provider resources — `crossplane-contrib/provider-upjet-azuread:config/generated.lst`

### Microsoft Learn / Graph docs
- Graph metadata + OpenAPI: <https://github.com/microsoftgraph/msgraph-metadata>, live: `https://graph.microsoft.com/v1.0/$metadata` (aka.ms/graph/metadata)
- Throttling: <https://learn.microsoft.com/en-us/graph/throttling>, <https://learn.microsoft.com/en-us/graph/throttling-limits>
- Advanced queries / `ConsistencyLevel`: <https://learn.microsoft.com/en-us/graph/aad-advanced-queries>
- Delta query: <https://learn.microsoft.com/en-us/graph/delta-query-overview>
- Permissions overview: <https://learn.microsoft.com/en-us/graph/permissions-overview>
- AKS Workload Identity: <https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview>
- `sendMail` action: <https://learn.microsoft.com/en-us/graph/api/user-sendmail>
- `revokeSignInSessions` action: <https://learn.microsoft.com/en-us/graph/api/user-revokesigninsessions>
- Group members `$ref`: <https://learn.microsoft.com/en-us/graph/api/group-post-members>
- Conditional Access policy: <https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy>
- Authentication Methods Policy (singleton): <https://learn.microsoft.com/en-us/graph/api/resources/authenticationmethodspolicy>
- Application resource (upsert by appId, soft-delete): <https://learn.microsoft.com/en-us/graph/api/resources/application>
- directoryObject (polymorphism): <https://learn.microsoft.com/en-us/graph/api/resources/directoryobject>
- JSON batching: <https://learn.microsoft.com/en-us/graph/json-batching>

### Uncertainty / gaps
- `microsoft/terraform-provider-msgraph` source code was reasoned about via secondary sources (the `deploymenttheory/terraform-provider-microsoft365` comparison doc and Microsoft's release notes). The GitHub repo URL was not directly verified during research; the four-archetype description is consistent across sources but should be confirmed against the actual source before implementation.
- Exact Graph entity counts (~900–1,000 v1.0) were estimated from CSDL file size and PowerShell SDK module structure; precise counts require parsing `schemas/v1.0-Prod.csdl`.
- The OData `$batch` and the multi-cloud (sovereign) CSDL variants were not investigated in implementation depth.

---

## 10. Surfacing Helpful Feedback: Which Archetype for a Given Endpoint?

A key UX failure in `terraform-provider-msgraph` is that users get no guidance on which resource type to use (`msgraph_resource` vs `msgraph_update_resource`, etc.) until `terraform apply` fails with a cryptic OData error. The same problem will exist for any multi-archetype operator design. This section describes how to solve it.

### 10.1 The classification signals are already in the CSDL

The `microsoftgraph/msgraph-metadata` CSDL encodes everything needed to classify any endpoint:

| CSDL signal | Correct archetype |
|---|---|
| `<EntitySet>` in `EntityContainer` + `InsertRestrictions/Insertable=true` | `GraphResource` (full CRUD) |
| `<Singleton>` in `EntityContainer` | `GraphUpdateResource` (PATCH-only, no-op delete) |
| `<Action>` or `<Function>` bound to an entity | `GraphAction` |
| Navigation property + `$ref` URL pattern | `GraphReferenceCollection` |
| `<EntitySet>` + `InsertRestrictions/Insertable=false` | `GraphUpdateResource` |

OData capability annotations make insertability explicit:
```xml
<Annotation Term="Org.OData.Capabilities.V1.InsertRestrictions">
  <Record><PropertyValue Property="Insertable" Bool="false"/></Record>
</Annotation>
```

### 10.2 Option A: Validating admission webhook (best UX — fails fast)

A Kubernetes validating admission webhook queries the CSDL classification for `spec.url` at admission time and rejects the resource *before* any reconcile loop runs — with a human-readable message and a remediation hint:

```
Error: GraphResource is not valid for "/policies/authenticationMethodsPolicy".
This endpoint is a singleton — it has no CREATE or DELETE operation.
Use GraphUpdateResource instead.

See: https://learn.microsoft.com/en-us/graph/api/resources/authenticationmethodspolicy
```

The webhook can classify `$ref` URL patterns and bound actions purely from path structure, without full CSDL parsing — covering the most common misuses at low cost.

### 10.3 Option B: Single CRD with auto-classification (simplest user-facing model)

Instead of four CRD kinds, expose **one** `GraphResource` CRD with:

```yaml
spec:
  url: /policies/authenticationMethodsPolicy
  mode: auto   # auto | crud | update | collection | action
  body: ...
```

`mode: auto` (default) causes the operator to classify the endpoint from CSDL at admission/reconcile time, stores the result in `status.detectedMode`, and surfaces a condition if the user's explicit mode contradicts the detection:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: ModeMismatch
    message: >
      spec.mode is "crud" but "/policies/authenticationMethodsPolicy" is a
      singleton (no INSERT). Set spec.mode: "update" or omit it for auto-detection.
```

This is the same reason `terraform-provider-azapi` is tolerable with a single `azapi_resource` — one resource type eliminates the "which archetype?" question for most users.

### 10.4 Option C: Structured status conditions for runtime errors

For errors that slip through (e.g., a 405 on POST to a singleton), map known Graph error codes and HTTP status codes to actionable condition messages rather than surfacing raw OData errors:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: WrongArchetype
    message: >
      POST to /policies/authenticationMethodsPolicy returned 405 Method Not Allowed.
      This endpoint does not support resource creation.
      Hint: use GraphUpdateResource, which manages fields on an existing singleton.
```

A lookup table of `(url pattern → archetype)` for the ~20 most common Entra singleton/action endpoints handles the majority of cases without CSDL parsing at runtime.

### 10.5 Option D: kubectl plugin / local dry-run

A `kubectl graph explain <url>` command that classifies an endpoint and prints the matching archetype, required permissions, and an example YAML manifest. Runs locally against the embedded CSDL — no cluster connection required. Surfaces the same information as the webhook before the user writes any YAML.

### 10.6 Recommendation

The highest-leverage combination is **Option A + Option C**:

- Admission webhook for fail-fast feedback with a clear hint before any cloud call is made
- Structured condition reasons with remediation hints for errors that still reach the reconciler

If a single-CRD model (Option B) is adopted, the `mode: auto` field with CSDL-backed classification subsumes Option A and is the cleanest long-term UX — it eliminates the archetype-selection problem entirely for the majority of users. The four-kind model trades UX simplicity for schema explicitness and is only preferable if strong typing of each archetype is a design goal.
