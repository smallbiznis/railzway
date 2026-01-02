# Why Billing Data Is Append-Only

Billing systems manage financial intent.

Once intent is recorded, mutating it introduces ambiguity and risk.
For this reason, Valora treats billing data as **append-only**.

---

## What Append-Only Means

Append-only means:

- records are created, not updated
- corrections are modeled as new records
- history is preserved explicitly

Nothing is silently overwritten.

---

## The Problem With Mutable Billing State

Mutable billing systems often rely on:

- updating invoice rows
- correcting totals in place
- rewriting billing state

This makes it impossible to answer:

- what was originally intended?
- when did the change occur?
- why was it changed?

---

## Append-Only in Practice

In Valora:

- usage events are immutable
- pricing versions are immutable
- invoice line items are immutable
- billing results are derived, not edited

Corrections are represented as:

- new adjustments
- new billing cycles
- explicit corrective records

---

## Benefits of Append-Only Billing

Append-only billing enables:

### Auditability

Every change has a traceable origin.

### Reproducibility

Billing output can be re-derived from history.

### Safety

No silent corruption of financial records.

### Clear Causality

Changes explain themselves via new records.

---

## Storage and Performance Considerations

Append-only does not imply inefficiency.

Valora relies on:

- aggregation by period
- indexed historical data
- deterministic recomputation

Performance optimizations do not compromise correctness.

---

## Why Valora Enforces This

Billing systems exist to answer questions months or years later.

Append-only data ensures those questions remain answerable.

---

## Summary

Valora enforces append-only billing because:

- financial intent must be preserved
- history must remain intact
- correctness outweighs convenience

> **If billing data can be overwritten,
> it cannot be trusted.**
>
