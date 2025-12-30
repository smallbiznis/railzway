import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"

import { admin } from "@/api/client"
import { Alert } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { Separator } from "@/components/ui/separator"
import { useOnboardingStore } from "@/stores/onboardingStore"

type Invite = { email: string; role: "OWNER" | "ADMIN" | "MEMBER" }

export default function OnboardingPage() {
  const navigate = useNavigate()
  const { currentStep, nextStep, prevStep, setOrgId, orgId, reset } = useOnboardingStore()

  const [orgName, setOrgName] = useState("")
  const [invites, setInvites] = useState<Invite[]>([])
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState<Invite["role"]>("ADMIN")
  const [currency, setCurrency] = useState("IDR")
  const [timezone, setTimezone] = useState("Asia/Jakarta")
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    reset()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (currentStep > 1 && !orgId) {
      reset()
    }
  }, [currentStep, orgId, reset])

  const progress = useMemo(() => (currentStep / 4) * 100, [currentStep])

  const handleCreateOrg = async (event: React.FormEvent) => {
    event.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const res = await admin.post("/organizations", {
        name: orgName,
        country_code: "ID",
        timezone_name: timezone,
        default_currency: currency,
      })
      const newOrgId = res.data?.org_id ?? res.data?.id
      setOrgId(newOrgId)
      nextStep()
    } catch (err: any) {
      setError(err?.message ?? "Unable to create organization.")
    } finally {
      setLoading(false)
    }
  }

  const handleSendInvites = async () => {
    if (!orgId || invites.length === 0) {
      nextStep()
      return
    }
    setError(null)
    setLoading(true)
    try {
      await admin.post(`/organizations/${orgId}/invites`, { invites })
      nextStep()
    } catch (err: any) {
      setError(err?.message ?? "Unable to send invites.")
    } finally {
      setLoading(false)
    }
  }

  const handleBillingPrefs = async () => {
    if (!orgId) {
      setError("Organization missing. Please start again.")
      return
    }
    setError(null)
    setLoading(true)
    try {
      await admin.post(`/organizations/${orgId}/billing-preferences`, {
        currency,
        timezone,
      })
      nextStep()
    } catch (err: any) {
      setError(err?.message ?? "Unable to save preferences.")
    } finally {
      setLoading(false)
    }
  }

  const handleFinish = () => {
    if (!orgId) {
      setError("Organization missing. Please start again.")
      return
    }
    navigate(`/orgs/${orgId}/dashboard`, { replace: true })
  }

  const addInvite = () => {
    if (!inviteEmail) return
    setInvites((prev) => [...prev, { email: inviteEmail, role: inviteRole }])
    setInviteEmail("")
  }

  const renderStep = () => {
    switch (currentStep) {
      case 1:
        return (
          <form className="space-y-4" onSubmit={handleCreateOrg}>
            <div className="space-y-2">
              <Label htmlFor="org-name">Organization name</Label>
              <Input
                id="org-name"
                data-testid="onboarding-org-name"
                value={orgName}
                onChange={(e) => setOrgName(e.target.value)}
                required
                placeholder="Acme Inc."
              />
            </div>
            {error && <Alert variant="destructive">{error}</Alert>}
            <Button
              type="submit"
              className="w-full"
              data-testid="onboarding-continue"
              disabled={loading}
            >
              {loading ? "Creating..." : "Continue"}
            </Button>
          </form>
        )
      case 2:
        return (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="invite-email">Invite by email</Label>
              <Input
                id="invite-email"
                data-testid="onboarding-invite-email"
                type="email"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                placeholder="user@example.com"
              />
              <Label htmlFor="invite-role">Role</Label>
              <Input
                id="invite-role"
                data-testid="onboarding-invite-role"
                value={inviteRole}
                onChange={(e) =>
                  setInviteRole(e.target.value.toUpperCase() as Invite["role"])
                }
                placeholder="OWNER / ADMIN / MEMBER"
              />
              <Button
                variant="outline"
                data-testid="onboarding-add-invite"
                onClick={addInvite}
                disabled={!inviteEmail}
              >
                Add invite
              </Button>
              <div className="flex flex-wrap gap-2">
                {invites.map((invite, idx) => (
                  <Badge key={`${invite.email}-${idx}`} variant="secondary">
                    {invite.email} · {invite.role}
                  </Badge>
                ))}
              </div>
            </div>
            {error && <Alert variant="destructive">{error}</Alert>}
            <div className="flex items-center gap-2">
              <Button data-testid="onboarding-send-invites" onClick={handleSendInvites} disabled={loading}>
                {loading ? "Sending..." : "Send invites"}
              </Button>
              <Button variant="outline" data-testid="onboarding-skip-invites" onClick={nextStep}>
                Skip for now
              </Button>
              <Button variant="ghost" onClick={prevStep}>
                Back
              </Button>
            </div>
          </div>
        )
      case 3:
        return (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="currency">Default currency</Label>
              <Input
                id="currency"
                data-testid="onboarding-currency"
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                placeholder="USD"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="timezone">Timezone</Label>
              <Input
                id="timezone"
                data-testid="onboarding-timezone"
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                placeholder="UTC"
              />
            </div>
            {error && <Alert variant="destructive">{error}</Alert>}
            <div className="flex items-center gap-2">
              <Button data-testid="onboarding-save-billing" onClick={handleBillingPrefs} disabled={loading}>
                {loading ? "Saving..." : "Continue"}
              </Button>
              <Button variant="outline" data-testid="onboarding-skip-billing" onClick={nextStep}>
                Skip
              </Button>
              <Button variant="ghost" onClick={prevStep}>
                Back
              </Button>
            </div>
          </div>
        )
      case 4:
      default:
        return (
          <div className="space-y-4 text-center">
            <Badge className="mx-auto w-fit">Step 4 of 4</Badge>
            <h2 className="text-2xl font-semibold">Your workspace is ready.</h2>
            {error && <Alert variant="destructive">{error}</Alert>}
            <Button className="w-full" data-testid="onboarding-finish" onClick={handleFinish}>
              Go to dashboard
            </Button>
          </div>
        )
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4">
      <Card className="w-full max-w-2xl">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>
                {currentStep === 1 && "Create your workspace"}
                {currentStep === 2 && "Invite your team"}
                {currentStep === 3 && "Billing preferences"}
                {currentStep === 4 && "All set"}
              </CardTitle>
              <CardDescription>
                {currentStep === 1 && "Your workspace is the boundary for billing and usage data."}
                {currentStep === 2 && "Billing works better with shared access."}
                {currentStep === 3 && "Set default behavior for invoices and usage."}
                {currentStep === 4 && "You can adjust settings anytime in the console."}
              </CardDescription>
            </div>
            <div className="text-sm text-text-muted">Step {currentStep} of 4</div>
          </div>
          <Separator className="mt-4" />
          <Progress value={progress} />
        </CardHeader>
        <CardContent>{renderStep()}</CardContent>
      </Card>
    </div>
  )
}
