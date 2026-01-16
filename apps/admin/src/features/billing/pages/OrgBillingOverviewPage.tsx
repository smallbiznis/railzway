import { Download } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { Area, AreaChart, Bar, BarChart, CartesianGrid, Line, XAxis, YAxis } from "recharts"
import type { DateRange } from "react-day-picker"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { cn } from "@/lib/utils"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

type SeriesPoint = {
  period: string
  value: number
}

type MrrResponse = {
  currency: string
  current?: number | null
  previous?: number | null
  growth_amount?: number | null
  growth_rate?: number | null
  series: SeriesPoint[]
  compare_series?: SeriesPoint[]
  has_data: boolean
}

type MrrMovementResponse = {
  currency: string
  new_mrr: number
  expansion_mrr: number
  contraction_mrr: number
  churned_mrr: number
  net_mrr_change: number
  has_data: boolean
}

type RevenueResponse = {
  currency: string
  total?: number | null
  previous?: number | null
  growth_amount?: number | null
  growth_rate?: number | null
  series: SeriesPoint[]
  compare_series?: SeriesPoint[]
  has_data: boolean
}

type OutstandingBalanceResponse = {
  currency: string
  outstanding: number
  overdue: number
  has_data: boolean
}

type CollectionRateResponse = {
  currency: string
  collection_rate?: number | null
  collected_amount: number
  invoiced_amount: number
  has_data: boolean
}

type SubscribersResponse = {
  current?: number | null
  previous?: number | null
  growth_amount?: number | null
  growth_rate?: number | null
  churn_rate?: number | null
  series: SeriesPoint[]
  compare_series?: SeriesPoint[]
  has_data: boolean
}

type RangePreset = "7d" | "30d" | "90d" | "custom"

const rangePresets: Array<{ value: RangePreset; label: string }> = [
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "custom", label: "Custom range" },
]

const granularityOptions = [
  { value: "day", label: "Daily" },
  { value: "month", label: "Monthly" },
]



const revenueChartConfig = {
  current: {
    label: "Revenue",
    color: "hsl(var(--accent-primary))",
  },
  previous: {
    label: "Previous",
    color: "hsl(var(--border-strong))",
  },
} satisfies ChartConfig

const subscriberChartConfig = {
  current: {
    label: "Subscribers",
    color: "hsl(var(--accent-primary))",
  },
  previous: {
    label: "Previous",
    color: "hsl(var(--border-strong))",
  },
} satisfies ChartConfig

const formatDateOnly = (value: Date) => {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, "0")
  const day = String(value.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

const formatRangeLabel = (range?: DateRange) => {
  if (!range?.from || !range?.to) return "Select dates"
  const formatter = new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  })
  return `${formatter.format(range.from)} - ${formatter.format(range.to)}`
}

const formatAxisLabel = (value: string, granularity: string) => {
  if (!value) return ""
  const date = new Date(granularity === "month" ? `${value}-01` : value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: granularity === "day" ? "2-digit" : undefined,
  }).format(date)
}

const formatCurrency = (amount: number | null | undefined, currency: string) => {
  if (amount === null || amount === undefined) return "No data"
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency,
    }).format(amount / 100)
  } catch {
    return `${(amount / 100).toFixed(2)} ${currency}`
  }
}

const formatCurrencyCompact = (amount: number | null | undefined, currency: string) => {
  if (amount === null || amount === undefined) return ""
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency,
      notation: "compact",
      maximumFractionDigits: 1,
    }).format(amount / 100)
  } catch {
    return `${(amount / 100).toFixed(0)} ${currency}`
  }
}

const formatSignedCurrency = (amount: number | null | undefined, currency: string) => {
  if (amount === null || amount === undefined) return "No data"
  const sign = amount < 0 ? "-" : "+"
  const value = formatCurrency(Math.abs(amount), currency)
  return `${sign}${value}`
}

const formatCount = (value: number | null | undefined) => {
  if (value === null || value === undefined) return "No data"
  return new Intl.NumberFormat("en-US", { notation: "compact" }).format(value)
}

const formatPercent = (value: number | null | undefined) => {
  if (value === null || value === undefined) return "No data"
  return new Intl.NumberFormat("en-US", {
    style: "percent",
    maximumFractionDigits: 1,
  }).format(value)
}

const formatPercentCompact = (value: number | null | undefined) => {
  if (value === null || value === undefined) return "No data"
  return new Intl.NumberFormat("en-US", {
    style: "percent",
    maximumFractionDigits: 1,
  }).format(value)
}

const buildDeltaLabel = (
  amount: number | null | undefined,
  rate: number | null | undefined,
  currency?: string
) => {
  if (amount === null || amount === undefined || rate === null || rate === undefined) {
    return "No comparison"
  }
  const direction = amount >= 0 ? "+" : "-"
  const absAmount = Math.abs(amount)
  const amountLabel = currency ? formatCurrency(absAmount, currency) : formatCount(absAmount)
  const rateLabel = formatPercent(Math.abs(rate))
  return `${direction}${amountLabel} (${direction}${rateLabel})`
}

type MetricCardProps = {
  title: string
  value: string
  subtitle?: string
  loading: boolean
  hasData: boolean
}

const MetricCard = ({ title, value, subtitle, loading, hasData }: MetricCardProps) => {
  return (
    <Card className="h-full">
      <CardHeader className="space-y-1">
        <CardTitle className="text-sm font-medium text-text-muted">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {loading ? (
          <Skeleton className="h-8 w-32" />
        ) : hasData ? (
          <div className="text-2xl font-semibold">{value}</div>
        ) : (
          <div className="text-sm text-text-muted">No data</div>
        )}
        {subtitle ? <div className="text-xs text-text-muted">{subtitle}</div> : null}
      </CardContent>
    </Card>
  )
}

type DeltaBadgeProps = {
  amount?: number | null
  rate?: number | null
  currency?: string
  compact?: boolean
}

const DeltaBadge = ({ amount, rate, currency, compact }: DeltaBadgeProps) => {
  if (amount === null || amount === undefined || rate === null || rate === undefined) {
    return <span className="text-xs text-text-muted">No comparison</span>
  }
  const positive = amount >= 0
  const label = buildDeltaLabel(amount, rate, currency)
  return (
    <Badge
      variant="outline"
      className={cn(
        "text-xs",
        positive
          ? "border-status-success/40 text-status-success"
          : "border-status-error/40 text-status-error",
        compact && "px-2 py-0.5"
      )}
    >
      {label}
    </Badge>
  )
}

export default function OrgBillingOverviewPage() {
  const [rangePreset, setRangePreset] = useState<RangePreset>("30d")
  const [customRange, setCustomRange] = useState<DateRange | undefined>()
  const [granularity, setGranularity] = useState("day")
  const [compare, setCompare] = useState(false)

  const [mrr, setMrr] = useState<MrrResponse | null>(null)
  const [mrrMovement, setMrrMovement] = useState<MrrMovementResponse | null>(null)
  const [revenue, setRevenue] = useState<RevenueResponse | null>(null)
  const [outstandingBalance, setOutstandingBalance] =
    useState<OutstandingBalanceResponse | null>(null)
  const [collectionRate, setCollectionRate] = useState<CollectionRateResponse | null>(null)
  const [subscribers, setSubscribers] = useState<SubscribersResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  const activeRange = useMemo(() => {
    const end = new Date()
    if (rangePreset === "custom" && customRange?.from && customRange?.to) {
      return { start: customRange.from, end: customRange.to }
    }
    const days = rangePreset === "7d" ? 7 : rangePreset === "90d" ? 90 : 30
    const start = new Date(end)
    start.setDate(end.getDate() - (days - 1))
    return { start, end }
  }, [rangePreset, customRange])

  const queryParams = useMemo(() => {
    return {
      start: formatDateOnly(activeRange.start),
      end: formatDateOnly(activeRange.end),
      granularity,
      compare,
    }
  }, [activeRange, granularity, compare])

  const handleExport = (type: string) => {
    const params = new URLSearchParams({
      start: formatDateOnly(activeRange.start),
      end: formatDateOnly(activeRange.end),
      granularity,
      compare: String(compare),
      format: "csv",
    })
    const url = `${admin.getUri()}/billing/overview/${type}?${params.toString()}`
    window.location.href = url
  }

  useEffect(() => {
    let isActive = true
    const fetchData = async () => {
      setIsLoading(true)
      setError(null)
      setIsForbidden(false)
      try {
        const [
          mrrResp,
          mrrMovementResp,
          revenueResp,
          outstandingResp,
          collectionResp,
          subscribersResp,
        ] = await Promise.all([
          admin.get<MrrResponse>("/billing/overview/mrr", { params: queryParams }),
          admin.get<MrrMovementResponse>("/billing/overview/mrr-movement", {
            params: queryParams,
          }),
          admin.get<RevenueResponse>("/billing/overview/revenue", { params: queryParams }),
          admin.get<OutstandingBalanceResponse>("/billing/overview/outstanding", {
            params: queryParams,
          }),
          admin.get<CollectionRateResponse>("/billing/overview/collection-rate", {
            params: queryParams,
          }),
          admin.get<SubscribersResponse>("/billing/overview/subscribers", { params: queryParams }),
        ])
        if (!isActive) return
        setMrr(mrrResp.data)
        setMrrMovement(mrrMovementResp.data)
        setRevenue(revenueResp.data)
        setOutstandingBalance(outstandingResp.data)
        setCollectionRate(collectionResp.data)
        setSubscribers(subscribersResp.data)
      } catch (err) {
        if (!isActive) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
        } else {
          setError(getErrorMessage(err, "Failed to load billing overview."))
        }
      } finally {
        if (isActive) {
          setIsLoading(false)
        }
      }
    }

    fetchData()
    return () => {
      isActive = false
    }
  }, [queryParams])

  const revenueSeries = revenue?.series ?? []
  const subscriberSeries = subscribers?.series ?? []
  const revenueCompare = compare ? revenue?.compare_series ?? [] : []
  const subscriberCompare = compare ? subscribers?.compare_series ?? [] : []



  const revenueChartData = useMemo(() => {
    if (!revenueSeries.length) return []
    return revenueSeries.map((point, index) => ({
      period: point.period,
      current: point.value,
      previous: revenueCompare[index]?.value ?? null,
    }))
  }, [revenueSeries, revenueCompare])

  const subscriberChartData = useMemo(() => {
    if (!subscriberSeries.length) return []
    return subscriberSeries.map((point, index) => ({
      period: point.period,
      current: point.value,
      previous: subscriberCompare[index]?.value ?? null,
    }))
  }, [subscriberSeries, subscriberCompare])

  const movementCurrency = mrrMovement?.currency ?? mrr?.currency ?? "USD"
  const netMrrChange = mrrMovement?.net_mrr_change
  const netMrrTone =
    netMrrChange !== null && netMrrChange !== undefined && netMrrChange < 0
      ? "text-status-error"
      : "text-status-success"
  const outstandingCurrency = outstandingBalance?.currency ?? mrr?.currency ?? "USD"
  const collectionCurrency = collectionRate?.currency ?? mrr?.currency ?? "USD"

  if (isForbidden) {
    return <ForbiddenState />
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold">Billing overview</h1>
        <p className="text-sm text-text-muted">
          Revenue-first insights for subscriptions, growth, and retention.
        </p>
      </div>

      <div className="flex flex-col gap-3 rounded-xl border border-border-subtle bg-bg-surface p-4 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-wrap items-center gap-3">
          <Select value={rangePreset} onValueChange={(value) => setRangePreset(value as RangePreset)}>
            <SelectTrigger className="w-40" size="sm">
              <SelectValue placeholder="Range" />
            </SelectTrigger>
            <SelectContent>
              {rangePresets.map((preset) => (
                <SelectItem key={preset.value} value={preset.value}>
                  {preset.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {rangePreset === "custom" && (
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" size="sm">
                  {formatRangeLabel(customRange)}
                </Button>
              </PopoverTrigger>
              <PopoverContent align="start" className="w-auto p-0">
                <Calendar
                  mode="range"
                  numberOfMonths={2}
                  selected={customRange}
                  onSelect={setCustomRange}
                  initialFocus
                />
              </PopoverContent>
              <PopoverContent align="start" className="w-auto p-0">
                <Calendar
                  mode="range"
                  numberOfMonths={2}
                  selected={customRange}
                  onSelect={setCustomRange}
                  initialFocus
                />
              </PopoverContent>
            </Popover>
          )}

          <Select value={granularity} onValueChange={setGranularity}>
            <SelectTrigger className="w-32" size="sm">
              <SelectValue placeholder="Granularity" />
            </SelectTrigger>
            <SelectContent>
              {granularityOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="gap-2">
                <Download className="h-4 w-4" />
                Export
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>Download report</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => handleExport("revenue")}>Revenue</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => handleExport("mrr")}>MRR</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => handleExport("subscribers")}>Subscribers</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => handleExport("mrr-movement")}>MRR Movement</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => handleExport("outstanding")}>Outstanding Balance</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => handleExport("collection-rate")}>Collection Rate</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div className="flex items-center gap-2 text-sm text-text-muted">
          <Switch checked={compare} onCheckedChange={setCompare} />
          <span>Compare to previous period</span>
        </div>
      </div>

      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle>MRR movement</CardTitle>
          <CardDescription>Explains why recurring revenue changed.</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="grid gap-4 lg:grid-cols-[1.2fr_1fr]">
              <div className="space-y-3">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-10 w-48" />
                <Skeleton className="h-4 w-64" />
              </div>
              <div className="space-y-2">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
              </div>
            </div>
          ) : mrrMovement?.has_data ? (
            <div className="grid gap-6 lg:grid-cols-[1.2fr_1fr]">
              <div className="space-y-3">
                <div className="text-xs font-medium uppercase tracking-wide text-text-muted">
                  Net MRR change
                </div>
                <div className={cn("text-3xl font-semibold", netMrrTone)}>
                  {formatSignedCurrency(netMrrChange ?? null, movementCurrency)}
                </div>
                <div className="text-sm text-text-muted">
                  New and expansion MRR increase growth, while churn and contraction reduce it.
                </div>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-text-muted">New MRR</span>
                  <span className="font-medium text-status-success">
                    {formatSignedCurrency(mrrMovement?.new_mrr ?? null, movementCurrency)}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-text-muted">Expansion MRR</span>
                  <span className="font-medium text-status-success">
                    {formatSignedCurrency(mrrMovement?.expansion_mrr ?? null, movementCurrency)}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-text-muted">Contraction MRR</span>
                  <span className="font-medium text-status-error">
                    {formatSignedCurrency(
                      mrrMovement ? -mrrMovement.contraction_mrr : null,
                      movementCurrency
                    )}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-text-muted">Churned MRR</span>
                  <span className="font-medium text-status-error">
                    {formatSignedCurrency(
                      mrrMovement ? -mrrMovement.churned_mrr : null,
                      movementCurrency
                    )}
                  </span>
                </div>
              </div>
            </div>
          ) : (
            <div className="text-sm text-text-muted">No data</div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <MetricCard
          title="MRR"
          value={formatCurrency(mrr?.current ?? null, mrr?.currency ?? "USD")}
          subtitle={compare ? "Month-over-month recurring baseline." : "Recurring baseline at period end."}
          loading={isLoading}
          hasData={Boolean(mrr?.has_data)}
        />
        <MetricCard
          title="Net revenue"
          value={formatCurrency(revenue?.total ?? null, revenue?.currency ?? "USD")}
          subtitle={compare ? "Booked from ledger entries." : "Booked revenue in selected period."}
          loading={isLoading}
          hasData={Boolean(revenue?.has_data)}
        />
        <Card className="h-full">
          <CardHeader className="space-y-1">
            <CardTitle className="text-sm font-medium text-text-muted">Outstanding balance</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {isLoading ? (
              <Skeleton className="h-8 w-32" />
            ) : outstandingBalance?.has_data ? (
              <div className="space-y-1">
                <div className="text-2xl font-semibold">
                  {formatCurrency(outstandingBalance.outstanding ?? null, outstandingCurrency)}
                </div>
                <div className="text-xs text-status-warning">
                  Overdue {formatCurrency(outstandingBalance.overdue ?? null, outstandingCurrency)}
                </div>
              </div>
            ) : (
              <div className="text-sm text-text-muted">No data</div>
            )}
            <div className="text-xs text-text-muted">Total owed but not collected.</div>
          </CardContent>
        </Card>
        <Card className="h-full">
          <CardHeader className="space-y-1">
            <CardTitle className="text-sm font-medium text-text-muted">Collection rate</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {isLoading ? (
              <Skeleton className="h-8 w-32" />
            ) : collectionRate?.has_data && collectionRate.collection_rate !== undefined ? (
              <div className="text-2xl font-semibold">
                {formatPercentCompact(collectionRate.collection_rate ?? null)}
              </div>
            ) : (
              <div className="text-sm text-text-muted">No data</div>
            )}
            <div className="text-xs text-text-muted">
              Percentage of invoiced revenue successfully collected.
            </div>
            {collectionRate?.has_data ? (
              <div className="text-xs text-text-muted">
                Collected {formatCurrency(collectionRate.collected_amount ?? null, collectionCurrency)} /{" "}
                {formatCurrency(collectionRate.invoiced_amount ?? null, collectionCurrency)}
              </div>
            ) : null}
          </CardContent>
        </Card>
        <MetricCard
          title="Active subscribers"
          value={formatCount(subscribers?.current ?? null)}
          subtitle={compare ? "Active at period end." : "Current active subscribers."}
          loading={isLoading}
          hasData={Boolean(subscribers?.has_data)}
        />
        <MetricCard
          title="Churn rate"
          value={formatPercentCompact(subscribers?.churn_rate ?? null)}
          subtitle="Canceled or ended in period."
          loading={isLoading}
          hasData={subscribers?.churn_rate !== null && subscribers?.churn_rate !== undefined}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-1">


        <Card className="h-full">
          <CardHeader>
            <CardTitle>Revenue over time</CardTitle>
            <CardDescription>Ledger-backed revenue by period.</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-[260px] w-full" />
            ) : revenue?.has_data ? (
              <ChartContainer config={revenueChartConfig} className="h-[260px] w-full">
                <BarChart data={revenueChartData}>
                  <CartesianGrid vertical={false} />
                  <XAxis
                    dataKey="period"
                    tickLine={false}
                    axisLine={false}
                    tickMargin={8}
                    tickFormatter={(value) => formatAxisLabel(value, granularity)}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(value) => formatCurrencyCompact(value, revenue?.currency ?? "USD")}
                    width={72}
                  />
                  <ChartTooltip
                    cursor={false}
                    content={
                      <ChartTooltipContent
                        indicator="dot"
                        labelFormatter={(value) => formatAxisLabel(value as string, granularity)}
                        formatter={(value, name) => {
                          const label = name === "previous" ? "Previous" : "Current"
                          return (
                            <div className="flex w-full items-center justify-between gap-3">
                              <span className="text-text-muted">{label}</span>
                              <span className="font-medium">
                                {formatCurrency(value as number, revenue?.currency ?? "USD")}
                              </span>
                            </div>
                          )
                        }}
                      />
                    }
                  />
                  <Bar
                    dataKey="current"
                    fill="var(--color-current)"
                    radius={[6, 6, 0, 0]}
                  />
                  {compare && (
                    <Bar
                      dataKey="previous"
                      fill="var(--color-previous)"
                      radius={[6, 6, 0, 0]}
                      opacity={0.5}
                    />
                  )}
                </BarChart>
              </ChartContainer>
            ) : (
              <div className="flex h-[260px] items-center justify-center text-sm text-text-muted">
                No data
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="h-full">
          <CardHeader>
            <CardTitle>Subscriber trend</CardTitle>
            <CardDescription>Active subscriber count over time.</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-[220px] w-full" />
            ) : subscribers?.has_data ? (
              <ChartContainer config={subscriberChartConfig} className="h-[220px] w-full">
                <AreaChart data={subscriberChartData}>
                  <CartesianGrid vertical={false} />
                  <XAxis
                    dataKey="period"
                    tickLine={false}
                    axisLine={false}
                    tickMargin={8}
                    tickFormatter={(value) => formatAxisLabel(value, granularity)}
                  />
                  <YAxis tickLine={false} axisLine={false} width={48} />
                  <ChartTooltip
                    cursor={false}
                    content={
                      <ChartTooltipContent
                        indicator="line"
                        labelFormatter={(value) => formatAxisLabel(value as string, granularity)}
                        formatter={(value, name) => {
                          const label = name === "previous" ? "Previous" : "Current"
                          return (
                            <div className="flex w-full items-center justify-between gap-3">
                              <span className="text-text-muted">{label}</span>
                              <span className="font-medium">{formatCount(value as number)}</span>
                            </div>
                          )
                        }}
                      />
                    }
                  />
                  <Area
                    dataKey="current"
                    type="monotone"
                    stroke="var(--color-current)"
                    fill="var(--color-current)"
                    fillOpacity={0.18}
                  />
                  {compare && (
                    <Line
                      dataKey="previous"
                      type="monotone"
                      stroke="var(--color-previous)"
                      strokeDasharray="4 4"
                      dot={false}
                    />
                  )}
                </AreaChart>
              </ChartContainer>
            ) : (
              <div className="flex h-[220px] items-center justify-center text-sm text-text-muted">
                No data
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="h-full">
          <CardHeader>
            <CardTitle>Reports</CardTitle>
            <CardDescription>Export a snapshot of key metrics.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-sm text-text-muted">
              Reports are read-only exports from billing and ledger snapshots.
            </div>
            <Button variant="outline" size="sm" disabled>
              Download CSV
            </Button>
          </CardContent>
        </Card>
      </div>

      {compare && (
        <div className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
          <DeltaBadge amount={mrr?.growth_amount} rate={mrr?.growth_rate} currency={mrr?.currency} />
          <DeltaBadge amount={revenue?.growth_amount} rate={revenue?.growth_rate} currency={revenue?.currency} />
          <DeltaBadge amount={subscribers?.growth_amount} rate={subscribers?.growth_rate} />
        </div>
      )}
    </div>
  )
}
