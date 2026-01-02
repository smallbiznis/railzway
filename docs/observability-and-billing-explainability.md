# Observability and Billing Explainability

Billing systems are trusted only when their outputs can be explained.

In Valora, observability is not limited to metrics and logs.
It includes the ability to explain **why a specific billing result exists**.

---

## The Problem With Opaque Billing

Many billing systems produce outputs that are difficult to explain:

- invoices appear without clear derivation
- totals cannot be traced back to inputs
- support relies on screenshots and assumptions
- audits require manual reconstruction

This opacity erodes trust.

---

## Valora’s Definition of Explainability

A billing result is explainable if Valora can answer:

- which usage events contributed to it
- which pricing version was applied
- which billing cycle produced it
- which configuration was in effect
- when and why it was finalized

Explainability is treated as a **product requirement**, not a support feature.

---

## Observability Layers in Valora

Valora provides observability at multiple layers:

### Input-Level

- persisted usage events
- idempotency keys
- event timestamps

### Configuration-Level

- pricing versions
- effective dates
- subscription state transitions

### Computation-Level

- billing cycle boundaries
- aggregation steps
- rating results

### Output-Level

- invoice line items
- billing totals
- derived states

Each layer is independently inspectable.

---

## Determinism Enables Observability

Because billing computation is deterministic:

- results can be re-derived
- explanations do not depend on logs or timing
- observability does not degrade over time

Historical billing remains explainable months or years later.

---

## Observability Without Mutation

Valora does not rely on mutable debug state.

Instead:

- explanations are derived from persisted facts
- no special “debug mode” is required
- recomputation yields the same explanation

This avoids discrepancies between “debug” and “production” views.

---

## Operational Visibility

Valora supports operational observability through:

- explicit billing cycle states
- computation progress indicators
- failure reasons that map to domain concepts

Operational failures are visible without inspecting internal logs.

---

## Why Explainability Matters

Explainable billing enables:

- faster issue resolution
- reduced customer disputes
- reliable audits
- confident pricing changes

Without explainability, billing systems become a support liability.

---

## Summary

Valora treats observability as the ability to explain billing outcomes.

> **If a billing result cannot be explained clearly,
> it should not exist.**
>
