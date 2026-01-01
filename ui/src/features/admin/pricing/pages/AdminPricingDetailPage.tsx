import { useMemo } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"

import { admin } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

import { AddPriceAmountDialog } from "@/features/admin/pricing/components/AddPriceAmountDialog"
import type { Currency, Price, PriceAmount } from "@/features/admin/pricing/types"
import {
  formatCurrencyAmount,
  formatDateTime,
  formatPricingModel,
  formatUnit,
  parseDate,
  resolveCurrency,
} from "@/features/admin/pricing/utils"

const fetchPrice = async (priceId: string) => {
  const response = await admin.get(`/prices/${priceId}`)
  return response.data?.data as Price
}

const fetchPriceAmounts = async (priceId: string) => {
  const response = await admin.get(`/prices/${priceId}/amounts`)
  const payload = response.data?.data
  if (Array.isArray(payload)) {
    return payload as PriceAmount[]
  }
  if (payload && typeof payload === "object") {
    const list =
      (payload as { amounts?: PriceAmount[] }).amounts ??
      (payload as { price_amounts?: PriceAmount[] }).price_amounts
    return Array.isArray(list) ? list : []
  }
  return []
}

const fetchCurrencies = async () => {
  const response = await admin.get("/currencies")
  const payload = response.data?.data
  return Array.isArray(payload) ? (payload as Currency[]) : []
}

const getAmountStatus = (amount: PriceAmount) => {
  const now = new Date()
  const effectiveFrom = parseDate(amount.effective_from)
  const effectiveTo = parseDate(amount.effective_to)
  if (effectiveTo && effectiveTo.getTime() <= now.getTime()) {
    return "Expired"
  }
  if (effectiveFrom && effectiveFrom.getTime() > now.getTime()) {
    return "Scheduled"
  }
  if (!amount.effective_to && effectiveFrom) {
    return "Active"
  }
  return "Unknown"
}

export default function AdminPricingDetailPage() {
  const navigate = useNavigate()
  const { priceId } = useParams<{ priceId: string }>()
  const {
    data: price,
    isLoading: priceLoading,
    error: priceError,
  } = useQuery({
    queryKey: ["price", priceId],
    queryFn: () => fetchPrice(priceId ?? ""),
    enabled: Boolean(priceId),
  })
  const {
    data: amountsData,
    isLoading: amountsLoading,
    error: amountsError,
  } = useQuery({
    queryKey: ["price_amounts", priceId],
    queryFn: () => fetchPriceAmounts(priceId ?? ""),
    enabled: Boolean(priceId),
  })
  const {
    data: currenciesData,
    isLoading: currenciesLoading,
  } = useQuery({
    queryKey: ["currencies"],
    queryFn: fetchCurrencies,
  })

  const amounts = useMemo(() => {
    const list = amountsData ?? []
    return [...list].sort((a, b) => {
      const aTime = parseDate(a.effective_from)?.getTime() ?? 0
      const bTime = parseDate(b.effective_from)?.getTime() ?? 0
      return bTime - aTime
    })
  }, [amountsData])

  const currencies = useMemo(() => currenciesData ?? [], [currenciesData])
  const activeByCurrency = useMemo(() => {
    return amounts.reduce<Record<string, number>>((acc, amount) => {
      const status = getAmountStatus(amount)
      if (status !== "Active") return acc
      const code = amount.currency?.toUpperCase() ?? "UNKNOWN"
      acc[code] = (acc[code] ?? 0) + 1
      return acc
    }, {})
  }, [amounts])

  const hasMultipleActive = Object.values(activeByCurrency).some(
    (count) => count > 1
  )

  if (!priceId) {
    return (
      <div className="text-text-muted text-sm">
        Select a price to view details.
      </div>
    )
  }

  return (
    <div className="space-y-6 px-4 py-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-text-muted text-sm">
            <button
              type="button"
              className="text-text-muted hover:text-text-primary transition-colors"
              onClick={() => navigate("/admin/pricing")}
            >
              Pricing
            </button>
            <span>/</span>
            <span className="text-text-primary">
              {price?.name ?? price?.code ?? "Price detail"}
            </span>
          </div>
          <h1 className="text-2xl font-semibold">
            {price?.name ?? "Price"}{" "}
            <span className="text-text-muted text-base font-normal">
              {price?.code ? `(${price.code})` : ""}
            </span>
          </h1>
        </div>
        {price && (
          <AddPriceAmountDialog
            priceId={priceId}
            priceName={price.name}
            currencies={currencies}
            priceAmounts={amounts}
          />
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Metadata</CardTitle>
          <CardDescription>Read-only pricing attributes.</CardDescription>
        </CardHeader>
        <CardContent>
          {priceLoading && (
            <div className="flex items-center gap-2 text-text-muted text-sm">
              <Spinner />
              Loading price
            </div>
          )}
          {priceError && (
            <div className="text-status-error text-sm">
              {(priceError as Error)?.message ?? "Unable to load price."}
            </div>
          )}
          {price && (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <div>
                <div className="text-text-muted text-xs">Pricing model</div>
                <div className="font-medium">
                  {formatPricingModel(price.pricing_model)}
                </div>
              </div>
              <div>
                <div className="text-text-muted text-xs">Unit</div>
                <div className="font-medium">{formatUnit(price.billing_unit)}</div>
              </div>
              <div>
                <div className="text-text-muted text-xs">Interval</div>
                <div className="font-medium">
                  {price.billing_interval
                    ? `${price.billing_interval_count ?? 1} ${price.billing_interval.toLowerCase()}`
                    : "-"}
                </div>
              </div>
              <div>
                <div className="text-text-muted text-xs">Created</div>
                <div className="font-medium">{formatDateTime(price.created_at)}</div>
              </div>
              <div>
                <div className="text-text-muted text-xs">Price ID</div>
                <div className="font-medium">{price.id}</div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle>Pricing history</CardTitle>
              <CardDescription>
                Historical price amounts are immutable and displayed in full.
              </CardDescription>
            </div>
            {currenciesLoading && (
              <div className="flex items-center gap-2 text-text-muted text-sm">
                <Spinner />
                Loading currencies
              </div>
            )}
          </div>
          <Separator />
        </CardHeader>
        <CardContent>
          {amountsLoading && (
            <div className="flex items-center gap-2 text-text-muted text-sm">
              <Spinner />
              Loading price history
            </div>
          )}
          {amountsError && (
            <div className="text-status-error text-sm">
              {(amountsError as Error)?.message ?? "Unable to load price history."}
            </div>
          )}
          {hasMultipleActive && (
            <div className="text-status-error text-sm mb-3">
              Multiple active amounts detected for a currency. Only one active
              version should exist per currency.
            </div>
          )}
          {!amountsLoading && !amountsError && amounts.length === 0 && (
            <div className="text-text-muted text-sm">
              No price amounts yet. Add a new price version to begin tracking.
            </div>
          )}
          {!amountsLoading && !amountsError && amounts.length > 0 && (
            <>
              {/* Pricing history is append-only; we never edit rows in-place. */}
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Currency</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Effective from</TableHead>
                    <TableHead>Effective to</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {amounts.map((amount) => {
                    const status = getAmountStatus(amount)
                    const currency = resolveCurrency(currencies, amount.currency)
                    const statusVariant =
                      status === "Active"
                        ? "default"
                        : status === "Scheduled"
                          ? "secondary"
                          : "outline"
                    return (
                      <TableRow key={amount.id}>
                        <TableCell className="font-medium">
                          {amount.currency?.toUpperCase() ?? "-"}
                        </TableCell>
                        <TableCell>
                          {formatCurrencyAmount(amount, currency)}
                        </TableCell>
                        <TableCell>
                          {formatDateTime(amount.effective_from)}
                        </TableCell>
                        <TableCell>
                          {formatDateTime(amount.effective_to)}
                        </TableCell>
                        <TableCell>
                          <Badge variant={statusVariant}>{status}</Badge>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
