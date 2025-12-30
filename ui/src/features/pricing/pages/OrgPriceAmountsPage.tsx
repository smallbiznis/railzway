import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"

import { admin } from "@/api/client"

export default function OrgPriceAmountsPage() {
  const { orgId } = useParams()
  const [priceAmounts, setPriceAmounts] = useState<unknown[]>([])
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
      .get("/price_amounts")
      .then((response) => {
        if (!isMounted) return
        setPriceAmounts(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load price amounts.")
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
        <h1 className="text-2xl font-semibold">Price Amounts</h1>
        <p className="text-text-muted text-sm">
          Review per-unit amounts for prices in this organization.
        </p>
      </div>
      {isLoading && (
        <div className="text-text-muted text-sm">
          Loading price amounts...
        </div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && (
        <pre className="bg-bg-subtle overflow-auto rounded-md p-4 text-xs">
          {JSON.stringify(priceAmounts, null, 2)}
        </pre>
      )}
    </div>
  )
}
