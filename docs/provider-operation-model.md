# Provider operation model (HTTP-verb-agnostic)

This document defines the provider behavior model for resources whose lifecycle cannot be safely inferred from HTTP method names alone.

## Goals

- Support CRUD-like and action-style endpoints consistently.
- Keep provider adapters thin while preserving explicit lifecycle control.
- Enable safe reconciliation for APIs with long-running, asynchronous, or non-standard behaviors.

## Resource operation contract

Each resource provider should implement a contract shaped around behavior, not transport verbs:

- `Observe`: fetch current external state and normalize it.
- `PlanCreate`: decide whether and how create should run from desired + observed inputs.
- `Create`: execute create behavior.
- `PlanUpdate`: compute required changes when observed state differs.
- `Update`: execute update behavior.
- `Delete`: execute delete behavior.
- `InvokeAction(actionName, payload)`: execute non-CRUD state transitions.
- `GetOperationStatus(opId)`: poll long-running operation state when applicable.

## Reconciliation expectations

1. Observe current state.
2. Compare desired state against normalized observed state.
3. Decide action using explicit planning functions.
4. Execute operation or action.
5. Poll/track long-running operations until terminal status.
6. Re-observe and update status for next loop iteration.

## Idempotency and drift

- Providers should expose stable identity and diff semantics.
- Create/update/delete/action handlers should be safe for retries where possible.
- Drift must be represented as a first-class reconciliation input.

## Error classes

Classify errors with intent-aware handling:

- **Transient**: retry with backoff.
- **Terminal-validation**: fail reconciliation with actionable diagnostics.
- **Conflict/precondition**: re-observe and re-plan.
- **Async-in-progress**: continue polling.

Avoid broad catch-all logic that hides provider/API behavior.

## Why this model

Some APIs use action endpoints, eventually consistent workflows, or operation state machines that do not map directly to REST CRUD assumptions. This contract captures those behaviors explicitly while keeping providers thin and reusable.
