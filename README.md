# Valora OSS

**Valora OSS** is an open-source **deterministic billing engine** for modern SaaS and platform products.

![Release CI](https://github.com/smallbiznis/valora/actions/workflows/github-release.yml/badge.svg)
![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)
![Release](https://img.shields.io/github/v/release/smallbiznis/valora)
![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)

Valora extracts billing concerns—usage metering, pricing, subscriptions, and invoicing—out of application code and into a **dedicated, self-hosted engine** with explicit boundaries.

Valora computes **what should be billed**, not **how payments are executed**.

## What Valora Is (and Is Not)

### Valora **is**:

- A billing computation engine
- A control plane for pricing and usage-based billing
- A deterministic system for generating invoices and billing states
- Designed for self-hosted, multi-tenant SaaS

### Valora **is not**:

- A payment gateway
- A merchant of record
- A settlement or reconciliation system
- An infrastructure or SLA platform

Payment execution is intentionally **out of scope**.

---

## Core Capabilities

### Subscription Management

- Flat-rate and usage-based subscriptions
- Trialing, active, past_due, canceled states
- Scheduled cancellation and renewal windows
- Organization-scoped isolation

### Usage Metering & Rating

- Metered billing via usage events
- Flexible pricing models (flat, tiered, hybrid)
- Deterministic aggregation per billing period
- Idempotent usage ingestion

### Invoicing

- Automated invoice generation
- Subscription + usage line items
- Explicit invoice state transitions
- Proration support (where applicable)

### Multi-Tenancy

- Logical tenant isolation
- Organization-scoped authorization
- Designed for SaaS and platform architectures

---

## Design Principles

Valora is built around a few strict principles:

- **Deterministic by design**Billing outputs are derived solely from persisted inputs and configuration.
- **Explicit boundaries**Billing logic is separated from payments, identity, and infrastructure concerns.
- **Self-hosted ownership**Teams retain control over data, logic, and deployment.
- **Composable primitives**
  Pricing behavior is modeled explicitly, not hidden in application code.

---

## Architecture & Security

Valora OSS includes first-class documentation:

- **Architecture overview and trust boundaries** → `ARCHITECTURE.md`
- **Security scope and assumptions** → `SECURITY.md`
- **Lightweight threat model** → `THREAT_MODEL.md`

These documents describe:

- deterministic billing flow
- tenant isolation model
- security responsibilities
- explicit non-goals

---

## Deployment Model

Valora OSS is **self-hosted software**.

The adopting organization is responsible for:

- infrastructure security
- TLS termination and networking
- secrets management
- database operations and backups

Valora makes minimal assumptions about the runtime environment to remain portable.

---

## Who Valora Is For

- SaaS teams scaling beyond hardcoded billing
- Platform products with usage-based pricing
- Backend engineers building long-lived systems
- Teams that value clarity, ownership, and auditability

---

## Roadmap (High-Level)

- Harden core billing lifecycle
- Expand pricing and proration models
- Improve usage ingestion and aggregation
- Optional hosted control plane (**Valora Cloud**, future)

---

## Releases

Releases are tag-driven and backed by a curated changelog.

- Release policy and tagging flow: `RELEASE.md`
- Release notes and history: `CHANGELOG.md`

---

## License

Valora OSS is open-source.
See the `LICENSE` file for details.

---

## Documentation

📚 Read the documentation at:

- Local: http://localhost:3000 (after `pnpm start` in /docs)
- Source: ./docs/docs/index.md

> Valora aims to make billing **boring, predictable, and explainable**—so teams can focus on building their products.
