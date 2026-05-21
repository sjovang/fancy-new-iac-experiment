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

### §11 additions
- Crossplane managed-resource management policies (`spec.managementPolicies` array, values `Observe`/`Create`/`Update`/`Delete`/`LateInitialize`): <https://docs.crossplane.io/latest/managed-resources/managed-resources/> (see "Management policy")
- Crossplane design doc for observe-only resources (origin of the `managementPolicies` model): <https://github.com/crossplane/crossplane/blob/main/design/design-doc-observe-only-resources.md>
- Azure Service Operator annotations including `serviceoperator.azure.com/reconcile-policy` (values `manage`, `detach-on-delete`, `skip`): <https://azure.github.io/azure-service-operator/guide/annotations/>
- Terraform `lifecycle` meta-argument (`prevent_destroy`, `ignore_changes`, `create_before_destroy`, `replace_triggered_by`): <https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle>
- kro overview and instance/RGD model (CEL expressions on resource templates): <https://kro.run/docs/overview> and <https://kro.run/docs/concepts/instances>
- OData v4 `Org.OData.Capabilities.V1` vocabulary (`InsertRestrictions`, `UpdateRestrictions`, `DeleteRestrictions`, navigation `ContainsTarget`): <https://github.com/oasis-tcs/odata-vocabularies/blob/main/vocabularies/Org.OData.Capabilities.V1.md>
- Microsoft Graph long-running actions / 202 + `Location` polling: <https://learn.microsoft.com/en-us/graph/long-running-actions-overview>
- Microsoft Graph `addPassword` (one-shot secret return; motivates `responseCapture`): <https://learn.microsoft.com/en-us/graph/api/application-addpassword>
- Authentication Methods Policy singleton (used in §11.6.2): <https://learn.microsoft.com/en-us/graph/api/resources/authenticationmethodspolicy>
- PIM for Groups `assignmentScheduleRequests` endpoint (used in §11.6.3): <https://learn.microsoft.com/en-us/graph/api/privilegedaccessgroup-post-assignmentschedulerequests>
- Group members `$ref` collection (used in §11.6.4): <https://learn.microsoft.com/en-us/graph/api/group-post-members>
- Local: canonical IR + transform/override model (accepted): `docs/decisions/accepted/0004-schema-ingestion.md`
- Local: provider operation contract (`Observe`/`Create`/`Update`/`Delete`/`InvokeAction`/`GetOperationStatus`): `docs/provider-operation-model.md`
- Local: sibling research on non-HTTP-verb CRUD semantics (PIM `action`/`type`): `docs/research/some-api-s-does-not-use-http-methods-for.md`

### §12 additions
- Microsoft Graph permissions reference (per-permission Delegated vs Application support): <https://learn.microsoft.com/en-us/graph/permissions-reference>
- Microsoft Graph permissions overview (concepts, scopes, consent): <https://learn.microsoft.com/en-us/graph/permissions-overview>
- Microsoft Graph auth concepts (delegated vs app-only access): <https://learn.microsoft.com/en-us/graph/auth/auth-concepts>
- Microsoft Graph error responses (including `Authorization_RequestDenied`): <https://learn.microsoft.com/en-us/graph/errors>
- `microsoftgraph/microsoft-graph-docs-contrib` (per-endpoint Permissions tables; source for the curated delegated-only table): <https://github.com/microsoftgraph/microsoft-graph-docs-contrib>
- Canonical `/me` endpoint family (delegated-only by construction): <https://learn.microsoft.com/en-us/graph/api/resources/user>
- `user: sendMail` (representative Application-permission alternative cited in §12.5 rejection message): <https://learn.microsoft.com/en-us/graph/api/user-sendmail>
- On-Behalf-Of flow (cited only to mark the broker option as out-of-scope per §12.8): <https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow>
- AKS Workload Identity (already cited in §5.3; reused as basis for the Application-only context in §12.4): <https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview>

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

---

## 11. Flexible per-resource operation model

This section is an *extension* of §6.1 and §10.3 rather than a replacement. The executive recommendation in §9 stands; what follows refines the internal shape of the four archetypes into a single, more flexible primitive: a per-resource-type **operation profile**, paired with a per-instance **`managementPolicies`** allow-list. The intent is to give end users (and the codegen) a declarative way to express CRUD + action semantics that fits the Graph quirks already catalogued in §2.2, without leaking transport details into authoring.

### 11.1 Why this is needed

§6.1 introduced four CRD kinds (`GraphResource`, `GraphUpdateResource`, `GraphReferenceCollection`, `GraphAction`). §10.3 Option B observed that those four kinds are largely an artefact of the underlying API quirks and can collapse into a single CRD with a `mode` field. Both formulations are different surface presentations of the same underlying object: a description of *how each lifecycle slot is realised as a transport call*.

The conversation around composing Graph resources with kro [E12] surfaced a second, orthogonal axis: even when a resource type *can* do full CRUD, individual instances often need to be pinned to a subset (observe-only, no-delete, late-init-only). Crossplane calls this `managementPolicies`; ASO exposes it as the `serviceoperator.azure.com/reconcile-policy` annotation; Terraform expresses fragments of it via `lifecycle { prevent_destroy, ignore_changes }`. None of these is wired into the four-archetype design as it stands.

Splitting these two axes — *what the resource type supports* (the operation profile) and *what a specific instance is allowed to do* (the management policy) — produces a model that:

- absorbs every quirk in §2.2 without per-resource controller branching,
- subsumes the four kinds from §6.1 as four canned profiles,
- gives end users the same instance-level controls Crossplane and ASO users already expect, and
- composes cleanly with the kro authoring surface accepted in [E9] without forcing kro to understand transport.

### 11.2 The operation profile schema

An operation profile describes, for one Graph resource type, the transport-level recipe for each of five lifecycle slots: `observe`, `create`, `update`, `delete`, and zero or more named `actions`. Each slot carries enough metadata for the generic reconciler to execute it without resource-specific code.

```yaml
operationProfile:
  apiVersion: v1.0           # or beta; matches §2.1 metadata channel
  consistency:
    writeWaitPolicy: poll-until-observed   # see §2.2 eventual consistency
    consistencyHeader: eventual             # see §2.2 advanced queries
  throttle:
    retryAfterHeader: Retry-After           # see §2.2 throttling
    perTenantBucketHint: low

  observe:
    method: GET
    path: /groups/{externalId}
    pagination: odataNextLink               # @odata.nextLink for collections
    notFoundIs: deleted                     # 404 → reconcile as drift

  create:
    method: POST
    path: /groups
    bodyTemplate: ${spec.body}
    responseCapture:
      externalId: $.id
    async:
      mode: sync                            # or: location-header-lro
    onConflict: adoptByKeyField             # see §5.4 adopt-or-create

  update:
    method: PATCH
    path: /groups/{externalId}
    bodyTemplate: ${diff(spec.body, status.observed)}
    ifMatchFrom: status.etag                # optional optimistic concurrency

  delete:
    method: DELETE
    path: /groups/{externalId}
    softDelete:
      kind: two-phase                       # see §2.2 soft-delete
      purgePath: /directory/deletedItems/{externalId}
      purgeWhen: spec.deletionPolicy == "purge"

  actions:
    - name: addPassword
      method: POST
      path: /applications/{externalId}/addPassword
      bodyTemplate: ${spec.passwordCredential}
      responseCapture:
        secretRef: $.secretText             # one-shot read; see §5.5
```

Every field listed above maps to a quirk already documented in §2.2 or earlier. The table below shows the mapping explicitly so that adding a new quirk to §2.2 in the future has an obvious landing slot in the profile schema rather than a new code path in the controller.

| §2.2 quirk | Profile expression |
|---|---|
| Singleton (no POST/DELETE) | `create.method: PATCH` against the singleton path, treated as upsert-no-op when already present; `delete: { method: noop, warn: true }` |
| Action endpoint (POST RPC) | Entry under `actions[]`; never invoked by reconciliation, only by user-triggered subresource or sibling CR |
| OData function (GET RPC) | Same as actions but `method: GET`; result captured into `status` |
| `$ref` reference collection | `create.method: POST` to `…/{nav}/$ref` with `@odata.id` body; `delete.method: DELETE` against `…/{nav}/{id}/$ref`; `observe` paginated GET on the navigation property |
| Polymorphic types (`@odata.type`) | `bodyTemplate` always emits the discriminator; `observe` records it into `status.kind` so updates round-trip |
| LRO (202 + Location) | `async.mode: location-header-lro`; resume token stored as annotation per §5.2 |
| Eventual consistency | `consistency.writeWaitPolicy: poll-until-observed` plus `consistencyHeader: eventual` on follow-up reads |
| Application/ServicePrincipal split | Two profiles that share a `linkedProfile` reference; the SP profile's `create.onMissingLink` triggers the Application profile first |
| Soft-delete + restore | `delete.softDelete.kind: two-phase` with `purgePath` and a policy gate |
| Throttling + `Retry-After` | `throttle` block, honoured by the generic client; per-tenant bucket sized per §5 |

The profile is intentionally declarative: there are no hooks, no scripts, and no per-resource Go callbacks. This is the same constraint trait #4 already imposes on the provider operation contract [E7][E8] and is the property that lets the generic reconciler handle any new Graph endpoint without a rebuild.

### 11.3 Per-instance `managementPolicies`

The operation profile says what a resource type *can* do. The CR `spec.managementPolicies` field says what the controller *should* do on this particular instance. The effective allowed set is the intersection:

```
effective(slot) = (slot in profile.supports) AND (slot in spec.managementPolicies)
```

```yaml
apiVersion: graph.example.com/v1
kind: Group
spec:
  managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]
  body: { displayName: "Platform Engineering", mailEnabled: false, securityEnabled: true }
```

Omitting the field defaults to the full set the profile supports (current ASO behaviour). `LateInitialize` follows the Crossplane semantic: copy server-defaulted fields into `spec` once after the first successful observe, then never again.

The model deliberately reuses prior art rather than inventing a new vocabulary:

| Project | Field | Granularity | Notes |
|---|---|---|---|
| Crossplane managed resources | `spec.managementPolicies: [Observe, Create, Update, Delete, LateInitialize]` | Per instance, declarative array | Default `[*]`; `[Observe]` makes the resource observe-only; combinations supported |
| Azure Service Operator | `serviceoperator.azure.com/reconcile-policy` annotation | Per instance, enumerated string | Values include `manage`, `detach-on-delete`, `skip` — coarser than Crossplane but covers the most common cases |
| Terraform | `lifecycle { prevent_destroy, ignore_changes, create_before_destroy, replace_triggered_by }` | Per resource block, declarative | Fragmented across multiple keys; no explicit observe-only mode |

For this design we recommend the Crossplane array form because it is the most expressive of the three and composes naturally with the profile (intersection is well-defined per slot). The ASO annotation values can be expressed as presets — `manage` = `[Observe, Create, Update, Delete, LateInitialize]`, `detach-on-delete` = `[Observe, Create, Update, LateInitialize]`, `skip` = `[]` — and surfaced as a `mode` shortcut for users who prefer the annotation idiom.

If a user requests a slot the profile does not support (e.g. `Delete` on a singleton), the admission webhook from §10.2 rejects the manifest with a remediation hint pointing at the profile's `supports` set; this is the same fail-fast mechanism, applied to a different axis.

### 11.4 Collapsing the four archetypes

With the profile model in place, §6.1's four CRDs are best understood as four *preset profiles* rather than four schemas:

| §6.1 kind | Equivalent preset profile |
|---|---|
| `GraphResource` | All five slots active; `create` = POST, `update` = PATCH, `delete` = DELETE, `observe` = GET |
| `GraphUpdateResource` | `create.method: PATCH` (upsert-no-op), `delete: { method: noop }`, `update` = PATCH, `observe` = GET; `actions: []` |
| `GraphReferenceCollection` | `create/delete` against `…/$ref`; `update` decomposes into add/remove diff; `observe` paginated GET on the navigation property |
| `GraphAction` | All CRUD slots `noop`; one entry in `actions[]`; status carries last-invocation result |

This is the runtime form of §10.3 Option B. The user-facing surface can still be either:

- **Four typed CRDs** that internally instantiate the matching preset (good for OpenAPI strictness, schema documentation, kubectl explain UX);
- **One generic `GraphResource` CRD** with `spec.profileRef` (string preset name, e.g. `"reference-collection"`) or inline profile override (good for the long tail, mirrors `terraform-provider-azapi`).

Both can coexist, and a small number of high-value typed CRDs (the ones listed in §6.2) can ship alongside the generic CRD without duplicating logic — they are thin wrappers that fix `profileRef` and tighten the body schema.

### 11.5 Where the profile comes from

Three sources, in order of preference:

1. **Auto-derived from CSDL/OpenAPI by the generator.** The same signals listed in §10.1 (capability annotations, `EntityContainer` membership, bound actions and functions) determine every field of the profile mechanically. `InsertRestrictions/Insertable=false` ⇒ `create` becomes upsert-no-op; `<Action Name="addPassword" IsBound="true">` ⇒ a new `actions[]` entry; navigation property with `ContainsTarget="false"` ⇒ `$ref` collection shape. This produces a baseline profile for every endpoint in v1.0 and beta without human input, and it is the canonical IR contract recorded in [E10] (`docs/decisions/accepted/0004-schema-ingestion.md`).
2. **Curated overrides for known-broken or ambiguous endpoints.** A small, version-controlled table inside the operator image patches the generated profile for endpoints where the CSDL is wrong, missing capability annotations, or describes a shape the runtime cannot honour. This is the same kind of override surface called out as a follow-up in [E10] and the open action-modeling question in [E11].
3. **User-supplied override on the CR spec.** An escape hatch for endpoints that ship before the generator catches up, or for tenant-specific extensions. This is gated by a cluster-scoped admin opt-in (`GraphOperatorConfig.allowInlineProfileOverrides: true`) because, as §11.8 notes, it is a privilege-escalation vector if left open by default.

### 11.6 Worked examples

#### 11.6.1 Normal entity — `Group` with full CRUD

```yaml
apiVersion: graph.example.com/v1
kind: Group
metadata: { name: platform-eng }
spec:
  # managementPolicies omitted → defaults to profile.supports
  body:
    displayName: Platform Engineering
    mailEnabled: false
    mailNickname: platform-eng
    securityEnabled: true
# Effective profile (auto-derived, shown for clarity, not normally written by user):
# create:  POST   /groups
# observe: GET    /groups/{id}
# update:  PATCH  /groups/{id}
# delete:  DELETE /groups/{id}  (soft-delete: two-phase, restore-window 30d)
```

#### 11.6.2 Singleton with restricted policy — `authenticationMethodsPolicy`

```yaml
apiVersion: graph.example.com/v1
kind: AuthenticationMethodsPolicy
metadata: { name: tenant-default }
spec:
  managementPolicies: ["Observe", "Update"]   # explicit; refuse Create/Delete even if profile allowed them
  body:
    registrationEnforcement:
      authenticationMethodsRegistrationCampaign:
        state: enabled
        snoozeDurationInDays: 1
# Profile (preset = "singleton"):
# create:  PATCH /policies/authenticationMethodsPolicy   (upsert-no-op)
# observe: GET   /policies/authenticationMethodsPolicy
# update:  PATCH /policies/authenticationMethodsPolicy
# delete:  noop  (warn: "singleton; nothing to delete")
```

The `managementPolicies` array makes the operator-side guarantee explicit even though the profile's `create` and `delete` are already inert; this protects the resource from a future profile change that adds destructive semantics.

#### 11.6.3 PIM ticket — same endpoint, different `action` field

This is the case from the sibling research file (`some-api-s-does-not-use-http-methods-for.md`), expressed in the profile schema:

```yaml
apiVersion: graph.example.com/v1
kind: PimGroupAssignmentRequest
metadata: { name: oncall-prod }
spec:
  managementPolicies: ["Observe", "Create", "Update", "Delete"]
  body:
    principalId: 11111111-1111-1111-1111-111111111111
    groupId:     22222222-2222-2222-2222-222222222222
    accessId:    member
    scheduleInfo: { startDateTime: "2026-06-01T00:00:00Z", expiration: { type: noExpiration } }
    justification: "On-call rotation"
# Profile (preset = "ticket-action-field"):
# create: POST /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
#         bodyTemplate.action = adminAssign
# update: POST /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
#         bodyTemplate.action = adminUpdate
# delete: POST /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
#         bodyTemplate.action = adminRemove
# observe: GET /identityGovernance/privilegedAccess/group/assignmentScheduleRequests/{id}
# actions:
#   - name: selfActivate
#     method: POST
#     path:   /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
#     bodyTemplate.action = selfActivate
```

This is the example that motivates separating the profile from HTTP-verb assumptions: the three CRUD slots resolve to the same `POST` against the same path, differing only in the rendered request body. No per-resource controller code is required.

#### 11.6.4 `$ref` collection — group members

```yaml
apiVersion: graph.example.com/v1
kind: GroupMembers
metadata: { name: platform-eng-members }
spec:
  groupRef: { name: platform-eng }
  members:
    - { kind: User,             id: aaaa1111-... }
    - { kind: ServicePrincipal, id: bbbb2222-... }
# Profile (preset = "reference-collection"):
# observe: GET /groups/{groupId}/members
#          pagination: @odata.nextLink
# create:  POST /groups/{groupId}/members/$ref
#          bodyTemplate: { "@odata.id": "https://graph.microsoft.com/v1.0/directoryObjects/{id}" }
# delete:  DELETE /groups/{groupId}/members/{id}/$ref
# update:  decomposes into add/remove diff against status.observedMembers
```

The reconciler diffs `spec.members` against `status.observedMembers` and issues batched `POST …/$ref` and `DELETE …/$ref` calls; the profile does not need to know about diffing — that is a property of the `reference-collection` preset bound to this profile shape.

### 11.7 Composition with kro

The profile is invisible at the kro authoring layer. End users writing a ResourceGraphDefinition see only the typed CRD (or the generic `GraphResource`) and its `spec.managementPolicies` field; the profile lives one layer down inside the operator, exactly as the kro design doc requires for provider-layer concerns [E13].

The `managementPolicies` field, however, is fair game for RGD authors. A common request is to expose a single user-friendly `mode` string and translate it inside the RGD using kro's CEL surface:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata: { name: managed-group }
spec:
  schema:
    apiVersion: v1alpha1
    kind: ManagedGroup
    spec:
      displayName: string
      mode: string | enum=["readonly","managed","detach-on-delete"] | default="managed"
  resources:
    - id: group
      template:
        apiVersion: graph.example.com/v1
        kind: Group
        metadata: { name: ${schema.spec.displayName} }
        spec:
          body: { displayName: ${schema.spec.displayName}, securityEnabled: true, mailEnabled: false }
          managementPolicies: ${
            schema.spec.mode == "readonly"          ? ["Observe"] :
            schema.spec.mode == "detach-on-delete"  ? ["Observe","Create","Update","LateInitialize"] :
                                                       ["Observe","Create","Update","Delete","LateInitialize"]
          }
```

This keeps the RGD declarative and gives platform teams a knob that maps onto Crossplane and ASO idioms without leaking transport details. It reuses exactly the CEL pattern kro already supports for `instance.spec` expressions on resource templates.

### 11.8 Risks and open questions

| Risk | Discussion |
|---|---|
| **Where does the profile live at runtime?** Three plausible homes: (a) compiled into the operator binary at codegen time (fast, but every Graph schema bump requires an operator release); (b) shipped as a `ConfigMap` reloaded on startup (decoupled, but the operator and the data can drift in production); (c) a `GraphResourceType` CRD (a CRD-of-CRDs) reconciled like any other resource (most flexible, but adds a bootstrap dependency). Recommendation: ship (a) as the default and (b) as an override; reserve (c) until multi-tenant deployments demand it. |
| **Drift between operator version and live Graph API.** The CSDL the generator consumed at build time can diverge from `https://graph.microsoft.com/v1.0/$metadata` at any point. A reconciler that hits a 4xx because the path moved or a capability flipped must surface a structured condition (`reason: ProfileDrift`) rather than retry blindly. A periodic CSDL-fetch sidecar can compare live metadata to the embedded profile set and emit metrics; this is cheaper than full re-derivation. |
| **User-supplied profiles as a privilege-escalation vector.** A malicious or careless user with `create` permission on `GraphResource` could point `delete.path` at `/directoryRoles/{id}/members/$ref/{ownerId}` and use the operator's app permissions to remove themselves from oversight. Inline profile overrides MUST be off by default, gated by a cluster-scoped flag, and ideally additionally constrained by an admission policy that whitelists path prefixes. The curated-override table from §11.5(2) is the safer mechanism for legitimate one-off patches. |
| **Beta → v1.0 promotion.** When Graph promotes a resource from beta to v1.0, the path, body shape, and capability annotations may all change. The profile's `apiVersion` field pins the channel, but instances pinned to `beta` will keep using the old profile after promotion. A version-aliasing layer in the generator (`v1.0-or-beta-fallback`) handles the common case where the beta profile is forward-compatible; for the rest, an operator-emitted `Deprecated` condition with the promoted v1.0 path is the minimum acceptable migration aid. |
| **Profile expressiveness ceiling.** The schema in §11.2 is deliberately small. Endpoints that need request signing, multi-step orchestration, or non-OData pagination will hit the ceiling. The escape hatch is the `actions[]` mechanism plus a sibling CR that drives the orchestration — not extending the profile vocabulary, because every new field becomes a new code path in every reconciler. |

---

## 12. Auth-context capability and delegated-only endpoints

A category of Microsoft Graph endpoints accepts only **delegated** permissions — a token obtained on behalf of a signed-in user — and rejects **application** permissions issued under the client-credentials grant (the basis of workload identity, managed identity, and certificate-based service principals). Under the auth model recommended in §5.3, the operator runs with an application context and will receive `403 Authorization_RequestDenied` for every such endpoint, at reconcile time, with no preventative signal. This section describes how the operator surfaces that constraint *before* any cloud call is issued, and why the scope is deliberately limited to **hard-blocking** rather than introducing a delegated-token broker.

### 12.1 The concrete failure mode

A user installs the operator, configures it with AKS Workload Identity per §5.3, and applies:

```yaml
apiVersion: graph.example.com/v1
kind: GraphResource
spec:
  url: /me/messages
  body: { subject: "Test", body: { contentType: Text, content: "Hello" }, toRecipients: [{ emailAddress: { address: "ops@example.com" } }] }
```

The reconciler issues `POST https://graph.microsoft.com/v1.0/me/messages` with the workload-identity token. Graph responds:

```
HTTP/1.1 403 Forbidden
{ "error": { "code": "Authorization_RequestDenied",
             "message": "/me requests are not supported when using application permissions. ..." } }
```

The CR sits with `Ready=False, reason=ProviderError`, the message buried inside a generic OData wrapper. The user has no way to know — short of reading the Graph docs for every endpoint they manage — that `/me/*` is *structurally* unreachable from this operator's credential context, not a transient or permission-scope problem. The same pattern recurs for endpoints that require a signed-in user identity (e.g. self-service access-package requests, certain Teams chat-as-user operations, parts of PIM self-activation, some `reports/` endpoints).

### 12.2 Where the delegated-only signal lives in Graph metadata

The "delegated vs application" distinction is documented on a per-endpoint basis across three sources, none of which is a complete, structured feed:

| Source | Form | Coverage | Usability |
|---|---|---|---|
| `microsoftgraph/microsoft-graph-docs-contrib` per-API markdown files | Free-text "Permissions" tables listing Delegated (work/school), Delegated (personal), Application | Effectively complete (every documented endpoint has one) | Unstructured markdown; rows like "Not supported" must be parsed to detect delegated-only |
| Graph CSDL capability annotations | Partial — `Org.OData.Capabilities.V1.PermissionType` for some entities; many endpoints are missing or list only one of the two contexts | Sparse | Structured but cannot be trusted alone |
| Microsoft Graph PowerShell SDK permission tables | Generated per cmdlet from the same docs source | Mirrors docs-contrib coverage | Easier to scrape (one row per cmdlet) but already derived |

The consequence is that the operator cannot infer auth-context support purely from the CSDL/OpenAPI artefacts in §2.1. A **curated table**, seeded from the docs-contrib markdown and refreshed in CI, is the practical source of truth. The table maps `(method, path-template) → { application: yes|no, delegated: yes|no }` and ships alongside the operation profiles described in §11. This is the same shape as the curated-override mechanism already required by §11.5(2); §12 simply adds a second column to it.

### 12.3 Representing auth-context capability in the operation profile

Extend the operation profile from §11.2 with one new field per lifecycle slot:

```yaml
operationProfile:
  # ... fields from §11.2 omitted for brevity ...
  create:
    method: POST
    path: /me/messages
    authContexts: [Delegated]              # NEW; default is [Application, Delegated]
  observe:
    method: GET
    path: /me/messages/{id}
    authContexts: [Delegated]
  update:
    method: PATCH
    path: /me/messages/{id}
    authContexts: [Delegated]
  delete:
    method: DELETE
    path: /me/messages/{id}
    authContexts: [Delegated]
```

`authContexts` is a set, not an enum, because many endpoints accept both. The default — applied when neither the CSDL nor the curated table says otherwise — is `[Application, Delegated]` (permissive). The curated table provides the override for endpoints known to be delegated-only. This keeps the §11 profile schema closed under extension: every quirk lands as a new field that defaults to the most permissive value, so unrecognised endpoints stay reachable.

### 12.4 The credential context the operator advertises

At startup, the operator classifies each configured credential and publishes the resulting set on a leader-elected `GraphOperatorStatus` CR and a Prometheus gauge:

| Credential mode (per §5.3) | Context produced |
|---|---|
| Workload Identity (federated token, default in-cluster) | `Application` |
| User-assigned managed identity | `Application` |
| Client certificate / client secret | `Application` |
| (none configured for delegated) | — |

In the current design, the operator's context set is always `{Application}`. There is no in-cluster path that produces a `Delegated` context, by deliberate scope choice (see §12.8). Operators running with multiple credential profiles (per the three-level hierarchy in §5.3) take the union across all configured credentials.

### 12.5 The hard-block check

The check is a slot-level set intersection between the credential contexts the operator can supply and the contexts each slot requires.

```
for slot in effective_slots(spec.managementPolicies, profile.supports):
    required = profile[slot].authContexts
    if required ∩ operator.contexts == ∅:
        reject(slot, required, operator.contexts)
```

This runs in two places:

1. **Admission webhook (primary defence; extends §10.2).** Rejects the manifest before any cloud call. A representative rejection:

   ```
   Error: Group "platform-eng-mail" cannot be admitted.
   The "/me/messages" endpoint requires a Delegated credential context for slot "Create".
   This operator is running with credential contexts: [Application].
   No application-permission path is available for /me/* endpoints.

   Remediation:
     - Use the Application-scoped equivalent: POST /users/{id}/messages (requires Mail.Send).
     - Or restrict this resource to observation only: spec.managementPolicies: ["Observe"]
       (this slot also requires Delegated and will be rejected).
     - Brokering a delegated token from inside the cluster is not supported in this version.
   ```

2. **Reconciler-time defence-in-depth.** The same check runs immediately before the reconciler issues a transport call, mapped to a structured condition rather than a webhook rejection:

   ```yaml
   status:
     conditions:
     - type: Ready
       status: "False"
       reason: AuthContextUnavailable
       message: >
         Slot "Create" on /me/messages requires Delegated; operator contexts are [Application].
         Manifest should not have reached the reconciler — admission webhook may be disabled.
   ```

   This catches three cases the webhook misses: webhook bypass via `--validate=false`, webhook unavailability at admission time, and post-admission changes to the operator's credential context (e.g. a Secret rotation that drops a credential).

### 12.6 Surfacing the constraint before users write YAML

Three additional touchpoints make the constraint visible without requiring a `kubectl apply` attempt:

- **`kubectl graph explain <url>` (extends §10.5).** Prints the required auth contexts alongside the archetype, permissions, and example YAML. For `/me/messages` it prints `Required auth contexts: [Delegated]` and `Application-permission alternative: /users/{id}/messages`.
- **Generated CRD documentation.** Typed CRDs that wrap a delegated-only profile (or any profile with at least one delegated-only slot) carry a banner in the generated reference docs and a `graph.example.com/auth-contexts: Delegated` label on the CRD object itself, so downstream tooling can filter on it.
- **Operator startup log.** A single summary line aids capacity planning: `"operator running in Application-only mode; 137 of 1,021 shipped profiles have at least one slot that requires Delegated and will be unreachable"`. The same numbers populate a `graph_operator_unreachable_profiles` gauge for dashboard alerting.

### 12.7 Mixed-context resources

Many resources are only *partially* unreachable: `GET /users/{id}/messages` accepts Application (`Mail.Read`), `POST /users/{id}/sendMail` accepts Application (`Mail.Send`), but `POST /me/messages` does not. The profile expresses the constraint per slot, so partial manageability follows directly from the §11 effective-slot rule:

```
effective(slot) = (slot in profile.supports)
                ∩ (slot in spec.managementPolicies)
                ∩ (operator.contexts ∩ profile[slot].authContexts ≠ ∅)
```

A user who pins `spec.managementPolicies: ["Observe"]` on a resource whose `Observe` slot accepts `[Application, Delegated]` is admitted even if the `Create`/`Update`/`Delete` slots are delegated-only — those slots are inactive for this instance and are not checked. This preserves the §11.3 guarantee that the per-instance policy is the ultimate authority over what the controller is allowed to attempt.

### 12.8 Explicitly out of scope (future work)

The following are deliberately not part of this design and should be tracked as separate proposals if needed:

| Out-of-scope item | Why deferred |
|---|---|
| **Delegated-token broker (OBO, refresh-token Secret, device-code).** Would let the operator drive delegated-only endpoints by exchanging a user token. | Each variant has a real cost: OBO requires a user-facing frontend to mint the initial token; long-lived refresh tokens have rotation, theft, and storage problems Kubernetes Secrets do not solve well; device-code is interactive and incompatible with controller loops. None can be added cleanly without changing the §5.3 auth model. Brokering is the obvious v2 follow-up but does not belong in the first cut. |
| **Per-CR principal override.** Run a given reconcile under a different identity. | Already covered by the three-level credential hierarchy in §5.3. That mechanism changes *which* principal is used, not the *type* of credential; it cannot turn an Application-context operator into a Delegated-context one. |
| **Conditional Access policy blocking application permissions.** A tenant CA rule can block app tokens for specific resources even when the permission grant is correct. | Produces the same 403 surface but with different remediation (admin must adjust CA, not switch credential type). Surfaced via the same `AuthContextUnavailable` condition but with a distinct reason code (`ConditionalAccessBlocked`); design of the CA-detection path is separate. |
| **Tenant-side feature flags that gate application support.** Certain governance APIs accept Application only after a tenant admin enables a preview or grants an additional consent. | The profile cannot statically know this. Treated as a refinement of §12.9's curated-table-lag risk: the runtime fallback (403 → suggest override) handles it, but a structured representation is left for follow-up. |

### 12.9 Risks and open questions

| Risk | Discussion |
|---|---|
| **Curated table lags Graph changes.** New endpoints, beta-to-v1.0 promotions, and silent docs corrections can all flip an endpoint's auth-context support between operator releases. | Mitigation has two parts: (1) a weekly CI job that fetches `microsoftgraph/microsoft-graph-docs-contrib` and diffs the parsed permission tables against the shipped curated table, opening a PR with deltas; (2) a runtime fallback that maps `403 Authorization_RequestDenied` with the specific `/me` and known-delegated-only sub-strings into `reason: AuthContextLikelyDelegatedOnly` and recommends adding the endpoint to the override table. This converts a misclassification from a silent failure into an actionable PR. |
| **Over-blocking.** Marking a slot delegated-only when it actually accepts application permissions in some tenants (because the tenant granted an additional preview permission, or uses a non-Commercial cloud where the surface differs) rejects legitimate usage. | Per-tenant override via the same admin-gated mechanism described in §11.5(3): the cluster admin can flip an entry from `[Delegated]` to `[Application, Delegated]` for a known good endpoint without rebuilding the operator. The override is logged and emits a metric so drift between operator and override set is observable. |
| **Coverage of sovereign clouds.** §2.1 lists six sovereign CSDL variants; the docs-contrib markdown describes the commercial cloud only. Some sovereign tenants restrict endpoints differently. | Out of scope for the first cut; documented as a known gap. Tenant override is the escape hatch. |
| **Webhook unavailability degrades the guarantee.** With the webhook down, manifests reach the reconciler and the user only sees `AuthContextUnavailable` after the fact. | Acceptable: the reconciler check guarantees no failed Graph call is issued, only that the failure is surfaced later than ideal. The webhook is a UX layer, not a security boundary. |
| **The user-supplied profile escape hatch from §11.5(3) can be used to bypass `authContexts`.** A user with permission to inline a profile could write `authContexts: [Application]` against a delegated-only endpoint and still 403 at runtime. | Already mitigated by the admin gate on inline profile overrides; the runtime check in §12.5 will still produce `AuthContextLikelyDelegatedOnly` and surface the mistake. No additional control is warranted. |

---
