import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
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

type PriceAmount = {
  id: string
  price_id: string
  currency: string
  unit_amount_cents: number
  minimum_amount_cents?: number | null
  maximum_amount_cents?: number | null
  effective_from: string
  effective_to?: string | null
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

const getStatus = (effectiveFrom: string, effectiveTo?: string | null) => {
  const now = Date.now()
  const start = new Date(effectiveFrom).getTime()
  const end = effectiveTo ? new Date(effectiveTo).getTime() : null
  if (start > now) return "Upcoming"
  if (end && end <= now) return "Expired"
  return "Active"
}

const formatCurrency = (amount: number, currency: string) => {
  const safeCurrency = currency?.toUpperCase() || "USD"
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: safeCurrency,
    }).format(amount / 100)
  } catch {
    return `${(amount / 100).toFixed(2)} ${safeCurrency}`
  }
}

export default function OrgPriceAmountDetailPage() {
  const { orgId, amountId } = useParams()
  const [amount, setAmount] = useState<PriceAmount | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  useEffect(() => {
    if (!amountId) {
      setIsLoading(false)
      return
    }

    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/price_amounts/${amountId}`)
      .then((response) => {
        if (!active) return
        setAmount(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load price amount."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [amountId])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading price amount...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this price amount." />
  }

  if (!amount) {
    return <div className="text-text-muted text-sm">Price amount not found.</div>
  }

  const status = getStatus(amount.effective_from, amount.effective_to)

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to={`/orgs/${orgId}/price-amounts`}>Price amounts</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{amount.id}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold">Price amount</h1>
        <Badge variant={status === "Active" ? "secondary" : "outline"}>{status}</Badge>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Amount details</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm md:grid-cols-2">
          <div>
            <div className="text-text-muted">Price ID</div>
            <div className="font-medium">{amount.price_id}</div>
          </div>
          <div>
            <div className="text-text-muted">Currency</div>
            <div className="font-medium">{amount.currency?.toUpperCase()}</div>
          </div>
          <div>
            <div className="text-text-muted">Unit amount</div>
            <div className="font-medium">
              {formatCurrency(amount.unit_amount_cents, amount.currency)}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Minimum amount</div>
            <div className="font-medium">
              {amount.minimum_amount_cents == null
                ? "-"
                : formatCurrency(amount.minimum_amount_cents, amount.currency)}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Maximum amount</div>
            <div className="font-medium">
              {amount.maximum_amount_cents == null
                ? "-"
                : formatCurrency(amount.maximum_amount_cents, amount.currency)}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Effective period</div>
            <div className="font-medium">
              {formatDateTime(amount.effective_from)} → {formatDateTime(amount.effective_to)}
            </div>
          </div>
          <div>
            <div className="text-text-muted">Created</div>
            <div className="font-medium">{formatDateTime(amount.created_at)}</div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
