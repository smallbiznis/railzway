# **ADR-001: Billing Truth & Source of Record**

**Status:** Accepted

**Date:** 2026-01-02

**Scope:** Billing architecture, usage lifecycle, financial correctness

---

## Context

Valora processes usage-based billing through multiple stages:

* Usage ingestion (`usage_events`)
* Context resolution / snapshotting (meter, subscription, item)
* Rating and aggregation per billing cycle
* Invoice and ledger generation (future)

Early in the design, multiple data models exist that *could* be interpreted as “billing data,” particularly:

* `usage_events`
* `usage_events.status`
* `rating_results`

Without a clear architectural decision, there is a risk that financial correctness becomes ambiguous or duplicated across layers.

---

## Problem

We need a **single, unambiguous source of billing truth** that determines:

* what is billable
* how much is billable
* which usage contributes to invoices and ledger entries

At the same time, the system must:

* accept usage before subscriptions or pricing exist
* support late-arriving usage
* allow re-rating and corrections
* avoid per-event financial mutation

---

## Decision

**`rating_results` is the sole source of billing truth in Valora.**

* All financial correctness (quantities, prices, amounts) **lives exclusively in `rating_results`.**
* `usage_events` is **not** a billing record.
* `usage_events.status` represents  **data-processing lifecycle state** , not financial state.

No invoice, ledger entry, or monetary decision may be derived directly from `usage_events`.

---

## Definitions

### Billing Truth

Billing truth refers to data that:

* directly determines money
* is used to generate invoices or ledger entries
* must be correct, atomic, and auditable

In Valora, billing truth is represented by:

* `rating_results`
* (and later: invoice lines, ledger entries)

### Source of Record

The **source of record** for billing is the dataset that downstream systems must trust  **without reinterpretation** .

In Valora:

* `rating_results` is the billing source of record
* all other data models are upstream inputs or pipeline artifacts

---

## Rationale

### Why not `usage_events`?

* Usage events are **raw operational data**
* They arrive independently of billing cycles
* They may arrive before subscriptions or pricing exist
* They may be reprocessed, corrected, or ignored later

Mutating usage events to reflect financial outcomes would:

* couple ingestion with pricing logic
* introduce race conditions
* risk “usage marked billable without billing results”

### Why `rating_results`?

* Rating is window-based and transactional
* Pricing is resolved at rating time
* Aggregation, currency, and amount are finalized here
* Rating results can be replayed, audited, or regenerated

This mirrors Stripe-like billing semantics, where usage recording is decoupled from billing finalization.

---

## Consequences

### Positive

* Clear separation of concerns:
  * ingestion ≠ billing
  * snapshot ≠ pricing
  * rating = financial truth
* Safe handling of late-arriving usage
* Idempotent and auditable billing
* Enables re-rating without mutating raw usage

### Trade-offs

* Event-level status such as `rated` is informational, not authoritative
* Billing state is not visible on individual usage events
* Requires joining or referencing `rating_results` for financial insight

These trade-offs are accepted to preserve billing correctness.

---

## Relationship to Usage Status (`usage_events.status`)

`usage_events.status` tracks  **pipeline progression only** :

| Status            | Meaning                                        |
| ----------------- | ---------------------------------------------- |
| `accepted`      | Event received and persisted                   |
| `enriched`      | Snapshot context resolved                      |
| `unmatched_*`   | Context resolution failed                      |
| `rated`(future) | Event participated in a completed rating cycle |

Even when introduced, `rated`  **does not become billing truth** .

It is a derived, post-rating marker for observability or auditing only.

---

## Future Considerations

* A post-rating step may mark usage events as `rated` **after** successful rating commits.
* This step must be idempotent and non-authoritative.
* Existing systems must continue to rely on `rating_results` for billing.

No future change may shift billing truth away from `rating_results`.

---

## Summary (One-line)

> Valora intentionally separates data-processing state from financial truth: usage events describe  *what happened* , while rating results define  *what is billed* .
>
