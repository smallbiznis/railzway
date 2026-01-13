import { useState } from "react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { inviteMembers } from "@/api/organization"
import { useOrgStore } from "@/stores/orgStore"
import type { Role } from "@/types/organization"

interface InviteMemberDialogProps {
  onSuccess?: () => void
}

export function InviteMemberDialog({ onSuccess }: InviteMemberDialogProps) {
  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<Role>("MEMBER")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const orgId = useOrgStore((s) => s.currentOrg?.id)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!orgId) {
      setError("No organization selected")
      return
    }

    setError(null)
    setLoading(true)

    try {
      await inviteMembers(orgId, [{ email, role }])
      setOpen(false)
      setEmail("")
      setRole("MEMBER")
      onSuccess?.()
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || "Failed to send invite")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>Invite Member</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[425px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Invite Team Member</DialogTitle>
            <DialogDescription>
              Send an invitation to join your organization. They'll receive an email with a link to
              accept.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="email">Email address</Label>
              <Input
                id="email"
                type="email"
                placeholder="colleague@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="role">Role</Label>
              <Select value={role} onValueChange={(value) => setRole(value as Role)}>
                <SelectTrigger id="role">
                  <SelectValue placeholder="Select a role" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="OWNER">Owner - Full access</SelectItem>
                  <SelectItem value="ADMIN">Admin - Manage settings</SelectItem>
                  <SelectItem value="FINOPS">FinOps - View billing & reports</SelectItem>
                  <SelectItem value="DEVELOPER">Developer - Manage API keys</SelectItem>
                  <SelectItem value="MEMBER">Member - Read-only access</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {error && <Alert variant="destructive">{error}</Alert>}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "Sending..." : "Send Invite"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
