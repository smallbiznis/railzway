import { useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert } from "@/components/ui/alert"
import { acceptInvite } from "@/api/organization"

export default function AcceptInvitePage() {
  const { inviteId } = useParams<{ inviteId: string }>()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleAccept = async () => {
    if (!inviteId) {
      setError("Invalid invite link")
      return
    }

    setError(null)
    setLoading(true)

    try {
      await acceptInvite(inviteId)
      // Redirect to orgs page to select the organization
      navigate("/orgs", { replace: true })
    } catch (err: any) {
      setError(
        err?.response?.data?.message ||
        err?.message ||
        "Failed to accept invitation. The invite may have expired or already been accepted."
      )
    } finally {
      setLoading(false)
    }
  }

  if (!inviteId) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle>Invalid Invite</CardTitle>
            <CardDescription>This invitation link is not valid.</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => navigate("/login")} className="w-full">
              Go to Login
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Accept Invitation</CardTitle>
          <CardDescription>
            You've been invited to join an organization. Click below to accept and get started.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && <Alert variant="destructive">{error}</Alert>}

          <div className="space-y-2">
            <Button onClick={handleAccept} disabled={loading} className="w-full">
              {loading ? "Accepting..." : "Accept Invitation"}
            </Button>
            <Button
              variant="outline"
              onClick={() => navigate("/login")}
              className="w-full"
            >
              Cancel
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
