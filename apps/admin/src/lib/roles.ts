export type OrgRole = "OWNER" | "ADMIN" | "MEMBER" | string

const normalizeRole = (role?: string) => (role ?? "").trim().toUpperCase()

export const isOrgOwner = (role?: string) => normalizeRole(role) === "OWNER"

export const isOrgAdmin = (role?: string) => normalizeRole(role) === "ADMIN"

export const canManageBilling = (role?: string) => {
  const r = normalizeRole(role)
  return r === "OWNER" || r === "ADMIN" || r === "FINOPS" || r === "DEVELOPER"
}
