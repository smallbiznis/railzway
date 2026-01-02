# Why Billing Is Not Real-Time

Valora does not attempt to compute billing in real time.

This is a deliberate trade-off.

Billing correctness is prioritized over immediacy.

---

## The Illusion of Real-Time Billing

“Real-time billing” often means:

- updating totals on every event
- mutating billing state continuously
- reacting to incomplete data

In practice, this leads to:

- race conditions
- inconsistent totals
- difficult reconciliation
- non-reproducible results

---

## Billing Requires Complete Context

Accurate billing depends on:

- complete usage data
- resolved pricing versions
- known billing period boundaries
- stable subscription state

These conditions are rarely satisfied at event time.

---

## Valora’s Model

Valora separates:

- **usage ingestion** (near real-time)
- **billing computation** (periodic and bounded)

Usage events are recorded immediately.
Billing results are computed **after the period is known**.

---

## Determinism Over Immediacy

By avoiding real-time billing:

- billing results are deterministic
- retries do not change outcomes
- historical invoices can be re-derived

Latency is traded for correctness.

---

## Observability Without Mutation

Valora does not mutate billing state in real time.

Instead, systems may:

- observe usage accumulation
- preview estimated totals
- compute projections outside billing finalization

Final billing results remain stable.

---

## Why This Matters

Real-time billing systems often fail when:

- pricing changes mid-period
- usage arrives late
- retries overlap with aggregation

Valora avoids these failure modes by design.

---

## Summary

Valora avoids real-time billing because:

- billing needs complete information
- correctness must be preserved
- determinism outweighs immediacy

> **Billing should be timely,
> but it should never be speculative.**
>
