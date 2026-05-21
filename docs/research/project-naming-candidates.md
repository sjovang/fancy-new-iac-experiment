# Project Naming Candidates

**Status:** Under consideration — no name selected yet
**Context:** The project requires a name that evokes reconciliation: continuously observing desired vs. actual state and making corrections. Previous candidates were rejected:

- **Hoopono** (Hawaiian-inspired) — too close to *Ho'oponopono*, a Hawaiian spiritual practice; felt culturally appropriative.
- **Fakalelei** (Tongan: "to fix, reconcile, make amends") — proximity to *fakalele*, a Tongan term with negative connotations.

---

## Design Principles the Name Should Reflect

- Reconciliation-first (continuous control loop)
- Dependency-DAG planning
- Cloud-agnostic
- Declarative intent
- Explicit lifecycle management

---

## Candidates

| # | Name | Language | Pronunciation | Core Meaning | Notes | Fit |
|---|------|----------|---------------|--------------|-------|-----|
| 1 | **cysoni** | Welsh | `kuh-SOH-nee` | To make consistent, harmonize | From `cyson` ("consistent, regular, steady") + verbal suffix `-i`. "To make [actual state] consistent [with desired state]" is near-literal description of a reconciler. | ⭐⭐⭐⭐⭐ |
| 2 | **cymodi** | Welsh | `kuh-MOH-dee` | To reconcile, come to agreement | From `cyd-` (together) + root for "mode/manner". Direct translation of "reconcile". | ⭐⭐⭐⭐⭐ |
| 3 | **réitigh** | Irish Gaelic | `RAY-tee` | To smooth the way, clear, unravel, settle | From Old Irish `réitigid`. Rich meaning: "to smooth the path, clear obstacles, unravel knots, arrange in order, solve, settle disputes." The accent on `é` is optional in CLI contexts. | ⭐⭐⭐⭐⭐ |
| 4 | **patanisha** | Swahili | `pah-tah-NEE-shah` | To reconcile, regulate, make fit/compatible | Causative of `patana` ("to match, fit together, agree"). Literally "to cause desired and actual state to fit together." | ⭐⭐⭐⭐⭐ |
| 5 | **sovinto** | Finnish | `SOH-vin-toh` | Reconciliation, settlement, peace | From `sopia` ("to agree, to fit, to be compatible"). Names the outcome the tool produces. | ⭐⭐⭐⭐ |
| 6 | **sætta** | Old Norse | `SAYT-ah` | To reconcile, make peace among, bring to terms | Attested in Njáls saga, Laxdæla saga. `æ` diacritic is a CLI obstacle; transliterates as `saetta` (Italian: "lightning" — unrelated, benign). | ⭐⭐⭐⭐ |
| 7 | **rétta** | Old Norse | `REHT-ah` | To make right, make straight, adjust, redress | From Proto-Germanic `*raihtijaną`. Transliterates cleanly as `retta` (Italian: "straight/upright" — on-theme). | ⭐⭐⭐⭐ |
| 8 | **usawa** | Swahili | `oo-SAH-wah` | Balance, equilibrium, parity | From `u-` + `-sawa` ("equal, even, level"). Names the steady-state the tool seeks. Short and clean. | ⭐⭐⭐⭐ |
| 9 | **samræma** | Icelandic | `SAM-rye-mah` | To harmonize, reconcile, bring into accord | `sam-` (together) + `ræma` (to harmonize/tune). `æ` diacritic is the main drawback. | ⭐⭐⭐ |
| 10 | **allipanakuy** | Quechua | `ah-lee-pah-NAH-koo-ee` | To mutually make right/compatible | `alli` ("good/right") + reciprocal suffix. Semantically precise but 11 characters is long for a CLI tool. | ⭐⭐⭐ |

---

## Rejected / Flagged

| Name | Language | Reason |
|------|----------|--------|
| **whakatika** | Māori | `whaka-` prefix is pronounced `fah-kah` — same phoneme issue as Fakalelei |
| **tika** | Māori / pan-Polynesian | Conflicts with **Apache Tika**, a major Apache Software Foundation toolkit (tika.apache.org) |

---

## Top Recommendations

1. **cysoni** — "to make consistent" is almost a technical specification of a reconciliation engine
2. **réitigh** — semantically rich: smoothing the path, disentangling, arranging in order
3. **patanisha** — the causative structure maps cleanly onto "cause desired and actual state to fit together"

---

*Research conducted May 2026. Sources: Te Aka Māori Dictionary (maoridictionary.co.nz), Foclóir Gaeilge-Béarla (teanglann.ie), Cleasby-Vigfusson Old Norse Dictionary, Glosbe multilingual corpus, Apache Tika (tika.apache.org).*
