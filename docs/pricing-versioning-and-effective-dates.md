# Pricing Versioning and Effective Dates

Pricing changes are inevitable.

What matters is whether pricing changes are:

- explicit
- time-bound
- safe to apply

Valora treats pricing as **versioned configuration**, not mutable state.

---

## The Problem With Mutable Pricing

In many systems, pricing is updated in place:

- a price is edited
- new logic applies immediately
- historical invoices become ambiguous

This leads to:

- accidental re-rating
- unclear invoice provenance
- broken audits

---

## Valora’s Approach

Valora models pricing as:

- append-only versions
- each version has an effective time range
- no version overwrites another

A pricing version, once persisted, is never mutated.

---

## How Pricing Is Resolved

When computing billing for a period, Valora:

1. determines the billing period boundaries
2. selects the pricing version effective during that period
3. ignores versions outside that window
4. applies pricing rules deterministically

This resolution is deterministic and repeatable.

---

## Effective Dates Are First-Class

Effective dates are not metadata.
They are part of the pricing model.

Every pricing version must declare:

- when it becomes effective
- optionally, when it expires

This makes pricing intent explicit.

---

## Safe Pricing Changes

With versioned pricing:

- future prices can be prepared safely
- past invoices remain untouched
- pricing changes are reviewable before activation

Pricing changes become a controlled operation, not a runtime mutation.

---

## Why This Matters

Pricing versioning enables:

- safe experimentation
- historical correctness
- audit-friendly billing
- clear reasoning about revenue impact

---

## Summary

Valora treats pricing as immutable, versioned intent.

> **Pricing should change over time,
> but history should never change with it.**
>
