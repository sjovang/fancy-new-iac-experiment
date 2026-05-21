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

---

## Round 2 — "Bring Order to Chaos"

**Theme:** Words meaning to bring order, impose structure, tame disorder, or make the unpredictable predictable — the cosmological counterpart to reconciliation.

| # | Name | Language | Pronunciation | Core Meaning | Notes | Fit |
|---|------|----------|---------------|--------------|-------|-----|
| 1 | **niyama** | Sanskrit नियम | `ni-YAH-muh` | Rule, regulation, self-discipline | Yoga's second limb (ethical observances); doublet of "nomos." No tech conflicts. 6 chars. | ⭐⭐⭐⭐⭐ |
| 2 | **uhlelo** | Zulu | `oo-HLEH-lo` | Arrangement, order, system; also the standard Zulu word for "software" | The software double-meaning is serendipitous. Wiktionary-verified. No conflicts. 6 chars. | ⭐⭐⭐⭐⭐ |
| 3 | **kittu** | Akkadian (Babylonian) | `KIT-too` | Cosmic truth, cosmic justice, right order | The ordering principle of the cosmos in Babylonian theology, paired with *mīšaru* (justice). 5 chars. No conflicts. | ⭐⭐⭐⭐⭐ |
| 4 | **trefn** | Welsh | `TREV-n` | Order, sequence, routine, law and order | Wiktionary-verified. From Proto-Celtic *trebā*. 5 chars. No conflicts anywhere. | ⭐⭐⭐⭐⭐ |
| 5 | **nizam** | Classical Arabic نِظَام | `nee-ZAHM` | Order, arrangement, system, regime | Wiktionary-verified. 5 chars. Historical gravitas (Nizam of Hyderabad). No tech conflicts. | ⭐⭐⭐⭐ |
| 6 | **tsari** | Hausa | `TSAH-ree` | Arrangement, plan, system, order | Wiktionary-verified. 5 chars. Clean romanization. No conflicts. | ⭐⭐⭐⭐ |
| 7 | **skipan** | Faroese / Old Norse | `SKEE-pan` | Arrangement, system, order, rule | Wiktionary-verified. 6 chars. "Skip" association reads as skipping chaotic state — manageable. No tech conflicts. | ⭐⭐⭐⭐ |
| 8 | **olungu** | Tamil ஒழுங்கு | `oh-LUNG-goo` | Order, discipline, arrangement, regularity | Core Tamil word with no loanword ambiguity. 6 chars. No tech conflicts. | ⭐⭐⭐⭐ |
| 9 | **krama** | Sanskrit / Old Javanese क्रम | `KRAH-muh` | Course, order, system, conduct, custom, law | Wide cross-cultural reach across South and Southeast Asia. ⚠️ Serbo-Croatian *krama* = "junk/trash" — minor flag for Balkan markets. | ⭐⭐⭐⭐ |
| 10 | **juram** | Mongolian журам | `ZHOO-rahm` | Rule, regulation, protocol, order | Standard Mongolian legal/administrative term. 5 chars. No tech conflicts. | ⭐⭐⭐⭐ |
| 11 | **rigi** | Georgian რიგი | `REE-gee` | Row, order, sequence, turn | 4 chars — short and distinctively non-English. Rigi mountain (Switzerland) is a tourist site, not a tech brand. No tech conflicts. | ⭐⭐⭐⭐ |
| 12 | **riap** | Khmer រៀប | `RYAP` | To arrange, put in order, organize | Root of *រៀបរយ* (riap-roy, "orderly"). 4 chars. Unusual vowel cluster makes it memorable. No conflicts. | ⭐⭐⭐⭐ |
| 13 | **kamay** | Quechua | `KAH-my` | To create, animate, give order to; to breathe life into cosmos | From *kamaq* (the Andean cosmological creator-organizer). 5 chars. ⚠️ Kamay is also the Aboriginal name for Botany Bay (Australia) — no tech conflict but worth awareness. | ⭐⭐⭐⭐ |
| 14 | **melahua** | Classical Nahuatl | `meh-LAH-wah` | To go straight, straighten out, set things right, correct | From root *-melaoa* (to make straight / tell truth directly). 7 chars. Highly distinctive. No conflicts. | ⭐⭐⭐⭐ |
| 15 | **atunse** | Yoruba àtúnṣe | `ah-TOON-sheh` | Rearrangement, correction, rectification, reform, amendment | From *tún* (redo) + *ṣe* (to do). "Constitutional amendment" = *Àtúnṣe Ìgbìmọ̀*. 6 chars. No tech conflicts. | ⭐⭐⭐⭐ |
| 16 | **ayos** | Tagalog | `AH-yos` | Order, arrangement, fix, orderly condition | Proto-Austronesian root; used as both verb (to fix/arrange) and noun. 4 chars. No tech conflicts. | ⭐⭐⭐⭐ |
| 17 | **asha** | Avestan 𐬀𐬴𐬀 | `AH-shah` | Cosmic truth, cosmic order, righteousness — the ordering principle of the universe | Wiktionary-verified. Doublet of Sanskrit *ṛta*. 4 chars. ⚠️ Common personal name across multiple cultures — reduces brand distinctiveness, though no tech conflicts. | ⭐⭐⭐⭐ |
| 18 | **antola** | Basque | `an-TOH-lah` | To organize, arrange, put in order | Pre-Indo-European language isolate. Verb stem of *antolatu* (organized). 6 chars. No conflicts. | ⭐⭐⭐ |
| 19 | **taratibu** | Swahili (← Arabic *tartīb*) | `ta-RA-tee-boo` | Pattern, method, organisation, step-by-step order | Wiktionary-verified. 8 chars. Informally also means "carefully, in order, slowly and properly" — a strong operational metaphor. | ⭐⭐⭐ |
| 20 | **faatere** | Tahitian (Reo Tahiti) | `fah-TEH-reh` | To administer, pilot, navigate, steer, govern | From Polynesian causative *faa-* + *tere* (to navigate/flow). "Steering toward order." ⚠️ May carry glottal stop (*fa'atere*) in formal spelling. 7 chars. | ⭐⭐⭐ |
| 21 | **pulega** | Samoan | `poo-LEH-gah` | Administration, management, governance | Standard Samoan word for governance/management. 6 chars. Semantic fit is indirect — more "administration" than "taming chaos." No conflicts. | ⭐⭐⭐ |
| 22 | **karg** | Armenian կarG | `KARG` | Order, rank, arrangement, class, sequence | 4 chars. ⚠️ German *karg* = "barren/scarce/meager" — phonetic overlap only, no etymological relation, but worth noting for European markets. | ⭐⭐⭐ |
| 23 | **usoro** | Igbo | `oo-SOH-roh` | System, arrangement, method, manner, way | 5 chars. Standard Igbo. ⚠️ Wiktionary citation is weak; meaning attested in standard Igbo dictionaries but worth independent verification. | ⭐⭐⭐ |
| 24 | **reaghey** | Manx Gaelic | `RAY-ee` | To arrange, settle, deal with, manage, resolve | Cognate of Irish *réitigh* and Scottish Gaelic *rèitich*. Living Celtic language (Isle of Man). 7 chars. Unusual spelling. No conflicts. | ⭐⭐⭐ |
| 25 | **siraat** | Amharic ሥርዓት | `si-RAAT` | Order, system, rule, regulation | 6 chars. Standard Amharic. ⚠️ Phonetic overlap with Arabic *ṣirāṭ* (the straight path in Islamic eschatology) — sensitive homophone in MENA markets. Could romanize as *sir'at* to differentiate. | ⭐⭐⭐ |

### Namespace Check

No conflicts found with Apache projects, CNCF landscape, HashiCorp tools, or common Linux CLI utilities for any of the 25 candidates.

### Flagged and excluded from table

| Word | Language | Reason |
|------|----------|--------|
| **maat** | Ancient Egyptian | High cultural appropriation sensitivity |
| **kosmos / cosmos** | Ancient Greek | Direct conflict with Cosmos blockchain |
| **taxis** | Ancient Greek | Conflicts with "taxi" (consumer connotation) and biology term |
| **ordne / ordna** | Danish/Swedish | Danish vulgar slang |
| **riaghailt** | Scottish Gaelic | Pronunciation too opaque for English speakers |

### Top picks from this round

1. **niyama** — "self-discipline/rule" from Sanskrit; short, clean, no conflicts, and resonates with the control-loop concept
2. **uhlelo** — Zulu for both "system/order" *and* "software"; the dual meaning is uniquely apt
3. **trefn** — Welsh for "order/sequence"; extremely short, no issues, strong Celtic family continuity with *cymodi*/*cysoni*

---

*Round 2 research conducted May 2026. Sources: Wiktionary (trefn, nizam, tsari, skipan, uhlelo, krama, taratibu, asha), Glosbe multilingual corpus, CNCF landscape, Apache project catalog.*
