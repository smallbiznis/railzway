import { useMemo } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"

import { admin } from "@/api/client"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
  const response = await admin.get("/price_amounts", {
    params: {
      price_id: priceId,
      effective_from: "1970-01-01T00:00:00Z",
    },
  })
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

const statusBadgeStyles = (
  status: ReturnType<typeof getAmountStatus>
) => {
  switch (status) {
    case "Active":
      return {
        className: "border-status-success/30 bg-status-success/10 text-status-success",
        dotClassName: "bg-status-success",
      }
    case "Upcoming":
      return {
        className: "border-sky-200 bg-sky-50 text-sky-700",
        dotClassName: "bg-sky-500",
      }
    case "Expired":
    default:
      return {
        className: "border-border-subtle bg-bg-subtle text-text-muted",
        dotClassName: "bg-border-strong",
      }
  }
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
  const displayCurrency = useMemo(() => {
    const fromAmounts = amounts[0]?.currency
    if (fromAmounts) return fromAmounts.toUpperCase()
    const metadataCurrency = price?.metadata?.currency
    if (typeof metadataCurrency === "string" && metadataCurrency.trim()) {
      return metadataCurrency.trim().toUpperCase()
    }
    return "-"
  }, [amounts, price])

  const amountsWithStatus = useMemo(
    () =>
      amounts.map((amount) => ({
        amount,
        status: getAmountStatus(amount),
      })),
    [amounts]
  )

  const activeCount = useMemo(
    () =>
      amountsWithStatus.filter(({ status }) => status === "Active").length,
    [amountsWithStatus]
  )
  const hasUpcoming = useMemo(
    () =>
      amountsWithStatus.some(({ status }) => status === "Upcoming"),
    [amountsWithStatus]
  )
  const hasMultipleActive = activeCount > 1
  const hasActive = activeCount > 0

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
              onClick={() => navigate("/admin/prices")}
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
                <div className="text-text-muted text-xs">Currency</div>
                <div className="font-medium">{displayCurrency}</div>
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
                Active price is billed today. Scheduled price activates later.
                Expired price is retained for audit.
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
              Multiple active price versions detected. Only one active price
              amount should exist per price.
            </div>
          )}
          {!amountsLoading && !amountsError && !hasActive && hasUpcoming && (
            <Alert className="mb-3">
              <AlertTitle>Price not active yet</AlertTitle>
              <AlertDescription>
                Scheduled price versions exist, but no active price is in effect
                yet.
              </AlertDescription>
            </Alert>
          )}
          {!amountsLoading && !amountsError && amounts.length === 0 && (
            <div className="text-text-muted text-sm">
              No pricing defined.
            </div>
          )}
          {!amountsLoading && !amountsError && amounts.length > 0 && (
            <>
              {/* Pricing history is append-only; we never edit rows in-place. */}
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Amount</TableHead>
                    <TableHead>Currency</TableHead>
                    <TableHead>Effective from</TableHead>
                    <TableHead>Effective to</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created at</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {amountsWithStatus.map(({ amount, status }) => {
                    const currency = resolveCurrency(currencies, amount.currency)
                    const statusStyle = statusBadgeStyles(status)
                    return (
                      <TableRow key={amount.id}>
                        <TableCell>
                          {formatCurrencyAmount(amount, currency)}
                        </TableCell>
                        <TableCell className="font-medium">
                          {amount.currency?.toUpperCase() ?? "-"}
                        </TableCell>
                        <TableCell>
                          {formatDateTime(amount.effective_from)}
                        </TableCell>
                        <TableCell>
                          {amount.effective_to ? formatDateTime(amount.effective_to) : "—"}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="outline"
                            className={statusStyle.className}
                          >
                            <span
                              aria-hidden
                              className={`size-1.5 rounded-full ${statusStyle.dotClassName}`}
                            />
                            {status}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {formatDateTime(amount.created_at)}
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

      <Card>
        <CardHeader>
          <CardTitle>Add new price version</CardTitle>
          <CardDescription>
            Create a future effective price amount without editing history.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-between gap-4 flex-wrap">
          <div className="text-text-muted text-sm">
            Use this to schedule a new active price amount for this price.
          </div>
          {price && (
            <AddPriceAmountDialog
              priceId={priceId}
              priceName={price.name}
              currencies={currencies}
              priceAmounts={amounts}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
