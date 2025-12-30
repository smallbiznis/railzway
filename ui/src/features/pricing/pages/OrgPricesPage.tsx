import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"

import { admin } from "@/api/client"

export default function OrgPricesPage() {
  const { orgId } = useParams()
  const [prices, setPrices] = useState<unknown[]>([])
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
      .get("/prices")
      .then((response) => {
        if (!isMounted) return
        setPrices(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load prices.")
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
        <h1 className="text-2xl font-semibold">Prices</h1>
        <p className="text-text-muted text-sm">
          Review and manage price points for your products.
        </p>
      </div>
      {isLoading && (
        <div className="text-text-muted text-sm">Loading prices...</div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && (
        <pre className="bg-bg-subtle overflow-auto rounded-md p-4 text-xs">
          {JSON.stringify(prices, null, 2)}
        </pre>
      )}
    </div>
  )
}
