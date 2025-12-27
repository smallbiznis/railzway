# Threat Model (Valora OSS)

## Purpose

This document provides a **lightweight threat model** for Valora OSS.

It is intended to:

- Identify key trust boundaries
- Highlight realistic threat scenarios
- Clarify which threats are mitigated by Valora OSS and which are delegated to the adopting system

This is **not** a formal compliance document.

---

## System Overview

Valora OSS is a self-hosted billing engine responsible for:

- Usage ingestion
- Pricing and rating
- Subscription lifecycle management
- Invoice generation

Valora OSS does **not** process payments or store payment credentials.

---

## Assets

Primary assets protected by Valora OSS:

- Billing correctness and determinism
- Usage records and aggregation results
- Pricing and subscription configuration
- Invoice state and metadata
- Tenant (organization) data isolation

---

## Trust Boundaries

Valora OSS operates across the following trust boundaries:

1. **Client → Valora API**

   - Authenticated API requests
   - Usage ingestion and billing configuration
2. **Valora → Database**

   - Persistent storage of billing data
3. **Valora → External Systems**

   - Payment providers (via references only)
   - Identity providers (upstream authentication)

Valora OSS assumes that network security and infrastructure isolation are provided by the deploying environment.

---

## Threat Categories (High-Level)

Threats are grouped using a simplified STRIDE-style model.

---

## Spoofing

### Threat

An attacker attempts to impersonate a valid user or organization.

### Mitigation

- Authentication enforced at API boundaries
- Organization-scoped authorization checks
- No implicit trust of client-provided organization identifiers

### Residual Risk

Credential compromise at the identity provider level.

---

## Tampering

### Threat

Malicious modification of:

- Usage records
- Pricing configuration
- Subscription state

### Mitigation

- Server-side validation of all billing inputs
- Deterministic billing computation
- Explicit state transitions for subscriptions and invoices

### Residual Risk

Database-level tampering by privileged infrastructure operators.

---

## Repudiation

### Threat

A user disputes a billing outcome or claims an action did not occur.

### Mitigation

- Immutable usage records once finalized
- Invoice state transitions are explicit and traceable
- Timestamps and identifiers recorded for billing operations

### Residual Risk

Lack of external audit logs if not configured by the adopting system.

---

## Information Disclosure

### Threat

Unauthorized access to billing or tenant data.

### Mitigation

- Logical tenant isolation at the application level
- Authorization enforced on all read and write paths
- No storage of sensitive payment credentials

### Residual Risk

Misconfigured access controls or database exposure by the deploying environment.

---

## Denial of Service

### Threat

Excessive usage ingestion or API requests degrade system availability.

### Mitigation

- Expectation of upstream rate limiting
- Stateless API design where possible
- Idempotent operations for critical workflows

### Residual Risk

Resource exhaustion if rate limits are not enforced externally.

---

## Elevation of Privilege

### Threat

A user gains access to operations outside their intended role or tenant.

### Mitigation

- Organization-scoped authorization checks
- Explicit permission checks for administrative operations
- No reliance on client-provided trust assertions

### Residual Risk

Authorization misconfiguration by the adopting application.

---

## Non-Goals

Valora OSS does **not** attempt to mitigate:

- Infrastructure-level attacks (network, kernel, container escape)
- Physical security threats
- Payment fraud or card data compromise
- Identity provider compromise

These are explicitly delegated to the adopting system.

---

## Security Assumptions

Valora OSS assumes:

- TLS is enforced by the deployment environment
- Secrets are managed securely by the operator
- Databases are not publicly accessible
- External identity providers are correctly configured

---

## Summary

Valora OSS focuses on **billing correctness and logical isolation**, while deliberately minimizing its security scope.

By keeping security boundaries explicit and delegating high-risk domains to specialized systems, Valora OSS reduces complexity and limits its attack surface.

---

## Disclaimer

This threat model is provided for informational purposes only.

Security outcomes depend on deployment architecture, operational practices, and integration choices made by the adopting system.
