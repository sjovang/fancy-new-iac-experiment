# Research report: backing behind CUE vs kro (and foundation involvement)

**Date:** 2026-05-19  
**Question:** Compare the institutional backing behind **CUE** and **kro**; is CNCF or another foundation involved?

## Executive answer

- **kro has stronger foundation-adjacent backing** because it is a **Kubernetes SIG Cloud Provider subproject**, and Kubernetes itself is hosted by CNCF.  
  - kro states this directly in its README.  
  - Kubernetes states it is hosted by CNCF.  
- **CUE appears independent (community/project stewardship model), not foundation-hosted in the sources reviewed.**  
  - CUE docs/repo describe an open-source project with maintainers and named project stewards.  
  - I found no explicit CNCF/Linux Foundation hosting claim in CUE primary sources.
- **Neither appears as a standalone CNCF project page** at the checked URLs (`/projects/kro/`, `/projects/cue/` both 404), which suggests kro is backed *through Kubernetes* rather than as its own CNCF top-level project, and CUE is not a CNCF project (at least at time of this check).

---

## Side-by-side comparison

| Dimension | CUE | kro |
|---|---|---|
| Primary home/org | `cue-lang/cue` GitHub org/repo | `kubernetes-sigs/kro` GitHub org/repo |
| Stated governance/backing | Open-source project with maintainers and named **Project Stewards** in CoC; contribution flow references core maintainers | Explicitly a **subproject of Kubernetes SIG Cloud Provider** |
| Foundation tie in primary docs | No explicit CNCF/LF project-hosting claim found in CUE sources reviewed | Indirect CNCF tie via Kubernetes: Kubernetes is “hosted by CNCF” |
| Contribution legal model | DCO (`Signed-off-by`, Developer Certificate of Origin) | Under Kubernetes community governance where contributors sign CNCF CLA |
| CNCF standalone project page check | `https://www.cncf.io/projects/cue/` → 404 | `https://www.cncf.io/projects/kro/` → 404 |

---

## Evidence and interpretation

## 1) CUE backing

1. **CUE describes itself as open source** and documents technical roots (including Google history) in its intro docs.  
   Source: <https://cuelang.org/docs/introduction/>

2. **Governance signals are project-internal/community-oriented**:  
   - `CONTRIBUTING.md` references “core project maintainers” and that maintainers use GerritHub for review.  
   - CUE CoC identifies named **Project Stewards** and project-specific reporting channel (`conduct@cuelang.org`).  
   Sources:  
   - <https://github.com/cue-lang/cue/blob/master/CONTRIBUTING.md>  
   - <https://cuelang.org/docs/reference/code-of-conduct/>

3. **Contribution/legal model is DCO-style**, not CLA-style in the docs reviewed.  
   Source: <https://github.com/cue-lang/cue/blob/master/CONTRIBUTING.md>

4. **No explicit CNCF/LF hosting claim** found in CUE repo/doc sources reviewed.  
   Supporting checks:  
   - Repo search for “Cloud Native Computing Foundation” / “Linux Foundation” in `cue-lang/cue` returned no hits during this research.  
   - CNCF direct project URL for CUE is 404: <https://www.cncf.io/projects/cue/>

**Interpretation:** CUE appears to be backed by its own open-source project organization and maintainers/stewards, not a foundation-hosted governance umbrella (based on the examined sources).

---

## 2) kro backing

1. **kro explicitly states institutional position**: “kro is a subproject of Kubernetes SIG Cloud Provider.”  
   Source: <https://github.com/kubernetes-sigs/kro/blob/main/README.md>

2. **SIG Cloud Provider docs include kro as a subproject** and show formal SIG charter/governance structure.  
   Sources:  
   - <https://github.com/kubernetes/community/blob/master/sig-cloud-provider/README.md>  
   - <https://github.com/kubernetes/community/blob/master/sig-cloud-provider/CHARTER.md>

3. **Kubernetes itself is hosted by CNCF**, establishing kro’s indirect foundation tie through the Kubernetes project.  
   Source: <https://github.com/kubernetes/kubernetes/blob/master/README.md>

4. **Kubernetes governance requires CNCF CLA** for contributors.  
   Source: <https://github.com/kubernetes/community/blob/master/governance.md>

5. **kro is not (from these checks) a standalone CNCF project page** (`/projects/kro/` is 404), consistent with “CNCF-through-Kubernetes” rather than separate CNCF project branding.  
   Source: <https://www.cncf.io/projects/kro/>

**Interpretation:** kro has meaningful foundation-backed governance context via Kubernetes+CNCF, even if kro itself is not presented as a separate CNCF top-level project page.

---

## 3) Are CNCF or other foundations involved?

### CNCF involvement

- **kro:** **Yes, indirectly and materially** (as a Kubernetes SIG subproject; Kubernetes is CNCF-hosted).  
- **CUE:** **No direct evidence found** of CNCF project hosting/acceptance in primary CUE docs/repo; direct CNCF project URL returns 404.

### Other foundations

- In the sources reviewed, I did **not** find explicit “hosted by Foundation X” statements for CUE or for kro as a standalone project.  
- For kro, the operative foundation context is CNCF via Kubernetes governance.

---

## Practical takeaway for risk/strategy

- If your decision criterion is **formal ecosystem/governance anchoring in CNCF structures**, **kro is stronger** because it is embedded in Kubernetes SIG governance.  
- If your criterion is **independent language project with its own maintainer/steward model**, **CUE fits that profile**.

---

## Confidence and caveats

- **High confidence** on kro’s Kubernetes/CNCF linkage (explicit statements in kro, Kubernetes, and Kubernetes community docs).  
- **Moderate-high confidence** that CUE is not CNCF-hosted based on checked sources and 404 project URL; however, absence checks are inherently time-sensitive and should be periodically re-verified against CNCF project directories.

---

## Primary sources used

- CUE intro/docs: <https://cuelang.org/docs/introduction/>  
- CUE repo README: <https://github.com/cue-lang/cue/blob/master/README.md>  
- CUE contributing: <https://github.com/cue-lang/cue/blob/master/CONTRIBUTING.md>  
- CUE code of conduct: <https://cuelang.org/docs/reference/code-of-conduct/>  
- CUE security: <https://github.com/cue-lang/cue/blob/master/SECURITY.md>  
- CUE repo metadata API: <https://api.github.com/repos/cue-lang/cue>  
- CUE org metadata API: <https://api.github.com/orgs/cue-lang>  
- kro README: <https://github.com/kubernetes-sigs/kro/blob/main/README.md>  
- kro OWNERS: <https://github.com/kubernetes-sigs/kro/blob/main/OWNERS>  
- kro repo metadata API: <https://api.github.com/repos/kubernetes-sigs/kro>  
- Kubernetes SIG Cloud Provider README: <https://github.com/kubernetes/community/blob/master/sig-cloud-provider/README.md>  
- Kubernetes SIG Cloud Provider charter: <https://github.com/kubernetes/community/blob/master/sig-cloud-provider/CHARTER.md>  
- Kubernetes README: <https://github.com/kubernetes/kubernetes/blob/master/README.md>  
- Kubernetes governance: <https://github.com/kubernetes/community/blob/master/governance.md>  
- CNCF projects directory: <https://www.cncf.io/projects/>  
- CNCF project URL checks: <https://www.cncf.io/projects/cue/>, <https://www.cncf.io/projects/kro/>

