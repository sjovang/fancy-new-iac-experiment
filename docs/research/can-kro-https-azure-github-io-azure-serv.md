# Research report: kro + Azure Service Operator (ASO) for Azure-only scope, and Entra operator maturity

**Date:** 2026-05-19  
**Question:**  
1. Can **kro + ASO** be a good existing alternative if scope is reduced to **Azure-only**?  
2. Does a service operator for **Microsoft Entra ID** exist?  
3. If yes, is it mature or limited?

## Executive answer

Yes — **kro + ASO is a credible Azure-only alternative** if you want reusable higher-level APIs/compositions on top of Azure resources managed in Kubernetes. The fit is strong because kro is explicitly designed to compose any K8s resource (including CRDs), and its docs call out ASO as an example integration.  
However, this stack is a tradeoff: **ASO is mature/stable, but kro is still pre-1.0 (`v1alpha1` API) and can introduce breaking changes**, so platform risk is higher than using ASO alone.

For Entra ID: **an operator path exists inside ASO**, but it is currently **very limited**. ASO has an `entra.azure.com` API group, and the published supported resource list currently shows **one released resource: `SecurityGroup` (supported from v2.14.0)**. That is real support, but it is narrow compared to the breadth of Entra entities available via Microsoft Graph (users, groups, apps, service principals, roles, etc.).

---

## Findings

### 1) Is kro + ASO a good Azure-only alternative?

## Why the combo is viable

1. **kro composes arbitrary K8s resources and infers dependencies** from CEL expressions; it is not cloud-specific.  
   - kro overview: “works with any Kubernetes resource - native or CRD,” and explicitly cites ASO for Azure Blob examples.  
   Source: <https://kro.run/docs/overview/>
2. **ASO is Azure-native and stable**, with broad ARM coverage and active releases.  
   - “Project is stable.”  
   - “Supports more than 150 Azure resources.”  
   - “CRDs generated from Azure Resource Manager schemas.”  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/_index.md>
3. **Azure-only alignment is explicit in ASO positioning.**  
   - ASO FAQ: “ASO is not and will not ever be multi-cloud.”  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/guide/frequently-asked-questions.md>

## Where this combo helps most

- You want to keep Azure provisioning in Kubernetes but expose **higher-level app/platform APIs** for developers.
- You need **DAG inference and ordering** across multiple resources at composition level (kro strength), while retaining ASO’s Azure reconciliation.
- You want to avoid writing/owning a custom operator/controller for composition logic.

## Key caveats

1. **kro maturity risk:** kro README states API is currently `v1alpha1` and may introduce breaking changes.  
   Source: <https://github.com/kubernetes-sigs/kro/blob/main/README.md>
2. **Two-controller operational model:** you operate both kro and ASO (upgrades, auth, observability, failure debugging across layers).
3. **ASO alone does not provide full composition language/DAG abstractions**; FAQ explicitly says ASO does not have general-purpose DAG support or templating.  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/guide/frequently-asked-questions.md>

## Bottom line on Q1

For an Azure-only platform, **kro + ASO is a strong “buy instead of build” option** if your goal includes opinionated abstractions/compositions.  
If your goal is only direct Azure resource management from K8s (without high-level composition), **ASO alone** is usually simpler and lower-risk.

---

### 2) Does a service operator for Entra ID exist?

**Yes, within ASO v2 there is Entra support** (`entra.azure.com/*`), including controller/config guidance and API types.

Evidence:

1. Entra CRD pattern exists in ASO supported resources docs (`entra.azure.com/*`).  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/reference/entra/_index.md>
2. ASO has Entra-specific API group/types and reconcilers.  
   Sources:  
   - <https://github.com/Azure/azure-service-operator/tree/main/v2/api/entra/v1>  
   - <https://github.com/Azure/azure-service-operator/blob/main/v2/api/entra/v1/securitygroup_types.go>  
   - <https://github.com/Azure/azure-service-operator/blob/main/v2/api/entra/v1/doc.go>
3. Entra requires extra config (`ENTRA_APP_ID`) in ASO settings.  
   Sources:  
   - <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/guide/aso-controller-settings-options.md>  
   - <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/guide/frequently-asked-questions.md>

---

### 3) Is Entra operator support mature or limited?

Current evidence indicates **limited scope, not broad Entra coverage**.

1. ASO Entra supported-resources page currently lists **one released resource**: `SecurityGroup` (CRD v1, supported from v2.14.0).  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/reference/entra/_index.md>
2. Entra v1 reference page content is centered on `SecurityGroup` schema/status.  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/docs/hugo/content/reference/entra/v1.md>
3. `v2/api/entra/v1` file layout is small and centered on `securitygroup_types.go` (plus boilerplate/versioning files).  
   Source: <https://github.com/Azure/azure-service-operator/tree/main/v2/api/entra/v1>
4. Code imports Microsoft Graph SDK for Entra resource reconciliation, confirming this is data-plane Graph integration, not broad ARM-generated coverage.  
   Source: <https://github.com/Azure/azure-service-operator/blob/main/v2/api/entra/v1/securitygroup_types.go>

Context: Microsoft positions full Entra identity/network access automation through **Microsoft Graph APIs** (users, groups, apps, roles, etc.), which is much broader than ASO’s current Entra resource surface.  
Source: <https://learn.microsoft.com/graph/api/resources/identity-network-access-overview?view=graph-rest-1.0>

## Bottom line on Q2/Q3

- A service-operator-style Entra implementation **does exist in ASO**.  
- It is currently **narrow** (SecurityGroup-focused), so treat it as **early/limited**, not a full Entra management plane.

---

## Maturity snapshot (from upstream metadata)

### ASO

- Repo created: 2019-07-18, actively updated, stable v2 positioning.  
  Source: <https://api.github.com/repos/Azure/azure-service-operator>  
- Recent stable tags include v2.16.1, v2.19.0, v2.18.0, v2.17.0, etc.  
  Source: <https://api.github.com/repos/Azure/azure-service-operator/releases?per_page=20>

### kro

- Repo created: 2024-09-12; active release train (`v0.9.x`) but still pre-1.0.  
  Source: <https://api.github.com/repos/kubernetes-sigs/kro>  
- Recent releases: v0.9.2, v0.9.1, v0.9.0, v0.8.x…  
  Source: <https://api.github.com/repos/kubernetes-sigs/kro/releases?per_page=20>  
- API stability note remains `v1alpha1` with possible breaking changes.  
  Source: <https://github.com/kubernetes-sigs/kro/blob/main/README.md>

---

## Practical recommendation

If your strategic decision is “Azure-only, reduce scope, ship faster”:

1. **Use ASO as the Azure control-plane baseline** (mature, broad Azure coverage).  
2. Add **kro only where you truly need reusable higher-level compositions** (service templates/platform APIs).  
3. For Entra, assume **limited operator coverage** today (SecurityGroup) and plan for **Graph/Terraform/CLI-based workflows** for broader Entra objects until coverage expands.

---

## Local project alignment (from this workspace)

Your repo already contains a **kro-style prototype** and mapping work:

- `prototypes/kro/README.md`  
- `prototypes/mapping.md`  
- `docs/research/usability.md` (notes kro can compose ASO/CRDs and highlights DAG inference)

These artifacts indicate that adopting kro+ASO is already directionally aligned with your current experimentation.

---

## Confidence and uncertainty

- **High confidence**: ASO maturity, kro compatibility model, and existence/limited scope of ASO Entra support (direct upstream evidence).
- **Moderate confidence**: ecosystem alternatives beyond ASO for Entra operator-style management, because this research focused on official ASO/kro/Microsoft sources rather than exhaustive community project benchmarking.

