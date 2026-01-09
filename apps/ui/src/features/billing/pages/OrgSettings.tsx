import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useOrgStore } from "@/stores/orgStore"

export default function OrgSettings() {
  const org = useOrgStore((s) => s.currentOrg)

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
    </div>
  )
}
