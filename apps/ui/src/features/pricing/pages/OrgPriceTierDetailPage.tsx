import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

type PriceTier = {
  id: string
  price_id: string
  tier_mode: number
  start_quantity: number
  end_quantity?: number | null
  unit_amount_cents?: number | null
  flat_amount_cents?: number | null
  unit: string
  created_at?: string
}

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

const formatRange = (start: number, end?: number | null) => {
  if (end === null || end === undefined) return `${start}+`
  return `${start} - ${end}`
}

export default function OrgPriceTierDetailPage() {
  const { orgId, tierId } = useParams()
  const [tier, setTier] = useState<PriceTier | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  useEffect(() => {
    if (!tierId) {
      setIsLoading(false)
      return
    }

    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/price_tiers/${tierId}`)
      .then((response) => {
        if (!active) return
        setTier(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load price tier."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [tierId])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading price tier...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this price tier." />
  }

  if (!tier) {
    return <div className="text-text-muted text-sm">Price tier not found.</div>
  }

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to={`/orgs/${orgId}/price-tiers`}>Price tiers</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{tier.id}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Price tier</h1>
        <p className="text-text-muted text-sm">Tier configuration details.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Tier details</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm md:grid-cols-2">
          <div>
            <div className="text-text-muted">Price ID</div>
            <div className="font-medium">{tier.price_id}</div>
          </div>
          <div>
            <div className="text-text-muted">Tier mode</div>
            <div className="font-medium">Mode {tier.tier_mode}</div>
          </div>
          <div>
            <div className="text-text-muted">Range</div>
            <div className="font-medium">
              {formatRange(tier.start_quantity, tier.end_quantity)}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Unit</div>
            <div className="font-medium">{tier.unit}</div>
          </div>
          <div>
            <div className="text-text-muted">Unit amount (cents)</div>
            <div className="font-medium">
              {tier.unit_amount_cents == null ? "-" : `${tier.unit_amount_cents}¢`}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Flat amount (cents)</div>
            <div className="font-medium">
              {tier.flat_amount_cents == null ? "-" : `${tier.flat_amount_cents}¢`}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Created</div>
            <div className="font-medium">{formatDateTime(tier.created_at)}</div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
