# Research: Non-HTTP-verb CRUD semantics (PIM `action` / `type`) with a kro-based design

## Executive conclusion

Yes, this is **possible** with your current direction, but **not with kro alone**.

With your current decisions, kro should stay the declarative composition/orchestration layer, while provider adapters implement an explicit operation contract (`Observe`, `Create`, `Update`, `Delete`, `InvokeAction`). That contract already matches your design invariants and is specifically intended for APIs whose lifecycle semantics are not encoded in HTTP verbs.  

If the goal is “users can configure CRUD behavior easily” (including `action`/`type` mappings like `AdminAdd`/`AdminUpdate`), you need one additional layer: a provider-facing operation mapping model in your schema-ingestion/generation pipeline. This is feasible, but that part is still proposed/not finalized in your repo.

## What the APIs actually do (evidence)

### 1. Entra PIM (current v1 APIs) uses request objects with an `action` field

- `POST /roleManagement/directory/roleAssignmentScheduleRequests` drives multiple lifecycle behaviors by varying `action` (`adminAssign`, `adminUpdate`, `adminRemove`, etc.), rather than separate CRUD endpoints. [E4]
- The resource type (`unifiedRoleAssignmentScheduleRequest`) defines those same `action` values as the operation semantic. [E3]
- PIM for Groups (`privilegedAccessGroupAssignmentScheduleRequest`) follows the same pattern: one create endpoint with `action` selecting assign/update/remove/activate behavior. [E5][E6]

### 2. Azure-resource PIM (deprecated beta surface) uses `type` values like `AdminAdd` / `AdminUpdate`

- `governanceRoleAssignmentRequest` is explicitly described as a **ticket-modeled** lifecycle API, compared with directly exposing `POST/PUT/DELETE` on the target resource. [E1]
- The create endpoint `POST /privilegedAccess/azureResources/roleAssignmentRequests` uses `type` values (`AdminAdd`, `AdminUpdate`, `AdminRemove`, etc.) to choose operation semantics. [E2]

So the problem statement is real: these APIs encode lifecycle intent via request/action fields, not transport verbs.

## Fit against your current architecture decisions

Your own design already anticipates exactly this:

- Trait #4 requires operation control beyond HTTP verb assumptions and explicit modeling of CRUD + custom actions. [E7]
- Your provider operation model defines behavior-level handlers including `InvokeAction(actionName, payload)`. [E8]
- You accepted kro as the authoring surface, while explicitly leaving schema-ingestion/transform details as follow-up design work. [E9]

This means:

1. **Conceptually compatible:** Yes.
2. **Implemented end-to-end in current docs/design:** Not fully; key configuration machinery is still proposed. [E10][E11]

## Is “easy user configuration” possible with kro?

### Short answer

- **Possible with current design direction:** **Yes**, if operation mapping is owned by provider metadata/generation.
- **Possible using only raw kro RGD features:** **No**, not cleanly.

### Why kro alone is insufficient

kro is intentionally an in-cluster composition/orchestration system over Kubernetes resources. It is not itself a cloud API behavior engine/provider runtime. [E12][E13]

In kro project scope, provider layer implementation/abstraction is out of scope. [E13]  
So forcing end users to encode HTTP/action lifecycle semantics directly in RGDs would fight kro’s intended boundary.

## Practical design that fits your decisions

Implement an **operation profile** per generated resource type (in IR / provider package), then keep user-facing specs declarative:

```yaml
operationProfile:
  observe:
    transport: http
    method: GET
    path: /identityGovernance/privilegedAccess/group/assignmentScheduleRequests/{id}

  create:
    transport: http
    method: POST
    path: /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
    requestTemplate:
      action: adminAssign

  update:
    transport: http
    method: POST
    path: /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
    requestTemplate:
      action: adminUpdate

  delete:
    transport: http
    method: POST
    path: /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
    requestTemplate:
      action: adminRemove

  actions:
    activate:
      method: POST
      path: /identityGovernance/privilegedAccess/group/assignmentScheduleRequests
      requestTemplate:
        action: selfActivate
```

Then generated adapter logic maps:

- desired-state reconciliation → `PlanCreate`/`PlanUpdate` + `Create`/`Update`/`Delete`
- non-CRUD transitions → `InvokeAction`
- eventual consistency / async behavior → `GetOperationStatus`

This aligns with your provider contract and keeps users away from low-level API quirks. [E8][E10]

## Important gap to acknowledge

Your repo currently has this as direction/backlog, not finalized spec:

- Canonical IR + transform/override approach is **proposed**, not accepted yet. [E10]
- Action endpoint modeling (`Action` CR vs `actions[]`) is still an open implementation question. [E11]

So the right statement is: **feasible now in architecture, pending concrete schema/IR and generator decisions.**

## If you decide this is too much for the current kro track: alternatives

1. **Dedicated API-family controllers (no generic mapping):** fastest for one domain, but scales poorly and risks per-resource hacks.
2. **Azure-only split:** ASO for ARM resources + custom Entra/Graph operator for identity APIs. ASO is stable and broad for Azure resources, but Entra CRD coverage is limited (not full Graph lifecycle surface). [E14][E15]
3. **Generic “APIResource” controller first, kro second:** build one generic action-aware reconciler and compose it with kro; this can reduce bespoke controllers while preserving your kro authoring decision.

## Recommendation

Proceed with kro, but make one decision explicit in the next ADR:

**Adopt a first-class operation mapping model in the canonical IR/provider package, and require every generated resource to define Observe/Create/Update/Delete semantics plus optional action mappings.**

That gives you:

- support for PIM-style `action`/`type` APIs,
- deterministic reconciliation behavior,
- and a user experience where people declare desired state, not API choreography.

---

## Evidence references

- **[E1]** `governanceRoleAssignmentRequest` resource (deprecated), Microsoft Graph docs repo: describes ticket-modeled lifecycle vs direct `POST/PUT/DELETE`, and `type` values including `AdminAdd`, `AdminUpdate`, `AdminRemove`.  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/beta/resources/governanceroleassignmentrequest.md
- **[E2]** Create `governanceRoleAssignmentRequest` (deprecated): `POST /privilegedAccess/azureResources/roleAssignmentRequests` and operation table using `AdminAdd`, `AdminUpdate`, etc.  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/beta/api/governanceroleassignmentrequest-post.md
- **[E3]** `unifiedRoleAssignmentScheduleRequest` resource type: `action` encodes operation semantics (`adminAssign`, `adminUpdate`, `adminRemove`, etc.).  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/v1.0/resources/unifiedroleassignmentschedulerequest.md
- **[E4]** Create role assignment schedule request endpoint (`POST /roleManagement/directory/roleAssignmentScheduleRequests`) with `action` in request body.  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/v1.0/api/rbacapplication-post-roleassignmentschedulerequests.md
- **[E5]** `privilegedAccessGroupAssignmentScheduleRequest` resource type: operation semantics in `action` field.  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/v1.0/resources/privilegedaccessgroupassignmentschedulerequest.md
- **[E6]** Create group assignment schedule request endpoint (`POST /identityGovernance/privilegedAccess/group/assignmentScheduleRequests`) with `action` values.  
  https://github.com/microsoftgraph/microsoft-graph-docs-contrib/blob/main/api-reference/v1.0/api/privilegedaccessgroup-post-assignmentschedulerequests.md
- **[E7]** Local traits spec requires “operation control beyond HTTP verb assumptions.”  
  `docs/traits-spec.md` lines 31-34
- **[E8]** Local provider operation contract includes `Observe`, `Create`, `Update`, `Delete`, `InvokeAction`, `GetOperationStatus`.  
  `docs/provider-operation-model.md` lines 11-22
- **[E9]** kro authoring surface accepted; schema ingestion and transform model called out as out-of-scope follow-up.  
  `docs/decisions/accepted/0003-kro-authoring-surface.md` lines 47-55
- **[E10]** Canonical IR + generated kro artifacts is proposed direction, including transform/override and generated artifact contracts.  
  `docs/decisions/proposed/0004-schema-ingestion.md` lines 29-44
- **[E11]** Implementation research open question: action endpoints as separate `Action` CR vs `actions[]` block.  
  `docs/research/implementation.md` lines 35-37
- **[E12]** kro overview: composes Kubernetes resources and infers dependency graph from CEL; works with native/CRD resources.  
  https://kro.run/docs/overview/
- **[E13]** kro design doc scope: provider layer implementations/abstraction not in scope.  
  https://github.com/kubernetes-sigs/kro/blob/main/docs/design/kro.md
- **[E14]** ASO project status and breadth (`stable`, `150+` resources).  
  https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/_index.md
- **[E15]** ASO Entra support listing currently shows `SecurityGroup` in released resources.  
  https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/reference/entra/_index.md
