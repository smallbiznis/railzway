# Security Policy

## Overview

Valora is a billing engine designed to manage **billing logic**, including usage metering, pricing, rating, subscriptions, and invoice generation.

Valora **is not a payment processor** and **does not handle payment instruments**.

This document describes the security scope, assumptions, and reporting process for Valora.

---

## Security Scope

### In Scope

Valora is responsible for:

- Correct and deterministic billing computation
- Usage ingestion and aggregation
- Pricing and rating logic
- Subscription lifecycle management
- Invoice generation and state transitions
- Authentication and authorization for Valora APIs
- Tenant (organization) data isolation at the application level

---

### Explicitly Out of Scope

Valora **does NOT**:

- Store, transmit, or process credit card data
- Handle payment execution or settlement
- Act as a payment gateway or merchant of record
- Implement cryptographic primitives
- Manage customer payment credentials

Payment execution is delegated to **external payment providers** (e.g. Stripe, Midtrans, Xendit) integrated by the adopting application.

As a result, Valora **does not fall under PCI-DSS scope** by design.

---

## Data Handling

- Valora stores billing-related data such as:
  - usage records
  - pricing configuration
  - subscription state
  - invoice metadata
- Sensitive payment data (PAN, CVV, bank details) must never be sent to Valora APIs.
- Identifiers and tokens (e.g. customer IDs, external payment references) are treated as opaque values.

---

## Authentication & Authorization

- API authentication is enforced at the service boundary.
- Authorization decisions are scoped to organizations (tenants).
- Valora assumes that upstream identity providers are responsible for user authentication where applicable.

---

## Dependency Security

Valora relies on widely adopted Go libraries and infrastructure components, including:

- HTTP/gRPC frameworks
- SQL drivers and ORMs
- OpenTelemetry for observability

Dependencies are managed via Go modules.
Maintainers periodically run `go mod tidy` and static analysis tools to reduce dependency risk.

---

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly.

- **Do not** open a public GitHub issue.
- Send a report to: **security@valora.example**
  (replace with a real address before public release)

Please include:
- A clear description of the issue
- Steps to reproduce (if applicable)
- Potential impact assessment

We will acknowledge reports and aim to respond in a reasonable timeframe.

---

## Security Philosophy

Valora follows a **security-by-design** approach:

- Minimize security scope by avoiding payment processing
- Prefer explicit boundaries over implicit behavior
- Treat billing correctness as a deterministic computation
- Delegate high-risk domains (payments, card data) to specialized providers

---

## Disclaimer

Valora is provided "as is", without warranty of any kind.
Security responsibility is shared between Valora and the adopting system, depending on deployment and integration choices.
