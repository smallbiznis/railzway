import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"

import { admin } from "@/api/client"

export default function OrgPricingsPage() {
  const { orgId } = useParams()
  const [pricings, setPricings] = useState<unknown[]>([])
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
      .get("/pricings")
      .then((response) => {
        if (!isMounted) return
        setPricings(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load pricings.")
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
        <h1 className="text-2xl font-semibold">Pricings</h1>
        <p className="text-text-muted text-sm">
          Manage pricing groups for this organization.
        </p>
      </div>
      {isLoading && (
        <div className="text-text-muted text-sm">Loading pricings...</div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && (
        <pre className="bg-bg-subtle overflow-auto rounded-md p-4 text-xs">
          {JSON.stringify(pricings, null, 2)}
        </pre>
      )}
    </div>
  )
}
