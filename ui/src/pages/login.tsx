import { useState } from "react"
import { useNavigate } from "react-router-dom"

// import {auth} from "@/api/client"
import { LoginForm } from "@/components/login-form"
import { useAppMode } from "@/hooks/useAppMode"
import { useAuthStore } from "@/stores/authStore"

export default function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const mode = useAppMode()

  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (payload: { email: string; password: string }) => {
    setError(null)
    setIsLoading(true)
    try {
      await login(payload)
      const nextMustChangePassword = useAuthStore.getState().mustChangePassword
      navigate(nextMustChangePassword ? "/change-password" : "/orgs", { replace: true })
    } catch (err: any) {
      setError(err?.message ?? "Unable to sign in.")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40 px-4 py-12">
      <LoginForm onSubmit={handleSubmit} isLoading={isLoading} error={error} showSignup={mode === "cloud"} />
    </div>
  )
}
