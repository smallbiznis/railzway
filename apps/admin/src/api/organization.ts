import { auth } from "./client"
import type { InviteRequest } from "@/types/organization"

export async function inviteMembers(orgId: string, invites: InviteRequest[]) {
  return auth.post(`/user/orgs/${orgId}/invites`, { invites })
}

export async function acceptInvite(inviteId: string) {
  return auth.post(`/invites/${inviteId}/accept`)
}
