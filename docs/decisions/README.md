# Architecture Decision Records (ADRs)

This directory captures significant architectural decisions for the experimental IaC tool.

## How we use ADRs

- Every decision that materially affects architecture, public API, or cross-cutting behavior gets one ADR.
- ADRs are immutable once `Accepted`. To change a decision, create a new ADR that `Supersedes` the previous one.
- Filename convention: `NNNN-short-kebab-title.md` (zero-padded sequential number).
- Reference relevant ADRs from `docs/traits-spec.md` and any code that implements them.

## Lifecycle / status values

- `Proposed` — drafted, under discussion.
- `Accepted` — agreed, in effect.
- `Rejected` — considered and declined; kept for history.
- `Superseded by NNNN` — replaced by a later ADR.
- `Deprecated` — no longer applies, but not formally replaced.

## Template

Use [`0000-template.md`](./0000-template.md) when creating a new ADR.

## Folder organization

ADRs are organized by lifecycle/status value:

- `accepted/`
- `proposed/`
- `superseded/`
- `rejected/`
- `deprecated/`

## Index

| # | Title | Status |
|---|---|---|
| [0001](./accepted/0001-execution-surface.md) | Execution surface (K8s-native vs standalone reconciler) | Accepted |
| [0002](./superseded/0002-authoring-language.md) | Authoring language / surface | Superseded by 0003 |
| [0003](./accepted/0003-kro-authoring-surface.md) | Authoring surface pivot to kro | Accepted |
| [0004](./accepted/0004-schema-ingestion.md) | Schema ingestion for kro-oriented authoring | Accepted |
| [0005](./proposed/0005-resource-identity-rules.md) | Resource identity and stability rules | Proposed |
| [0006](./proposed/0006-plan-status-projection.md) | Plan and status projection model | Proposed |
| [0007](./proposed/0007-transform-extension-model.md) | Transform and extension model | Proposed |

Open questions tracked in §5 of `artifacts/research/i-want-to-start-a-research-session-and-l.md` should each become an ADR as they are decided.
