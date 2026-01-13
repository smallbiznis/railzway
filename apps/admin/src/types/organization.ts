export type Role = "OWNER" | "ADMIN" | "FINOPS" | "DEVELOPER" | "MEMBER"

export interface OrganizationMember {
  id: string
  user_id: string
  email: string
  role: Role
  created_at: string
}

export interface OrganizationInvite {
  id: string
  org_id: string
  email: string
  role: Role
  status: "pending" | "accepted" | "expired"
  invited_by: string
  created_at: string
}

export interface InviteRequest {
  email: string
  role: Role
}
