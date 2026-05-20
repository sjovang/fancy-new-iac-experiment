# Research: Authentication model for multi-cloud/private-cloud APIs (OIDC-first + secure fallback)

## 1) Executive summary

For this IaC runtime, the strongest default is an **OIDC-first, short-lived token exchange model** per provider, with Kubernetes projected service account tokens as the base workload identity in-cluster. This aligns with your K8s-native direction and avoids static credentials by default.

Where OIDC federation is unavailable or incomplete, use a **tiered fallback**:

1. **First fallback:** X.509 / workload certificate based exchange for short-lived credentials (for platforms that support cert-based federation).
2. **Second fallback:** brokered secret retrieval with strict TTL + rotation (**OpenBao** / cloud secret managers), mounted ephemerally.
3. **Last resort:** provider-native long-lived credentials (API keys, client secrets), but wrapped in policy controls, audit, and aggressive rotation.

This should be captured as the next auth ADR (proposed `0008-authentication-model`) under `docs/decisions/proposed/`, consistent with your ADR process and lifecycle organization [`docs/decisions/README.md`](../../../../../../../../Users/tjs/Developer/Private/iac-experiment/docs/decisions/README.md).

---

## 2) Scope and assumptions

### In scope
- Authentication patterns for: **Azure**, **Entra ID / Microsoft Graph**, **AWS**, **GCP**, **OCI**, and private-cloud families (notably **VMware** and **OpenStack**).
- Preference for secretless/OIDC where possible.
- Secure fallback candidates, preferably open-source and strongly backed ecosystems.

### Assumptions
- Runtime is Kubernetes-native (per accepted ADR direction).
- Providers are thin API wrappers and should not require heavy hand-coded auth per resource type.
- Authentication must work for both cloud control plane APIs and enterprise/private APIs.

---

## 3) OIDC-first target architecture

### 3.1 Runtime identity source
- Use Kubernetes service account identity with projected, bounded tokens (TokenRequest / projected volume), not legacy long-lived SA token secrets:
  - Kubernetes now recommends TokenRequest-based short-lived tokens and projected volume mounting over legacy long-lived SA token secrets.  
    Sources:  
    - <https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection>  
    - <https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/>  
    - <https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/>  
    - <https://kubernetes.io/docs/concepts/configuration/secret/>

### 3.2 Provider exchange
- Exchange Kubernetes/JWT identity for provider-native short-lived access tokens:
  - **Microsoft Entra workload identity federation** supports federating external IdP tokens (including Kubernetes and SPIFFE/SPIRE) to Entra access tokens.  
    <https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation>
  - **AKS workload identity** shows Kubernetes SA token + OIDC issuer + Entra token exchange model.  
    <https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview>
  - **AWS STS AssumeRoleWithWebIdentity** explicitly supports OIDC-compatible IdPs and returns temporary credentials.  
    <https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html>
  - **EKS IRSA** operationalizes this with OIDC service account tokens.  
    <https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html>
  - **GCP Workload Identity Federation** supports OIDC/SAML and token exchange via STS, and is intended to avoid service account keys.  
    <https://cloud.google.com/iam/docs/workload-identity-federation>
  - **GKE Workload Identity Federation** is the K8s-native path in GCP.  
    <https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity>
  - **OCI OKE workload identities** provide workload identity path (enhanced clusters; SDK-oriented constraints apply).  
    <https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contenggrantingworkloadaccesstoresources.htm>

### 3.3 Broker pattern for provider adapters
- Build a common `AuthBroker` contract in your runtime:
  - input: workload principal context (`namespace`, `serviceAccount`, audience, provider)
  - output: provider auth material (typically short-lived bearer token or STS-style temporary key set)
  - centralize token caching, jittered refresh, revocation handling, and audit metadata.

This avoids embedding auth flow logic in each resource provider.

---

## 4) Fallback strategy when OIDC is not available

### 4.1 Fallback A (preferred): cert-based / federated alternatives
- **AWS IAM best practices** recommend temporary credentials and federation/role methods over long-lived keys; includes X.509-based IAM Roles Anywhere and web/SAML role assumption paths for off-cloud workloads.  
  <https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html>
- **OpenStack Keystone federation** supports SAML2/OIDC and external IdP delegation.  
  <https://docs.openstack.org/keystone/latest/admin/federation/introduction.html>

### 4.2 Fallback B: dynamic secret brokering (short-lived where possible)
- **OpenBao dynamic secret model** can generate dynamic credentials on demand and apply lease/revocation semantics (strong for short-lived fallback issuance).  
  - <https://openbao.org/>  
  - <https://github.com/openbao/openbao>
- **External Secrets Operator (ESO)** can sync external secret systems into Kubernetes for runtime consumption.  
  <https://external-secrets.io/latest/>
- **Secrets Store CSI Driver** mounts secrets from external stores into pods and is a Kubernetes SIG Auth subproject.  
  <https://github.com/kubernetes-sigs/secrets-store-csi-driver>

### 4.3 Fallback C (last resort): long-lived credentials with strict controls
- **Entra / Microsoft identity platform** supports client credentials with shared secret or certificate; Microsoft guidance emphasizes protecting credentials and prefers stronger methods (cert/federated).  
  - <https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow>  
  - <https://learn.microsoft.com/en-us/entra/identity-platform/certificate-credentials>
- **OCI API signing keys** are supported and operationally common, but are long-lived key material and must be managed securely.  
  <https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm>

---

## 5) Provider authentication pattern matrix

| Provider/API family | OIDC / federation support | Preferred pattern | Secure fallback(s) | Last-resort fallback | Notes / caveats |
|---|---|---|---|---|---|
| Azure Resource Manager (ARM) | Yes (via Entra workload identity federation) | K8s SA token -> Entra federated credential -> access token | Entra app cert (`private_key_jwt`) | Entra app client secret | Entra explicitly supports external IdP token trust and token exchange. |
| Microsoft Graph / Entra-protected APIs | Yes (same Entra federation model) | Same as above | App cert auth | App secret | Uniform with ARM because both are Entra-protected resources. |
| AWS APIs | Yes (OIDC + STS AssumeRoleWithWebIdentity; IRSA in EKS) | OIDC JWT -> STS temporary creds | SAML to STS; IAM Roles Anywhere (X.509) | IAM user access keys | AWS guidance strongly prefers temporary credentials and IAM roles. |
| GCP APIs | Yes (Workload Identity Federation + STS) | External token -> GCP STS federated token (direct or SA impersonation) | X.509 federation path | Service account keys | GCP explicitly positions WIF as alternative to service account keys. |
| OCI APIs | Partial/varies by surface | OKE workload identity (where applicable) | Dynamic group/instance principal patterns where suitable | OCI API signing keys | OKE workload identity has constraints (enhanced cluster, SDK support, no dynamic group use for workload identities). |
| VMware vSphere APIs | Federation and token/session paths exist; OIDC federation support depends on vSphere version/IdP integration | Federated auth where available -> session token | SAML/token session flows | Basic auth + session bootstrap | vSphere docs show basic and token-based approaches; federation support is version/IdP dependent. |
| VMware NSX-T APIs | Supports multiple schemes including basic/session and client cert options | X.509 principal identity or session-based patterns | Session cookie/XSRF model | Basic auth | NSX docs include basic, session, and cert-based principal identity flows. |
| OpenStack Keystone-based APIs | Yes (SAML2/OIDC federation) | Federated token issuance via Keystone | App credentials with expiry/access rules | Password-based auth flows | App credentials are safer than embedding user passwords, but still secret-based. |

Evidence URLs used for VMware/OpenStack rows:
- vSphere session/token model and auth method guidance:  
  - <https://developer.broadcom.com/xapis/vsphere-automation-api/latest/cis/cis-session/>  
  - <https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere-sdks-tools/8-0/an-introduction-getting-started-with-vsphere-apis-and-sdks-8-0/getting-started-with-vsphere-apis-and-sdks/authentication-with-vsphere-apis.html>
- NSX-T auth schemes (basic, session, X.509 principal identity):  
  - <https://developer.broadcom.com/xapis/nsx-t-data-center-rest-api/latest/>
- OpenStack federation and app credentials:  
  - <https://docs.openstack.org/keystone/latest/admin/federation/introduction.html>  
  - <https://docs.openstack.org/keystone/latest/user/application_credentials.html>  
  - <https://docs.openstack.org/api-ref/identity/v3/?expanded=authenticate-detail#password-authentication-with-scoped-authorization>

---

## 6) Candidate open-source building blocks for secure short-lived fallback

| Tool | Role in architecture | Why it fits | Governance / backing signal |
|---|---|---|---|
| SPIFFE / SPIRE | Workload identity plane across heterogeneous environments | Strong fit for cloud-agnostic workload identity and mTLS/JWT-SVID based trust | SPIFFE/SPIRE publicly states CNCF graduation status. (<https://spiffe.io/> and <https://github.com/spiffe/spire>) |
| OpenBao | Dynamic secret and credential brokering | Open-source Vault-like secret management with dynamic secrets, leases, and revocation | Community-governed open-source project with OpenSSF ecosystem signals. |
| Secrets Store CSI Driver | Secret projection from external stores to pods | Good for runtime mount pattern without baking secrets into manifests | Kubernetes SIG Auth subproject. |
| External Secrets Operator | Sync external secrets into K8s secrets | Operationally simple for teams already standardized on external secret managers | Widely used OSS operator; CNCF status not asserted in sources reviewed. |
| cert-manager | Certificate lifecycle automation (for cert-based fallback paths) | Useful for automating cert issuance/rotation where cert auth is used | Active Kubernetes ecosystem project (CNCF ecosystem linkage visible in project metadata). |

---

## 7) Proposed policy model (implementation-oriented)

Define provider auth policy as declarative intent:

```yaml
auth:
  strategyOrder:
    - oidcFederation
    - x509Federation
    - dynamicSecretBroker
    - staticCredential
  constraints:
    maxCredentialTTL: 3600s
    disallowStaticCredentialsByDefault: true
    requireAuditSubject: true
  providerOverrides:
    oci:
      allowStaticCredential: true
      staticCredentialMaxAge: 24h
```

Key behavior:
- Engine attempts strategies in order per provider capability.
- Policy can explicitly deny static credentials except on allowlisted providers/resources.
- Every issued credential must include origin metadata for audit (`provider`, `principal`, `resourceRef`, `strategy`, `expiry`).

---

## 8) Risks and weak points

1. **Provider inconsistency:** Auth surfaces differ significantly (especially private-cloud APIs), so “one adapter per provider family” is still required even with a shared broker.
2. **OIDC trust config complexity:** Issuer/audience/subject mismatches are common failure modes (notably strict matching in Entra federation).
3. **Fallback drift risk:** Teams may overuse static credentials unless policy makes static auth opt-in and visible.
4. **Operational latency:** Dynamic broker patterns can add startup/API latency; cache and prefetch strategy must be explicit.
5. **K8s Secret leakage risk:** If fallback writes into Kubernetes Secrets, encryption-at-rest + RBAC hardening are mandatory.

---

## 9) Recommended decision for your repo (next ADR candidate)

Given your architecture goals and current ADR state:
- Keep **OIDC federation as mandatory default path** for all provider integrations.
- Standardize an **AuthBroker interface** and require provider implementations to declare capability matrix (`oidc`, `x509`, `dynamic`, `static`).
- Enforce a **policy gate** that blocks static credentials unless explicitly approved per provider/resource class.
- Require **bounded credential TTL** and full auth event audit schema.

This naturally follows your current proposed ADR sequence and should be captured as a dedicated auth ADR under `docs/decisions/proposed/`, with references to existing ADR index/process in `docs/decisions/README.md`.

---

## 10) Uncertainty notes

- VMware product-line auth capabilities vary by version/deployment mode; the matrix above uses official vSphere/NSX docs but should be validated against your exact target versions.
- OCI has multiple identity modes (instance principals, dynamic groups, OKE workload identity, API signing keys) with different applicability; workload identity path has explicit constraints in current docs.
- CNCF status is explicitly confirmed here for SPIFFE/SPIRE from sources reviewed; for some other tools (ESO, OpenBao), governance/backing is described without over-asserting CNCF project status.

---

## 11) Source list (canonical URLs)

- Local repo context  
  - `docs/decisions/README.md`  
  - `docs/decisions/proposed/0004-schema-ingestion.md`

- Kubernetes identity/token model  
  - <https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection>  
  - <https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/>  
  - <https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/>  
  - <https://kubernetes.io/docs/concepts/configuration/secret/>

- Microsoft / Azure / Entra  
  - <https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation>  
  - <https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview>  
  - <https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow>  
  - <https://learn.microsoft.com/en-us/entra/identity-platform/certificate-credentials>

- AWS  
  - <https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html>  
  - <https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html>  
  - <https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html>

- GCP  
  - <https://cloud.google.com/iam/docs/workload-identity-federation>  
  - <https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity>

- OCI  
  - <https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contenggrantingworkloadaccesstoresources.htm>  
  - <https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm>  
  - <https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm>

- VMware / private cloud  
  - <https://developer.broadcom.com/xapis/vsphere-automation-api/latest/cis/cis-session/>  
  - <https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere-sdks-tools/8-0/an-introduction-getting-started-with-vsphere-apis-and-sdks-8-0/getting-started-with-vsphere-apis-and-sdks/authentication-with-vsphere-apis.html>  
  - <https://developer.broadcom.com/xapis/nsx-t-data-center-rest-api/latest/>

- OSS fallback tooling references  
  - <https://spiffe.io/>  
  - <https://github.com/spiffe/spire>  
  - <https://openbao.org/>  
  - <https://github.com/openbao/openbao>  
  - <https://github.com/kubernetes-sigs/secrets-store-csi-driver>  
  - <https://external-secrets.io/latest/>  
  - <https://github.com/cert-manager/cert-manager>
