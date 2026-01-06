
# Valora OSS

**Valora OSS** is an open-source **deterministic billing computation engine** for modern SaaS and platform products.

![Release CI](https://github.com/smallbiznis/valora/actions/workflows/github-release.yml/badge.svg)
![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)
![Release](https://img.shields.io/github/v/release/smallbiznis/valora)
![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)

Valora extracts billing concerns—usage metering, pricing, subscriptions, and invoicing—out of application code and into a **dedicated, self-hosted engine** with explicit boundaries.

> **Valora determines what should be billed.
> It does not execute payments.**

---

## Why Valora Exists

Billing is a **financial truth system**, not a convenience feature.

In many systems, billing logic is:

- scattered across application code,
- difficult to audit or reproduce,
- tightly coupled with payments and entitlements,
- fragile under scale and change.

Valora takes a deliberate approach:
**make billing boring, deterministic, and explainable.**

This project is developed independently, outside of any employment responsibilities, as a space to design systems with clear boundaries, correctness, and accountability.

---

## What Valora Is

Valora is a **billing computation engine**, not an all-in-one billing platform.

### Core Capabilities (v1.0 Target)

- **Subscription Management**Trialing, active, past_due, and canceled states with explicit lifecycle transitions
- **Usage Metering**Idempotent ingestion, deterministic aggregation, late and out-of-order handling
- **Pricing Models**Flat-rate, tiered usage, and hybrid (base + usage) pricing with time-bound windows
- **Invoicing**Deterministic line-item generation, proration, and invoice state management
- **Multi-Tenancy**Organization-scoped isolation and authorization
- **Audit Trail**
  Immutable event log for all billing state changes

---

## Design Principles

Valora is built around strict principles:

- **Deterministic by Design**Billing outputs are derived solely from persisted inputs and configuration.
- **Explicit Boundaries**Billing logic is separated from payments, identity, and infrastructure concerns.
- **Self-Hosted Ownership**Teams retain full control over billing data and logic.
- **Composable Primitives**
  Pricing behavior is modeled explicitly, not hidden in application code.

---

## Scope & Non-Goals

### What Valora Is

Valora is a **deterministic billing computation engine** designed for modern SaaS and platform systems.
It extracts billing logic from application code into a dedicated, auditable system with explicit boundaries.

### What Valora Is Not (v1.0)

To preserve correctness and clarity, the following are **intentionally out of scope** for v1.0:

#### Payment Execution

- No credit card processing, bank transfers, or settlement
- No payment gateway integrations (Stripe, PayPal, etc.)
- Determines *what* to bill, not *how* to collect payment
- *Post-v1.0*: payment adapter interface

#### Merchant of Record & Compliance

- No tax calculation (VAT, GST, sales tax)
- No PCI-DSS, PSD2, or regulatory automation
- Tax amounts may be stored as external line items
- *Post-v1.0*: tax metadata structure

#### Dunning & Collections

- No retry logic for failed payments
- No automated email or recovery workflows
- Past-due state exists, recovery is external
- *Post-v1.0*: webhook events

#### Revenue Recognition & Accounting

- No GAAP / IFRS compliance
- No deferred revenue tracking
- Cash-basis invoice generation only
- *Post-v1.0*: ledger API integrations

#### Multi-Currency & FX

- Single currency per tenant
- No real-time FX handling
- *Post-v1.0*: multi-currency via external FX providers

#### Entitlements & Feature Gating

- No access control or feature flags
- Application layer is responsible
- *Post-v1.0*: entitlement sync via webhooks

#### Customer Self-Service UI

- No built-in customer portal
- API-first design
- *Post-v1.0*: optional reference UI

#### Advanced Analytics & Reporting

- No MRR/ARR dashboards or cohort analysis
- Raw data available via API
- *Post-v1.0*: Valora Cloud analytics layer

#### Complex Proration Models

- Day-based proration only
- Boundary rule: `[start inclusive, end exclusive]`
- *Post-v1.0*: configurable proration strategies

---

## Why These Boundaries Matter

Billing errors cost money, trust, and legal risk.

By limiting scope, Valora ensures:

- reproducible and auditable billing outputs,
- clear ownership of responsibilities,
- clean integration with best-of-breed tools,
- no black-box financial behavior.

Self-hosting is a **feature**, not a limitation:

- full data ownership,
- internal auditability,
- no vendor lock-in,
- compliance with data residency requirements.

---

## Architecture & Security

First-class documentation is provided:

- `ARCHITECTURE.md` — deterministic billing flows and trust boundaries
- `SECURITY.md` — security scope and assumptions
- `THREAT_MODEL.md` — lightweight threat model

---

## Deployment Model

Valora OSS is **self-hosted software**.

The adopting organization is responsible for:

- infrastructure and networking,
- TLS termination,
- secrets management,
- database operations and backups.

Valora makes minimal assumptions about the runtime environment.

---

## Who Valora Is For

Valora is designed for teams that:

- are scaling beyond hardcoded billing logic,
- operate usage-based or hybrid pricing models,
- build long-lived SaaS or platform systems,
- value correctness, clarity, and auditability.

Valora is **not a good fit** if you need:

- an all-in-one billing + payments + tax solution,
- no-code billing configuration,
- a managed service with instant SLA guarantees.

---

## Roadmap (High-Level)

**Toward v1.0**

- Harden subscription and billing lifecycles
- Deterministic usage ingestion
- Invoice reproducibility guarantees
- End-to-end billing truth tests
- Stable APIs and semantic contracts

**Post-v1.0**

- Payment adapter interfaces
- Webhook event system
- Multi-currency support
- Tax metadata integration
- Optional **Valora Cloud**

---

## Observability

Grafana dashboards expect the following metrics:

- `valora_scheduler_job_runs_total`
- `valora_scheduler_job_duration_seconds`
- `valora_scheduler_job_timeouts_total`
- `valora_scheduler_job_errors_total`
- `valora_scheduler_batch_processed_total`
- `valora_scheduler_batch_deferred_total`
- `valora_scheduler_runloop_lag_seconds`

---

## License

Valora OSS is released under the Apache 2.0 License.
See the `LICENSE` file for details.

---

## Documentation

📚 Documentation source: `./docs/docs/index.md`

> **Valora aims to make billing boring, predictable, and explainable**
> so teams can focus on building their products.
>
