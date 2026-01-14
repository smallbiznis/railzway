import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useOrgStore } from "@/stores/orgStore"
import { InviteMemberDialog } from "../components/InviteMemberDialog"
import { TeamMembersList } from "../components/TeamMembersList"
import { toast } from "sonner"

import { useState, useEffect } from "react"
import { auth } from "@/api/client"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Edit2 } from "lucide-react"

export default function OrgSettings() {
  const org = useOrgStore((s) => s.currentOrg)
  const setOrg = useOrgStore((s) => s.setCurrentOrg)
  const [isEditing, setIsEditing] = useState(false)
  const [name, setName] = useState(org?.name ?? "")

  useEffect(() => {
    if (org?.name) setName(org.name)
  }, [org?.name])

  const handleUpdate = async () => {
    if (!org) return
    try {
      // Using auth client because the route is under /auth/user/orgs/:id
      // Wait, checking server.go, I added it to `user` group which is `auth.Group("/user")`.
      // So endpoint is /auth/user/orgs/:id.
      // auth client base is /auth.
      const res = await auth.patch(`/user/orgs/${org.id}`, { name })

      // Update local store
      setOrg({ ...org, name: res.data.name })
      setIsEditing(false)
      toast.success("Organization updated")
    } catch (error) {
      console.error(error)
      toast.error("Failed to update organization")
    }
  }

  const handleInviteSuccess = () => {
    toast.success("Invitation sent", {
      description: "The team member will receive an email with instructions to join.",
    })
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Settings</h1>

      <Card>
        <CardHeader>
          <CardTitle>Workspace</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="text-sm">
            <div className="flex items-center justify-between">
              {isEditing ? (
                <div className="flex items-center gap-2">
                  <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="h-8 w-[200px]"
                  />
                  <Button size="sm" onClick={handleUpdate}>Save</Button>
                  <Button size="sm" variant="ghost" onClick={() => setIsEditing(false)}>Cancel</Button>
                </div>
              ) : (
                <div className="font-medium flex items-center gap-2">
                  {org?.name}
                  <Button size="sm" variant="ghost" className="h-6 w-6 p-0" onClick={() => setIsEditing(true)}>
                    <Edit2 className="h-3 w-3" />
                  </Button>
                </div>
              )}
            </div>
            <div className="text-text-muted">ID: {org?.id}</div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>Team Members</CardTitle>
            <CardDescription>Manage who has access to this organization</CardDescription>
          </div>
          <InviteMemberDialog onSuccess={handleInviteSuccess} />
        </CardHeader>
        <CardContent>
          <TeamMembersList />
        </CardContent>
      </Card>
    </div>
  )
}
