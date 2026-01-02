
# Why Valora Does Not Handle Payments

Valora intentionally does **not** handle payment execution.

This is not a missing feature.
It is a deliberate architectural decision.

Valora determines **what should be billed**.
It does not determine **how money moves**.

---

## The Problem With Coupling Billing and Payments

In many systems, billing logic and payment execution are tightly coupled:

- pricing rules live inside payment flows
- billing state is inferred from payment results
- changes to pricing require changes to payment logic
- audits require reconstructing logic from side effects

This coupling creates several problems:

- billing behavior becomes implicit and hard to reason about
- historical correctness depends on mutable payment logic
- re-rating or price changes risk corrupting past invoices
- payment failures obscure billing intent

In short:

> **Money movement becomes the source of truth.**

This is fragile.

---

## Valora’s Design Boundary

Valora draws a strict boundary:

- **Billing** answers: *what should be billed, when, and why*
- **Payments** answer: *how money is collected or transferred*

Valora owns the first.
It explicitly excludes the second.

This boundary allows Valora to remain:

- deterministic
- auditable
- explainable
- payment-provider agnostic

---

## What Valora Produces

Valora produces **billing facts**, not financial side effects.

Examples:

- invoice line items
- billing states
- rated usage totals
- proration results
- billing cycle outcomes

These outputs are:

- derived solely from persisted inputs
- repeatable given the same configuration
- independent of payment success or failure

---

## What Valora Does Not Do

Valora does **not**:

- charge credit cards
- store payment methods
- retry failed payments
- manage settlements or payouts
- reconcile bank statements

These concerns belong to payment providers and financial systems.

---

## Why This Separation Matters

Separating billing from payments enables:

### Deterministic Billing

Billing results do not change because:

- a payment was retried
- a provider changed behavior
- a webhook arrived late

### Safe Pricing Changes

Pricing logic can evolve without:

- rewriting payment flows
- corrupting historical invoices
- breaking auditability

### Provider Flexibility

Teams can:

- integrate Stripe, Adyen, Midtrans, or others
- change providers without rewriting billing logic
- support multiple providers in parallel

### Clear Responsibility Boundaries

Failures are easier to reason about:

- billing bugs are billing bugs
- payment failures are payment failures

---

## Integration Model

Valora is designed to sit **upstream** of payments.

A typical flow:

1. Application sends usage events to Valora
2. Valora computes billing state and invoices
3. Application passes invoice data to a payment provider
4. Payment provider executes collection
5. Payment results are optionally reflected back as metadata

Valora remains the system of record for **billing intent**.

---

## Non-Goals

Valora intentionally does not aim to become:

- a payment orchestration layer
- a merchant of record
- a financial ledger or accounting system
- a compliance abstraction over payment regulations

These domains have different constraints and responsibilities.

---

## Summary

Valora does not handle payments because:

- billing logic must be deterministic
- money movement is inherently side-effectful
- coupling the two creates fragile systems
- clear boundaries improve long-term correctness

> **Valora keeps billing boring, explicit, and predictable
> by refusing to own payment execution.**
>
