import { useEffect, useMemo, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { PricingNav } from "@/features/pricing/components/PricingNav"

type PriceAmount = {
  id: string
  price_id: string
  currency: string
  unit_amount_cents: number
  minimum_amount_cents?: number | null
  maximum_amount_cents?: number | null
  effective_from: string
  effective_to?: string | null
}

type Price = {
  id: string
  name?: string
  code?: string
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

const formatDate = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
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

export default function OrgPriceAmountsPage() {
  const { orgId } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [priceAmounts, setPriceAmounts] = useState<PriceAmount[]>([])
  const [prices, setPrices] = useState<Price[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const priceIdFilter = searchParams.get("price_id") ?? ""

  useEffect(() => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    Promise.all([
      admin.get("/price_amounts", {
        params: {
          price_id: priceIdFilter || undefined,
        },
      }),
      admin.get("/prices"),
    ])
      .then(([amountRes, priceRes]) => {
        if (!isMounted) return
        setPriceAmounts(amountRes.data?.data ?? [])
        setPrices(priceRes.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load price amounts."))
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, priceIdFilter])

  const priceLookup = useMemo(() => {
    const map = new Map<string, Price>()
    prices.forEach((price) => {
      map.set(price.id, price)
    })
    return map
  }, [prices])

  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading price amounts...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to price amounts." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Price amounts</h1>
          <p className="text-text-muted text-sm">
            Append-only rate history with effective periods.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild size="sm">
            <Link to={`${orgBasePath}/prices`}>Add price amount</Link>
          </Button>
        </div>
      </div>

      <PricingNav />

      <div className="flex flex-wrap items-center gap-3">
        <Input
          className="w-full max-w-md"
          placeholder="Filter by price ID"
          value={priceIdFilter}
          onChange={(event) => {
            const next = new URLSearchParams(searchParams)
            const value = event.target.value.trim()
            if (value) {
              next.set("price_id", value)
            } else {
              next.delete("price_id")
            }
            setSearchParams(next, { replace: true })
          }}
        />
      </div>

      {priceAmounts.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No price amounts yet</EmptyTitle>
            <EmptyDescription>
              Add amounts from a price detail page to start billing.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow>
                <TableHead>Price</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Effective</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {priceAmounts.map((amount) => {
                const price = priceLookup.get(amount.price_id)
                const status = getStatus(amount.effective_from, amount.effective_to)
                return (
                  <TableRow key={amount.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="font-medium">{price?.name || amount.price_id}</span>
                        <span className="text-text-muted text-xs">{price?.code || amount.price_id}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      {formatCurrency(amount.unit_amount_cents, amount.currency)}
                    </TableCell>
                    <TableCell>
                      {formatDate(amount.effective_from)} → {formatDate(amount.effective_to)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={status === "Active" ? "secondary" : "outline"}>
                        {status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button asChild variant="ghost" size="sm">
                        <Link to={`${orgBasePath}/price-amounts/${amount.id}`}>View</Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}
