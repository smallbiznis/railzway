import { auth } from "./client"
import type { InviteRequest } from "@/types/organization"

export async function inviteMembers(orgId: string, invites: InviteRequest[]) {
  return auth.post(`/user/orgs/${orgId}/invites`, { invites })
}

export async function acceptInvite(inviteId: string) {
  return auth.post(`/invites/${inviteId}/accept`)
}

export async function getInviteInfo(inviteId: string) {
  const res = await auth.get(`/invites/${inviteId}`)
  return res.data
}

export async function completeInvite(inviteId: string, data: { password: string; name: string; username?: string }) {
  const res = await auth.post(`/invites/${inviteId}/complete`, data)
  return res.data // Returns Session
}
