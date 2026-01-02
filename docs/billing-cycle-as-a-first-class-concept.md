# Billing Cycle as a First-Class Concept

In Valora, the billing cycle is not a background job.
It is a **first-class domain concept**.

A billing cycle defines **when billing is computed, what data is included,
and which results are considered final**.

---

## The Problem With Implicit Billing Cycles

In many systems, billing cycles are implicit:

- billing runs on a cron schedule
- invoices are generated when a job executes
- period boundaries are inferred from timestamps
- re-runs depend on runtime behavior

This leads to ambiguity:

- it is unclear which data belongs to which cycle
- retries may produce different results
- partial failures are hard to reason about
- historical billing cannot be reconstructed reliably

---

## Valora’s Model

Valora models a billing cycle as an explicit entity.

A billing cycle represents:

- a defined time window
- a fixed set of inputs
- a deterministic computation boundary

Once a billing cycle is closed, its outcome is final.

---

## What a Billing Cycle Owns

A billing cycle explicitly owns:

- start and end timestamps
- the usage events included in the period
- the pricing versions resolved for the period
- the subscription state during the period
- the resulting billing outputs

All billing results are derived **within** the context of a cycle.

---

## Cycle State Transitions

Billing cycles transition through explicit states, such as:

- `open`
- `computing`
- `closed`
- `corrected` (if applicable)

State transitions are recorded and auditable.
There are no implicit or hidden transitions.

---

## Deterministic Computation Boundary

When a billing cycle is evaluated:

- only data within the cycle window is considered
- late-arriving usage is excluded or handled explicitly
- pricing versions are resolved once
- results are persisted as derived outputs

Re-running the same cycle yields the same results.

---

## Corrections and Re-Billing

Corrections do not mutate a closed cycle.

Instead, Valora models corrections as:

- new billing cycles
- adjustment records
- explicit corrective flows

This preserves the integrity of historical billing.

---

## Why Billing Cycles Must Be First-Class

Treating billing cycles as first-class enables:

- clear reasoning about billing periods
- deterministic invoice generation
- safe retries and recomputation
- auditable financial history

Without explicit cycles, billing becomes a side effect of scheduling.

---

## Interaction With Other Concepts

Billing cycles interact explicitly with:

- usage ingestion (cutoff boundaries)
- pricing versioning (effective dates)
- invoicing (finalization triggers)
- append-only billing data

The cycle acts as the coordination point for all billing logic.

---

## Summary

Valora treats billing cycles as first-class because:

- billing must be bounded in time
- computation must be repeatable
- results must be final and explainable

> **If a billing system cannot clearly answer
> “which cycle produced this invoice,”
> it is not a reliable billing system.**
>
