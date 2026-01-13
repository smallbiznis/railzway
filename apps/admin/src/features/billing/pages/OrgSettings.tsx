import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useOrgStore } from "@/stores/orgStore"
import { InviteMemberDialog } from "../components/InviteMemberDialog"
import { toast } from "sonner"

export default function OrgSettings() {
  const org = useOrgStore((s) => s.currentOrg)

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
            <div className="font-medium">{org?.name}</div>
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
          <p className="text-sm text-text-muted">
            Team member management is available here. Invite colleagues to collaborate on billing,
            usage tracking, and more.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
