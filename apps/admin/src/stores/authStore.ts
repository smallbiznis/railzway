import { create } from "zustand"
import { persist } from "zustand/middleware"

import { auth } from "@/api/client"

type User = {
  id: string
  username: string
  email?: string
  displayName?: string
  provider?: string
  externalId?: string
}

type AuthState = {
  user: User | null
  isAuthenticated: boolean
  mustChangePassword: boolean
  login: (payload: { email: string; password: string }) => Promise<void>
  signup: (payload: { email: string; password: string; displayName?: string; orgName?: string }) => Promise<void>
  completeInvite: (inviteId: string, payload: { password: string; name: string; username?: string }) => Promise<void>
  logout: () => Promise<void>
  setMustChangePassword: (value: boolean) => void
}

const readAppMode = (): "oss" | "cloud" => {
  const raw = import.meta.env.VITE_APP_MODE
  if (raw === "oss" || raw === "cloud") {
    return raw
  }
  return "oss"
}

const resolveMustChangePassword = (payload: any): boolean => {
  if (readAppMode() !== "cloud") {
    return false
  }

  const metadata = payload?.metadata ?? payload?.session?.metadata ?? payload
  if (!metadata || typeof metadata !== "object") {
    return false
  }

  const mustChange = metadata.must_change_password ?? metadata.mustChangePassword
  if (typeof mustChange === "boolean") {
    return mustChange
  }
  if (typeof mustChange === "string") {
    if (mustChange.toLowerCase() === "true") return true
    if (mustChange.toLowerCase() === "false") return false
  }

  const passwordState = metadata.password_state ?? metadata.passwordState
  if (typeof passwordState === "string") {
    return passwordState.toLowerCase() === "default"
  }

  const isDefault = metadata.is_default ?? metadata.isDefault
  if (typeof isDefault === "boolean") {
    return isDefault
  }
  if (typeof isDefault === "string") {
    return isDefault.toLowerCase() === "true"
  }

  return false
}

const buildUser = (payload: any): User | null => {
  const metadata = payload?.metadata ?? payload
  if (!metadata) {
    return null
  }
  const id = metadata.user_id ?? metadata.id
  if (!id) {
    return null
  }
  const email = metadata.email
  const displayName = metadata.display_name ?? metadata.username ?? email
  const username = displayName ?? email ?? String(id)

  return {
    id: String(id),
    username,
    email,
    displayName,
    provider: metadata.provider ?? metadata.auth_provider,
    externalId: metadata.external_id,
  }
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      isAuthenticated: false,
      mustChangePassword: false,
      login: async (payload) => {
        try {
          const email = payload.email.trim().toLowerCase()
          const res = await auth.post("/login", {
            email,
            password: payload.password,
          })
          const user = buildUser(res.data)
          if (!user) {
            throw new Error("invalid_login_response")
          }
          const mustChangePassword = resolveMustChangePassword(res.data)
          set({ user, isAuthenticated: true, mustChangePassword })
        } catch (err) {
          set({ user: null, isAuthenticated: false, mustChangePassword: false })
          throw err
        }
      },
      signup: async (payload) => {
        try {
          const displayName = payload.displayName?.trim()
          const email = payload.email.trim().toLowerCase()
          const username =
            displayName ||
            email.split("@")[0] ||
            "user"
          const body: Record<string, string> = {
            username,
            email,
            password: payload.password,
          }
          if (payload.orgName?.trim()) {
            body.org_name = payload.orgName.trim()
          }
          const res = await auth.post("/signup", body)
          const user = buildUser(res.data)
          if (!user) {
            throw new Error("invalid_signup_response")
          }
          const mustChangePassword = resolveMustChangePassword(res.data)
          set({ user, isAuthenticated: true, mustChangePassword })
        } catch (err) {
          set({ user: null, isAuthenticated: false, mustChangePassword: false })
          throw err
        }
      },
      completeInvite: async (inviteId: string, payload: { password: string; name: string; username?: string }) => {
        try {
          const res = await auth.post(`/invites/${inviteId}/complete`, payload)
          const user = buildUser(res.data)
          if (!user) {
            throw new Error("invalid_invite_response")
          }
          const mustChangePassword = resolveMustChangePassword(res.data)
          set({ user, isAuthenticated: true, mustChangePassword })
        } catch (err) {
          set({ user: null, isAuthenticated: false, mustChangePassword: false })
          throw err
        }
      },
      logout: async () => {
        try {
          await auth.post("/logout")
        } catch (err) {
          console.warn("logout failed", err)
        }
        set({ user: null, isAuthenticated: false, mustChangePassword: false })
      },
      setMustChangePassword: (value) => set({ mustChangePassword: value }),
    }),
    {
      name: "railzway-auth",
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
        mustChangePassword: state.mustChangePassword,
      }),
    }
  )
)
