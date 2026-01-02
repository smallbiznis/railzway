# Idempotency and Usage Ingestion

Usage ingestion is the entry point of billing correctness.

If usage ingestion is not idempotent,
all downstream billing guarantees are compromised.

Valora treats idempotency as a **hard requirement**.

---

## The Problem With Non-Idempotent Ingestion

Without idempotency:

- retries create duplicate usage
- network failures cause overbilling
- clients cannot safely resend events
- billing results become untrustworthy

These failures are often silent and discovered too late.

---

## Valora’s Idempotency Model

Every usage event ingested by Valora must include:

- a stable idempotency key
- an organization or tenant scope
- a meter identifier
- an event timestamp

Idempotency is enforced **at persistence time**.

---

## What Idempotency Guarantees

With idempotent ingestion:

- retries are safe
- duplicate submissions are ignored
- billing aggregation remains correct
- clients do not need complex retry logic

The same event can be sent multiple times without changing billing results.

---

## What Valora Does Not Do

Valora does not:

- deduplicate heuristically
- infer duplicates by payload similarity
- rely on time windows to guess intent

Idempotency must be explicit.

---

## Interaction With Billing Cycles

Idempotent usage ingestion ensures that:

- billing cycles can be re-computed safely
- re-processing does not amplify usage
- deterministic aggregation remains intact

Idempotency is foundational to determinism.

---

## Failure Scenarios

If an ingestion request fails:

- clients may retry safely
- no manual reconciliation is required
- billing correctness is preserved

This shifts complexity away from clients.

---

## Summary

Valora enforces idempotent usage ingestion because:

- retries are inevitable
- correctness must survive failure
- billing systems must be resilient by default

> **If usage ingestion is not idempotent,
> billing correctness is an illusion.**
>
