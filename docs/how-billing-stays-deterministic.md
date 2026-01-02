# How Billing Stays Deterministic in Valora

Determinism is a core property of Valora.

In Valora, billing results are not inferred from runtime behavior,
side effects, or external systems.
They are **computed** from persisted inputs and configuration.

Given the same inputs, Valora will always produce the same billing output.

---

## What Deterministic Billing Means

In Valora, deterministic billing means:

- billing results are reproducible
- historical invoices can be re-derived
- pricing changes do not retroactively mutate past outcomes
- billing logic can be reasoned about statically

Determinism is treated as a **correctness requirement**, not an optimization.

---

## Inputs That Define Billing Output

Billing output in Valora is derived solely from:

- persisted usage events
- persisted pricing configuration
- explicit effective dates
- billing period boundaries
- subscription state transitions

No runtime-only state is used to determine billing results.

---

## What Is Explicitly Excluded

Valora does not derive billing results from:

- payment success or failure
- webhook delivery order
- retry attempts
- background job timing
- external API responses

These signals are inherently non-deterministic.

---

## Deterministic Aggregation

Usage aggregation in Valora is:

- scoped to a billing period
- derived from immutable usage records
- idempotent by design

Re-processing the same period yields the same totals.

---

## Pricing Versioning and Time

Pricing configuration is versioned and time-bound.

At billing time, Valora resolves pricing by:

- selecting the pricing version effective for the billing period
- ignoring future or superseded versions
- never mutating historical configuration

This prevents accidental re-rating of past invoices.

---

## Why Determinism Matters

Deterministic billing enables:

- safe re-computation
- explainable invoices
- reliable audits
- predictable financial behavior

Without determinism, billing systems become dependent on history that
cannot be reconstructed.

---

## Summary

Valora keeps billing deterministic by:

- persisting all billing-relevant inputs
- separating computation from execution
- refusing to infer intent from side effects

> **Billing that cannot be re-derived cannot be trusted.**
>
