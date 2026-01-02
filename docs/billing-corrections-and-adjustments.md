# Billing Corrections and Adjustments

Billing systems must support corrections.

What matters is **how corrections are modeled**.

Valora treats corrections as explicit domain events,
not as mutations of historical data.

---

## The Problem With In-Place Corrections

In many systems, corrections are applied by:

- editing invoices
- overwriting totals
- rerunning billing jobs

This creates ambiguity:

- original intent is lost
- history becomes unclear
- audits are compromised

---

## Valora’s Correction Model

Valora never mutates finalized billing results.

Instead, corrections are represented as:

- new billing cycles
- explicit adjustment records
- additive or compensating entries

The original billing outcome remains intact.

---

## Types of Corrections

Valora supports corrections such as:

- late-arriving usage adjustments
- pricing misconfiguration corrections
- subscription state corrections
- manual adjustments with explicit reasons

Each correction is modeled explicitly.

---

## Adjustment Records

An adjustment record includes:

- the reason for the correction
- the affected billing period
- the delta applied
- a reference to the original billing result

Adjustments are first-class entities, not annotations.

---

## Corrective Billing Flows

Corrections follow explicit flows:

1. original billing cycle remains closed
2. a corrective cycle or adjustment is created
3. the delta is applied transparently
4. invoices reflect both original and corrective entries

This preserves a complete billing history.

---

## Audit and Traceability

With explicit corrections:

- auditors can see what changed and why
- original and corrected values coexist
- billing intent is preserved

Nothing is silently rewritten.

---

## Why Valora Enforces This

Corrections are inevitable.
Silent mutation is not acceptable.

By enforcing explicit corrections, Valora ensures:

- historical integrity
- clear accountability
- predictable financial behavior

---

## Summary

Valora handles billing corrections by **adding context, not erasing history**.

> **Correct billing systems do not hide mistakes.
> They record and explain them.**
>
