import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Currency, Price, PriceAmount } from "@/features/admin/pricing/types"
import {
  formatCurrencyAmount,
  formatDateTime,
  formatPricingModel,
  formatUnit,
  parseDate,
  resolveCurrency,
} from "@/features/admin/pricing/utils"
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
}

const fetchPricing = async (pricingId: string) => {
  const response = await admin.get(`/pricings/${pricingId}`)
  return response.data?.data as Price
}

const fetchPriceAmounts = async (pricingId: string) => {
  const response = await admin.get("/price_amounts", {
    params: {
      price_id: pricingId,
      effective_from: "1970-01-01T00:00:00Z",
    },
  })
  const payload = response.data?.data
  if (Array.isArray(payload)) {
    return payload as PriceAmount[]
  }
  return []
}

const fetchCurrencies = async () => {
  const response = await admin.get("/currencies")
  const payload = response.data?.data
  return Array.isArray(payload) ? (payload as Currency[]) : []
}

const fetchPriceTiers = async () => {
  const response = await admin.get("/price_tiers")
  const payload = response.data?.data
  return Array.isArray(payload) ? (payload as PriceTier[]) : []
}

type AmountStatus = "Active" | "Expired" | "Upcoming"

const getAmountStatus = (amount: PriceAmount): AmountStatus => {
  const now = new Date()
  const effectiveFrom = parseDate(amount.effective_from)
  const effectiveTo = parseDate(amount.effective_to)
  if (effectiveFrom && effectiveFrom.getTime() > now.getTime()) {
    return "Upcoming"
  }
  if (effectiveTo && effectiveTo.getTime() <= now.getTime()) {
    return "Expired"
  }
  if (effectiveFrom && effectiveFrom.getTime() <= now.getTime()) {
    return "Active"
  }
  return "Upcoming"
}

const formatTierRange = (start: number, end?: number | null) => {
  if (end === null || end === undefined) return `${start}+`
  return `${start} - ${end}`
}

export default function OrgPricingDetailPage() {
  const navigate = useNavigate()
  const { orgId, pricingId } = useParams<{ orgId: string; pricingId: string }>()
  const [pricing, setPricing] = useState<Price | null>(null)
  const [amounts, setAmounts] = useState<PriceAmount[]>([])
  const [tiers, setTiers] = useState<PriceTier[]>([])
  const [currencies, setCurrencies] = useState<Currency[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  useEffect(() => {
    if (!pricingId) return
    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    Promise.all([
      fetchPricing(pricingId),
      fetchPriceAmounts(pricingId),
      fetchCurrencies(),
      fetchPriceTiers(),
    ])
      .then(([pricingData, amountsData, currenciesData, tiersData]) => {
        if (!active) return
        setPricing(pricingData)
        setAmounts(amountsData)
        setCurrencies(currenciesData)
        setTiers(tiersData.filter((tier) => tier.price_id === pricingId))
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load pricing model."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [pricingId])

  const amountsWithStatus = useMemo(
    () =>
      amounts.map((amount) => ({
        amount,
        status: getAmountStatus(amount),
      })),
    [amounts]
  )

  const displayCurrency = useMemo(() => {
    const fromAmounts = amounts[0]?.currency
    if (fromAmounts) return fromAmounts.toUpperCase()
    const metadataCurrency = pricing?.metadata?.currency
    if (typeof metadataCurrency === "string" && metadataCurrency.trim()) {
      return metadataCurrency.trim().toUpperCase()
    }
    return "-"
  }, [amounts, pricing])

  const activeCount = useMemo(
    () => amountsWithStatus.filter(({ status }) => status === "Active").length,
    [amountsWithStatus]
  )

  if (!pricingId) {
    return (
      <div className="text-text-muted text-sm">
        Select a pricing model to view details.
      </div>
    )
  }

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading pricing model...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this pricing model." />
  }

  if (!pricing) {
    return <div className="text-text-muted text-sm">Pricing model not found.</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-text-muted text-sm">
            <button
              type="button"
              className="text-text-muted hover:text-text-primary transition-colors"
              onClick={() => navigate(`/orgs/${orgId}/pricings`)}
            >
              Pricing models
            </button>
            <span>/</span>
            <span className="text-text-primary">
              {pricing.name ?? pricing.code ?? "Pricing detail"}
            </span>
          </div>
          <h1 className="text-2xl font-semibold">
            {pricing.name ?? "Pricing model"}{" "}
            <span className="text-text-muted text-base font-normal">
              {pricing.code ? `(${pricing.code})` : ""}
            </span>
          </h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="outline" size="sm">
            <Link to={`/orgs/${orgId}/prices/${pricingId}`}>View price</Link>
          </Button>
          <Button asChild variant="outline" size="sm">
            <Link to={`/orgs/${orgId}/price-amounts`}>Manage amounts</Link>
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Metadata</CardTitle>
          <CardDescription>Read-only pricing attributes.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <div className="text-text-muted text-xs">Pricing model</div>
              <div className="font-medium">
                {formatPricingModel(pricing.pricing_model)}
              </div>
            </div>
            <div>
              <div className="text-text-muted text-xs">Unit</div>
              <div className="font-medium">{formatUnit(pricing.billing_unit)}</div>
            </div>
            <div>
              <div className="text-text-muted text-xs">Interval</div>
              <div className="font-medium">
                {pricing.billing_interval
                  ? `${pricing.billing_interval_count ?? 1} ${pricing.billing_interval.toLowerCase()}`
                  : "-"}
              </div>
            </div>
            <div>
              <div className="text-text-muted text-xs">Currency</div>
              <div className="font-medium">{displayCurrency}</div>
            </div>
            <div>
              <div className="text-text-muted text-xs">Created</div>
              <div className="font-medium">{formatDateTime(pricing.created_at)}</div>
            </div>
            <div>
              <div className="text-text-muted text-xs">Pricing ID</div>
              <div className="font-medium break-all">{pricing.id}</div>
            </div>
          </div>
        </CardContent>
      </Card>

      {activeCount > 1 && (
        <Card className="border-status-error/40 bg-status-error/5">
          <CardContent className="pt-6">
            <div className="text-sm text-status-error">
              Multiple active price amounts detected. Only one active amount should be effective at a time.
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Price amount history</CardTitle>
          <CardDescription>Append-only pricing records with effective periods.</CardDescription>
        </CardHeader>
        <CardContent>
          {amounts.length === 0 ? (
            <div className="text-text-muted text-sm">No price amounts yet.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Currency</TableHead>
                  <TableHead>Amount</TableHead>
                  <TableHead>Effective period</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {amountsWithStatus.map(({ amount, status }) => {
                  const currency = resolveCurrency(currencies, amount.currency)
                  return (
                    <TableRow key={amount.id}>
                      <TableCell>{amount.currency?.toUpperCase()}</TableCell>
                      <TableCell>{formatCurrencyAmount(amount, currency)}</TableCell>
                      <TableCell>
                        {formatDateTime(amount.effective_from)} →{" "}
                        {formatDateTime(amount.effective_to)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={status === "Active" ? "secondary" : "outline"}>
                          {status}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Tier configuration</CardTitle>
          <CardDescription>Slabs and ranges for tiered pricing models.</CardDescription>
        </CardHeader>
        <CardContent>
          {tiers.length === 0 ? (
            <div className="text-text-muted text-sm">No tiers configured.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Range</TableHead>
                  <TableHead>Unit amount</TableHead>
                  <TableHead>Flat amount</TableHead>
                  <TableHead>Mode</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tiers.map((tier) => (
                  <TableRow key={tier.id}>
                    <TableCell>{formatTierRange(tier.start_quantity, tier.end_quantity)}</TableCell>
                    <TableCell>
                      {tier.unit_amount_cents === null || tier.unit_amount_cents === undefined
                        ? "-"
                        : `${tier.unit_amount_cents}¢`}
                    </TableCell>
                    <TableCell>
                      {tier.flat_amount_cents === null || tier.flat_amount_cents === undefined
                        ? "-"
                        : `${tier.flat_amount_cents}¢`}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">Mode {tier.tier_mode}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
