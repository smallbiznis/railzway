import { useState, type ComponentPropsWithoutRef } from "react"
import { Link } from "react-router-dom"

import { cn } from "@/lib/utils"
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
  FieldSeparator,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"

type LoginFormProps = {
  onSubmit: (payload: { email: string; password: string }) => Promise<void>
  isLoading?: boolean
  error?: string | null
  showSignup?: boolean
} & Omit<ComponentPropsWithoutRef<"div">, "onSubmit">

export function LoginForm({ className, onSubmit, isLoading, error, showSignup, ...props }: LoginFormProps) {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const canSignup = showSignup ?? true

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    await onSubmit({ email, password })
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card>
        <CardHeader className="text-center">
          <CardTitle className="text-xl">Welcome back</CardTitle>
          <CardDescription>
            Login
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <FieldGroup>
              {error && <Alert variant="destructive">{error}</Alert>}
              <FieldSeparator className="*:data-[slot=field-separator-content]:bg-bg-surface">
                Continue with email
              </FieldSeparator>
              <Field>
                <FieldLabel htmlFor="email">Email</FieldLabel>
                <Input
                  id="email"
                  data-testid="login-email"
                  type="email"
                  autoComplete="email"
                  placeholder="admin@valora.cloud"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </Field>
              <Field>
                <div className="flex items-center">
                  <FieldLabel htmlFor="password">Password</FieldLabel>
                  <a
                    href="#"
                    className="ml-auto text-sm underline-offset-4 hover:underline"
                  >
                    Forgot your password?
                  </a>
                </div>
                <Input
                  id="password"
                  data-testid="login-password"
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </Field>
              <Field>
                <Button type="submit" data-testid="login-submit" disabled={isLoading}>
                  {isLoading ? "Signing in..." : "Login"}
                </Button>
                <FieldDescription className="text-center" hidden={!canSignup}>
                  Don&apos;t have an account?{" "}
                  <Link to="/signup" className="text-accent-primary hover:underline">
                    Sign up
                  </Link>
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
      <FieldDescription className="px-6 text-center">
        By clicking continue, you agree to our <a href="#">Terms of Service</a>{" "}
        and <a href="#">Privacy Policy</a>.
      </FieldDescription>
    </div>
  )
}
