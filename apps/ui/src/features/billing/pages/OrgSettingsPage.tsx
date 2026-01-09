import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"

import { admin } from "@/api/client"

export default function OrgSettingsPage() {
  const { orgId } = useParams()
  const [settings, setSettings] = useState<Record<string, unknown> | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)

    admin
      .get(`/orgs/${orgId}/settings`)
      .then((response) => {
        if (!isMounted) return
        setSettings(response.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load settings.")
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold">Settings</h1>
        <p className="text-text-muted text-sm">
          Configure organization-level billing preferences.
        </p>
      </div>
      {isLoading && (
        <div className="text-text-muted text-sm">Loading settings...</div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && (
        <pre className="bg-bg-subtle overflow-auto rounded-md p-4 text-xs">
          {JSON.stringify(settings, null, 2)}
        </pre>
      )}
    </div>
  )
}
