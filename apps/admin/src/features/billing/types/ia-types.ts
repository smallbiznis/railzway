// Type definitions for Billing Operations IA (Information Architecture)

// ===== Common Types =====

export type EntityType = "invoice" | "customer"

export type AssignmentStatus = "assigned" | "in_progress" | "released" | "resolved" | "escalated"

export type SLAStatus = "fresh" | "active" | "aging" | "stale" | "breached" | "resolved"

export type ResolutionType =
  | "payment_received"
  | "issue_fixed"
  | "customer_contacted"
  | "escalated_to_manager"
  | "other"

// ===== Inbox Types =====

export interface InboxItem {
  entity_type: EntityType
  entity_id: string
  entity_name: string // Invoice number or customer name
  category: string // "overdue_invoice" | "failed_payment" | "high_exposure" (Deprecated / Mapped)
  risk_category?: string // "high_exposure" | "overdue_invoice"
  risk_score?: number
  amount_due?: number
  days_overdue?: number
  risk_level?: string
  currency: string
  customer_id?: string
  customer_name?: string
  invoice_id?: string
  invoice_number?: string
  last_attempt?: string | null
}

export interface InboxResponse {
  items: InboxItem[]
  currency: string
  next_cursor?: string
}

// ===== My Work Types =====

export interface MyWorkItem {
  assignment_id: string
  entity_type: EntityType
  entity_id: string
  entity_name: string

  // Snapshot values (at claim time)
  amount_due_at_claim: number
  days_overdue_at_claim: number

  // Current values (optional)
  current_amount_due?: number
  current_days_overdue?: number

  currency: string
  customer_id?: string
  customer_name?: string
  customer_email?: string
  invoice_id?: string
  invoice_number?: string

  // Assignment metadata
  claimed_at: string // ISO timestamp
  assignment_expires_at: string // ISO timestamp
  assignment_age_minutes: number
  status: AssignmentStatus
  sla_status: SLAStatus

  // Snapshot metadata (full JSON)
  snapshot_metadata?: Record<string, any>
}

export interface MyWorkResponse {
  items: MyWorkItem[]
  currency: string
}

// ===== Recently Resolved Types =====

export interface RecentlyResolvedItem {
  assignment_id: string
  entity_type: EntityType
  entity_id: string
  entity_name: string

  // Resolution details
  status: "resolved" | "released" | "escalated"
  resolved_at: string // ISO timestamp
  resolved_by: string
  resolution?: string
  release_reason?: string

  // Original snapshot values
  amount_due_at_claim: number
  days_overdue_at_claim: number

  currency: string
  customer_id?: string
  customer_name?: string
  invoice_id?: string
  invoice_number?: string

  // Metrics
  claimed_at: string
  days_to_resolve: number
}

export interface RecentlyResolvedResponse {
  items: RecentlyResolvedItem[]
  currency: string
}

// ===== Team View Types =====

export interface TeamMember {
  user_id: string
  user_name: string
  active_assignments: number
  average_assignment_age_minutes: number
  total_exposure: number
  escalation_count: number
  resolved_count_30d: number
}

export interface TeamSummary {
  total_active_assignments: number
  total_exposure: number
  average_assignment_age_minutes: number
  escalation_count: number
  currency: string
}

export interface TeamViewResponse {
  team_members: TeamMember[]
  summary: TeamSummary
}

// ===== Action Request Types =====

export interface ClaimAssignmentRequest {
  entity_type: EntityType
  entity_id: string
  assigned_to?: string
  assignment_ttl_minutes?: number
}

export interface ClaimAssignmentResponse {
  assignment: {
    entity_type: EntityType
    entity_id: string
    status: AssignmentStatus
    assigned_to: string
    assigned_at: string
    assignment_expires_at: string
  }
  status: string
}

export interface ResolveAssignmentRequest {
  entity_type: EntityType
  entity_id: string
  resolution: string
  resolved_by?: string
}

export interface ResolveAssignmentResponse {
  status: "resolved"
}

export interface ReleaseAssignmentRequest {
  entity_type: EntityType
  entity_id: string
  reason: string
  released_by?: string
}

// ===== Performance Types =====

export interface PerformanceMetrics {
  avg_response_minutes: number
  completion_ratio: number
  escalation_ratio: number
  exposure_handled: number
  total_assigned: number
  total_resolved: number
  total_escalated: number
}

export interface PerformanceScores {
  responsiveness: number
  completion: number
  effectiveness: number
  risk: number
  total: number
}

export interface FinOpsScoreSnapshot {
  period_start: string
  period_end: string
  metrics: PerformanceMetrics
  scores: PerformanceScores
}

export interface PerformanceData {
  current: FinOpsScoreSnapshot
  history: FinOpsScoreSnapshot[]
}

export interface PerformanceResponse {
  user_id: string
  period_type: string
  scoring_version: string
  snapshots: FinOpsScoreSnapshot[]
}

// ===== Exposure Types =====

export interface ExposureBucket {
  bucket: string
  amount: number
  count: number
}

export interface ExposureCategory {
  category: string
  amount: number
  count: number
}

export interface ExposureAnalysisResponse {
  total_exposure: number
  currency: string
  by_risk_category: ExposureCategory[]
  by_aging_bucket: ExposureBucket[]
  top_high_exposure: InboxItem[]
}
