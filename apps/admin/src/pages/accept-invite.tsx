import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert } from "@/components/ui/alert"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { acceptInvite, getInviteInfo } from "@/api/organization"
import { useAuthStore } from "@/stores/authStore"

interface PublicInviteInfo {
  id: string
  org_id: string
  org_name: string
  email: string
  role: string
  status: string
  invited_by: string
}

export default function AcceptInvitePage() {
  const { inviteId } = useParams<{ inviteId: string }>()
  const navigate = useNavigate()
  const { isAuthenticated, user, completeInvite } = useAuthStore()

  const [loading, setLoading] = useState(false)
  const [fetching, setFetching] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [inviteInfo, setInviteInfo] = useState<PublicInviteInfo | null>(null)

  // Form state for new users
  const [name, setName] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")

  useEffect(() => {
    if (!inviteId) return
    setFetching(true)
    getInviteInfo(inviteId)
      .then((data) => {
        setInviteInfo(data)
        setFetching(false)
      })
      .catch((err) => {
        console.error(err)
        setError("Invalid or expired invite link.")
        setFetching(false)
      })
  }, [inviteId])

  const handleAcceptExisting = async () => {
    if (!inviteId) return
    setError(null)
    setLoading(true)
    try {
      await acceptInvite(inviteId)
      navigate("/orgs", { replace: true })
    } catch (err: any) {
      setError(
        err?.response?.data?.message ||
        err?.message ||
        "Failed to accept invitation."
      )
    } finally {
      setLoading(false)
    }
  }

  const handleCreateAndAccept = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteId) return
    if (password !== confirmPassword) {
      setError("Passwords do not match")
      return
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters")
      return
    }

    setError(null)
    setLoading(true)
    try {
      await completeInvite(inviteId, {
        password,
        name,
        username: inviteInfo?.email.split("@")[0] || "user"
      })
      navigate("/orgs", { replace: true })
    } catch (err: any) {
      setError(
        err?.response?.data?.message ||
        err?.message ||
        "Failed to accept invitation."
      )
    } finally {
      setLoading(false)
    }
  }

  if (!inviteId || error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle>Invalid Invite</CardTitle>
            <CardDescription>{error || "This invitation link is not valid."}</CardDescription>
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

  if (fetching) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
        <div className="text-sm text-text-muted">Loading invite details...</div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Join {inviteInfo?.org_name}</CardTitle>
          <CardDescription>
            You've been invited to join <strong>{inviteInfo?.org_name}</strong> as <strong>{inviteInfo?.role}</strong>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && <Alert variant="destructive">{error}</Alert>}

          {isAuthenticated ? (
            <div className="space-y-4">
              <div className="bg-bg-subtle p-3 rounded-md text-sm">
                You are logged in as <strong>{user?.email}</strong>.
              </div>
              <Button onClick={handleAcceptExisting} disabled={loading} className="w-full">
                {loading ? "Accepting..." : "Accept Invitation"}
              </Button>
              <Button variant="outline" onClick={() => navigate("/orgs")} className="w-full">
                Cancel
              </Button>
            </div>
          ) : (
            <form onSubmit={handleCreateAndAccept} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" value={inviteInfo?.email} disabled />
              </div>
              <div className="space-y-2">
                <Label htmlFor="name">Full Name</Label>
                <Input
                  id="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="John Doe"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Create Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirmPassword">Confirm Password</Label>
                <Input
                  id="confirmPassword"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                />
              </div>
              <Button type="submit" disabled={loading} className="w-full">
                {loading ? "Creating Account..." : "Create Account & Join"}
              </Button>
              <div className="text-center text-sm text-text-muted">
                Already have an account? <a href="/login" className="text-primary hover:underline">Log in</a>
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
