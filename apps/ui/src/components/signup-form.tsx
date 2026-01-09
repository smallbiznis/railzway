import { useState, type ComponentPropsWithoutRef } from "react"
import { Link } from "react-router-dom"

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

type SignupFormProps = {
  onSubmit: (payload: { email: string; password: string; displayName?: string }) => Promise<void>
  isLoading?: boolean
  error?: string | null
} & Omit<ComponentPropsWithoutRef<typeof Card>, "onSubmit">

export function SignupForm({
  onSubmit,
  isLoading,
  error,
  ...props
}: SignupFormProps) {
  const [email, setEmail] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [password, setPassword] = useState("")

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    await onSubmit({
      email,
      password,
      displayName: displayName.trim() ? displayName : undefined,
    })
  }

  return (
    <Card {...props}>
      <CardHeader>
        <CardTitle>Create an account</CardTitle>
        <CardDescription>
          Enter your information below to create your account
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <FieldGroup>
            {error && <Alert variant="destructive">{error}</Alert>}
            <Field>
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                placeholder="admin@valora.cloud"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <FieldDescription>
                Use a work email you can access.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="display-name">Display name</FieldLabel>
              <Input
                id="display-name"
                type="text"
                placeholder="Admin"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
              <FieldDescription>
                Optional. This will be shown in your workspace.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="password">Password</FieldLabel>
              <Input
                id="password"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <FieldDescription>Must be at least 8 characters long.</FieldDescription>
            </Field>
            <FieldGroup>
              <Field>
                <Button type="submit" disabled={isLoading}>
                  {isLoading ? "Creating..." : "Create Account"}
                </Button>
                <FieldDescription className="px-6 text-center">
                  Already have an account?{" "}
                  <Link to="/login" className="text-accent-primary hover:underline">
                    Sign in
                  </Link>
                </FieldDescription>
              </Field>
            </FieldGroup>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
