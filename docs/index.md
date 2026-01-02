# The Valora Billing Model

Valora is built on a simple premise:

> **Billing is a computation problem, not a payment problem.**

This documentation describes the mental model behind Valora’s billing engine—
the principles, boundaries, and design decisions that shape how billing is
computed, explained, and corrected.

---

## What Valora Optimizes For

Valora optimizes for:

- correctness over immediacy
- determinism over convenience
- explicit intent over inferred behavior
- long-term auditability over short-term flexibility

These priorities inform every design decision in the system.

---

## Core Concepts

Valora’s billing model is composed of a small set of explicit concepts.

Each concept is documented independently, but designed to work together.

---

### Billing vs Payments

Billing determines **what should be billed**.
Payments determine **how money moves**.

Valora owns billing.
Payments are intentionally out of scope.

→ `why-valora-does-not-handle-payments.md`

---

### Deterministic Computation

Billing results must be reproducible.

Given the same inputs, Valora will always produce the same outputs.
No result depends on runtime side effects or external systems.

→ `how-billing-stays-deterministic.md`

---

### Pricing Over Time

Pricing changes must not rewrite history.

Valora models pricing as versioned, append-only configuration
with explicit effective dates.

→ `pricing-versioning-and-effective-dates.md`

---

### Append-Only Billing Data

Billing data represents financial intent.

Once recorded, intent is never mutated.
Corrections are modeled explicitly.

→ `why-billing-is-append-only.md`

---

### Billing Cycles

Billing cycles define computation boundaries.

Each cycle owns:

- a fixed time window
- a stable set of inputs
- a deterministic output

→ `billing-cycle-as-a-first-class-concept.md`

---

### Late-Arriving Usage

Usage may arrive late.

Valora evaluates usage by event time,
and handles late data explicitly without mutating history.

→ `handling-late-arriving-usage.md`

---

### Idempotent Usage Ingestion

Usage ingestion must survive retries.

Valora enforces idempotency to ensure that
retries never affect billing correctness.

→ `idempotency-and-usage-ingestion.md`

---

### Non-Real-Time Billing

Billing is not computed in real time.

Usage is ingested continuously.
Billing is computed once the context is complete.

→ `why-billing-is-not-real-time.md`

---

### Explainability and Observability

Billing results must be explainable.

Every invoice can be traced back to:

- usage events
- pricing versions
- billing cycles
- configuration state

→ `observability-and-billing-explainability.md`

---

### Corrections and Adjustments

Mistakes are inevitable.
Silent mutation is not acceptable.

Valora models corrections as explicit adjustments
that preserve historical integrity.

→ `billing-corrections-and-adjustments.md`

---

## How to Read This Documentation

This documentation is not a tutorial.

It is a set of design decisions that define how Valora works.
Readers are encouraged to read documents in any order,
but understanding emerges from their combination.

---

## What This Model Enables

By enforcing these principles, Valora enables:

- predictable billing behavior
- safe pricing evolution
- reliable audits
- clear separation of concerns
- explainable financial outcomes

These properties are difficult to retrofit.
Valora makes them foundational.

---

## Closing

Valora’s billing model is intentionally conservative.

It prioritizes correctness, clarity, and trust over speed and novelty.

> **Billing should be boring, explicit, and explainable.
> Valora exists to make that possible.**
>
