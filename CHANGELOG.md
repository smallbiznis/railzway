# Changelog

All notable changes to Valora will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Notes

- Frontend UI for Billing Operations IA in progress
- Additional integration tests planned
- OpenAPI/Swagger documentation planned

## [1.0.1] - 2026-01-15

### Fixed
- **GitHub Actions**: Fixed "Unrecognized named-value: 'secrets'" error in Docker and GitHub release workflows by correctly passing secrets to reusable workflows.

## [1.0.0] - 2026-01-15

### Added

#### Rebranding & Licensing
- **Official Rebranding**: Project transitioned from Valora to **Railzway**.
- **License Transition**: Shifted to **GNU Affero General Public License v3.0 (AGPL-3.0)**.

#### Telemetry & Observability
- **Anonymous Telemetry (Phone Home)**: Implemented background worker for usage statistics.
- **Enhanced Metrics**: Added Pulse, Engine Errors, Organization totals, and System Health (Memory) metrics.
- **Grafana Dashboard**: Included official telemetry dashboard JSON in `docs/grafana`.

#### DevOps & CI/CD
- **Standardized Docker**: All services updated with build-time version and telemetry injection.
- **Relocated Dockerfile**: Moved core Dockerfile to `apps/railzway/Dockerfile` for consistency.

#### Billing Operations - Information Architecture (IA) [from rc.1]

**Task-Centric Finance Operations Workspace**

Implemented a complete Information Architecture for Billing Operations with four distinct views designed for human-centric finance workflows:

**Inbox View**
- Triage queue for unassigned risky items (overdue invoices, failed payments, high exposure customers)
- `GET /admin/billing-operations/inbox` endpoint
- Claim functionality to assign items to users
- Dynamic routing: items appear only if they meet risk criteria AND have no active assignment

**My Work View**
- Stable list of tasks claimed by the current user
- `GET /admin/billing-operations/my-work` endpoint
- **Snapshot-based task stability**: Tasks remain visible even when underlying billing state changes (e.g., invoice paid)
- Captures entity state at claim time (amount due, days overdue, customer info)
- Actions: Resolve, Release, Escalate
- SLA status tracking (fresh, active, aging, stale, breached)

**Recently Resolved View**
- Audit trail of completed work (30-day window)
- `GET /admin/billing-operations/recently-resolved` endpoint
- Read-only view with resolution details
- Shows resolution type, who resolved, and time to resolution

**Team View**
- Manager oversight without competitive rankings
- `GET /admin/billing-operations/team` endpoint
- Team member statistics and summary metrics
- Role-gated (Owner/Admin only)
- Explicitly avoids leaderboards or best/worst labels

**New API Endpoints**
- `POST /admin/billing-operations/claim` - Claim an assignment to current user
- `POST /admin/billing-operations/resolve` - Mark assignment as successfully resolved
- `POST /admin/billing-operations/release` - Release assignment back to queue

**Database Schema**
- Added `resolved_at` (TIMESTAMPTZ) column to track resolution time
- Added `resolved_by` (TEXT) column to track who resolved the assignment
- Migration 0035: Automatic data migration from `released_at` to `resolved_at`
- Index `idx_billing_assignments_resolved_at` for Recently Resolved view performance
- Fixed GORM table name mapping with `TableName()` method

**Service Layer**
- `ResolveAssignment()` - Mark assignment as successfully completed with resolution type
- Updated `ReleaseAssignment()` - Now populates both `released_at` and `resolved_at`
- Updated `EvaluateSLAs()` - Auto-escalation sets `resolved_at` and `resolved_by="system"`
- `GetInbox()` - Returns unassigned risky items
- `GetMyWork()` - Returns user's claimed tasks with snapshot data
- `GetRecentlyResolved()` - Returns completed work from last 30 days
- `GetTeamView()` - Returns team performance overview

**Architecture Principles**
- **Task Stability**: Claimed work never disappears due to billing state changes
- **Human-Centric Design**: Focus on user workflow, not real-time data freshness
- **Finance-Grade Correctness**: Snapshot values ensure consistency
- **Complete Audit Trail**: All state transitions logged with timestamps and actors
- **Separation of Concerns**: Live financial data separated from human-owned work

**State Machine**
```
null → assigned (Claim)
assigned → resolved (Resolve - successful completion)
assigned → released (Release - return to queue)
assigned → escalated (Auto-escalate on SLA breach)
```

**SLA Evaluation**
- Initial Response SLA: 1 hour from assignment
- Idle Action SLA: 4 hours since last action
- Automatic escalation on breach
- Visual status indicators: fresh → active → aging → stale → breached

#### Core Billing System

- Usage ingestion with idempotency controls
- Flexible pricing engine (flat, tiered, volume, package)
- Subscription lifecycle management
- Invoice generation and finalization
- Payment processing integration
- Ledger accounting system
- Customer management
- Product catalog
- Tax calculation support

### Changed

- Billing Operations now uses task-centric IA instead of data-centric views
- Assignment lifecycle now tracks both release and resolution separately
- SLA evaluation includes resolved assignments in Recently Resolved view

### Fixed

- GORM table name mapping error (`billing_assignment_records` → `billing_operation_assignments`)
- Migration 0034 attempting to create index on non-existent `resolved_at` column
- Recently Resolved view query referencing missing database columns
- Missing package declaration in `ia_methods.go`

### Security

- Role-based access control for all Billing Operations endpoints
- Team View restricted to Owner/Admin roles
- Assignment actions validate user ownership
- Complete audit logging for all state transitions

### Performance

- Indexed queries for all IA views
- Partial indexes for status-based filtering
- Snapshot-based performance metrics (no live aggregation)
- Efficient connection pooling

### Database Migrations

- Migration 0034: Assignment snapshot metadata
- Migration 0035: Resolved columns with automatic data migration

### Technical Debt

- Integration test coverage can be improved
- OpenAPI/Swagger documentation pending
- Performance benchmarks not yet established
- Frontend UI implementation in progress

### Breaking Changes

None. All changes are additive and backward compatible.

### Notes

- Backend API is stable and production-ready
- Frontend UI for Billing Operations IA is 60% complete
- Recommended 2-4 weeks of production validation before v1.0.0 final release
- API contracts will not change between rc.1 and v1.0.0

---

## Pre-1.0 Development

Prior to v1.0.0-rc.1, the project was in active development with frequent breaking changes.
