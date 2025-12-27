# Security Policy

## Overview

Valora OSS is an open-source billing engine designed to manage **billing logic**, including usage metering, pricing, rating, subscriptions, and invoice generation.

Valora OSS **is not a payment processor** and **does not handle payment instruments**.

This document defines the **security scope, assumptions, and responsibilities** for users deploying Valora OSS.

---

## Security Scope

### In Scope

Valora OSS is responsible for:

- Correct and deterministic billing computation
- Usage ingestion and aggregation logic
- Pricing and rating configuration handling
- Subscription lifecycle state transitions
- Invoice generation and state management
- Application-level authentication and authorization hooks
- Logical tenant (organization) data isolation within the application domain

---

### Explicitly Out of Scope

Valora OSS **does NOT**:

- Store, transmit, or process credit card data
- Handle payment execution, settlement, or reconciliation
- Act as a payment gateway or merchant of record
- Implement cryptographic primitives
- Manage customer payment credentials or secrets
- Provide network-level security, infrastructure hardening, or runtime isolation

Payment execution must be handled by **external payment providers** (e.g. Stripe, Midtrans, Xendit) integrated by the adopting system.

As a result, Valora OSS **does not fall under PCI-DSS scope by design**.

---

## Data Handling

Valora OSS may store billing-related data such as:

- Usage records
- Pricing and rating configuration
- Subscription state
- Invoice metadata

Sensitive payment data (e.g. PAN, CVV, bank account details) **must never be sent** to Valora APIs.

Identifiers and external references (e.g. customer IDs, payment references, invoice IDs) are treated as **opaque values** and are not interpreted or validated beyond structural requirements.

---

## Authentication & Authorization

- Valora OSS enforces authentication and authorization at the application boundary.
- Authorization decisions are scoped to organizations (tenants).
- Valora OSS assumes that **upstream identity providers and access control systems** are responsible for:
  - User authentication
  - Credential management
  - Secret rotation
  - Session security

---

## Deployment Responsibility (OSS)

Valora OSS is **self-hosted software**.

The adopting organization is responsible for:

- Infrastructure security (networking, firewalls, TLS, secrets management)
- Database security and access control
- Runtime isolation and resource limits
- Backup, recovery, and operational monitoring
- Compliance obligations applicable to their deployment

Valora OSS does not provide operational or infrastructure-level security guarantees.

---

## Dependency Security

Valora OSS relies on commonly used Go libraries and infrastructure components, including:

- HTTP and gRPC frameworks
- SQL drivers and ORMs
- OpenTelemetry for observability

Dependencies are managed using Go modules.  
Maintainers periodically perform dependency hygiene and static analysis, but **users are encouraged to perform their own security reviews** appropriate to their risk profile.

---

## Reporting Security Issues

If you discover a security vulnerability in Valora OSS:

- **Do not** open a public GitHub issue.
- Report it privately via: **security@valora.example**  
  (replace with a valid address before public release)

Please include:
- A clear description of the issue
- Steps to reproduce (if applicable)
- An assessment of potential impact

We aim to acknowledge reports within a reasonable timeframe.

---

## Security Philosophy

Valora OSS follows a **security-by-design** philosophy:

- Minimize security scope by avoiding payment processing
- Prefer explicit boundaries over implicit behavior
- Treat billing correctness as a deterministic computation
- Delegate high-risk domains (payments, card data, credential storage) to specialized systems

---

## Disclaimer

Valora OSS is provided "as is", without warranty of any kind.

Security responsibilities are **shared** between Valora OSS and the adopting system, depending on deployment architecture, integration choices, and operational controls.