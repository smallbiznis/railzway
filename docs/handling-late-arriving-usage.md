
# Handling Late-Arriving Usage

Late-arriving usage is inevitable in distributed systems.

Events can be delayed due to:

- network latency
- client retries
- batch uploads
- offline processing

Valora treats late-arriving usage as a **first-class problem**, not an edge case.

---

## The Problem With Implicit Handling

In many billing systems, late usage is handled implicitly:

- included if it arrives “soon enough”
- silently excluded after invoicing
- inconsistently re-rated on retries

This leads to:

- unclear billing boundaries
- inconsistent invoices
- customer disputes
- broken trust

---

## Valora’s Explicit Boundary

Valora defines a clear boundary:

> **A billing cycle owns a fixed usage window.**

Usage events are evaluated **based on event time**, not arrival time.

---

## Usage Cutoff Rules

For each billing cycle, Valora defines:

- a start timestamp
- an end timestamp
- an optional cutoff policy

Usage events are included **only if**:

- their event time falls within the cycle window
- they arrive before the cycle is finalized

Late-arriving usage after finalization is handled explicitly.

---

## Explicit Handling Strategies

Valora does not silently mutate closed billing cycles.

Instead, late usage can be handled by:

- carrying forward into the next billing cycle
- generating explicit adjustment records
- triggering a corrective billing flow

The chosen strategy is **configured**, not inferred.

---

## Why Arrival Time Is Not Trusted

Arrival time is unreliable because it depends on:

- client behavior
- network conditions
- retry policies
- infrastructure load

Billing decisions must not depend on these variables.

---

## Auditability and Explainability

With explicit handling:

- every usage event has a clear billing outcome
- exclusions are explainable
- customers can be shown why usage was billed when it was

No usage event is silently ignored.

---

## Summary

Valora handles late-arriving usage by:

- defining explicit cycle boundaries
- evaluating usage by event time
- refusing to mutate closed billing cycles

> **Late usage must be handled deliberately,
> not hidden inside billing retries.**
>
