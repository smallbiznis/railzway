import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"

import { auth } from "@/api/client"
import { Alert } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useAuthStore } from "@/stores/authStore"

const passwordRules = [
  {
    label: "At least 8 characters",
    test: (value: string) => value.length >= 8,
  },
  {
    label: "One uppercase letter",
    test: (value: string) => /[A-Z]/.test(value),
  },
  {
    label: "One lowercase letter",
    test: (value: string) => /[a-z]/.test(value),
  },
  {
    label: "One number",
    test: (value: string) => /[0-9]/.test(value),
  },
  {
    label: "One symbol",
    test: (value: string) => /[^A-Za-z0-9]/.test(value),
  },
]

export default function ChangePasswordPage() {
  const navigate = useNavigate()
  const mustChangePassword = useAuthStore((s) => s.mustChangePassword)
  const setMustChangePassword = useAuthStore((s) => s.setMustChangePassword)

  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  useEffect(() => {
    if (!mustChangePassword) {
      navigate("/orgs", { replace: true })
    }
  }, [mustChangePassword, navigate])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setError(null)

    if (!currentPassword.trim()) {
      setError("Current password is required.")
      return
    }
    if (!newPassword.trim()) {
      setError("New password is required.")
      return
    }
    if (newPassword !== confirmPassword) {
      setError("New passwords do not match.")
      return
    }
    if (newPassword === currentPassword) {
      setError("New password must be different from current password.")
      return
    }

    const failedRules = passwordRules.filter((rule) => !rule.test(newPassword))
    if (failedRules.length > 0) {
      setError(`Password must include: ${failedRules.map((rule) => rule.label).join(", ")}.`)
      return
    }

    setIsLoading(true)
    try {
      await auth.post("/change-password", {
        current_password: currentPassword,
        new_password: newPassword,
      })
      setMustChangePassword(false)
      navigate("/", { replace: true })
    } catch (err: any) {
      const message =
        err?.response?.data?.error?.message ??
        err?.message ??
        "Unable to change password."
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4 py-12">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle>Change your password</CardTitle>
          <CardDescription>
            You must update your password before continuing.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <FieldGroup>
              {error && <Alert variant="destructive">{error}</Alert>}
              <Field>
                <FieldLabel htmlFor="current-password">Current password</FieldLabel>
                <Input
                  id="current-password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={currentPassword}
                  onChange={(event) => setCurrentPassword(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="new-password">New password</FieldLabel>
                <Input
                  id="new-password"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                />
                <FieldDescription>
                  Use a strong password that you do not reuse elsewhere.
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="confirm-password">Confirm new password</FieldLabel>
                <Input
                  id="confirm-password"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                />
              </Field>
              <div className="text-xs text-text-muted">
                <div className="font-medium text-text-muted">Password must include:</div>
                <ul className="list-disc space-y-1 pl-4">
                  {passwordRules.map((rule) => (
                    <li key={rule.label}>{rule.label}</li>
                  ))}
                </ul>
              </div>
              <Field>
                <Button type="submit" disabled={isLoading}>
                  {isLoading ? "Updating..." : "Update password"}
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
