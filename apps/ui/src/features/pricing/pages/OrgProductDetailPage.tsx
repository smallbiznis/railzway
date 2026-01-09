import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { Box, Plus } from "lucide-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type Product = {
  id: string
  name?: string
  code?: string
  description?: string | null
  active?: boolean
  metadata?: Record<string, unknown> | null
}

type Price = {
  id: string
  product_id: string
  name?: string
  code?: string
  description?: string | null
  pricing_model: string
  billing_interval: string
  billing_interval_count: number
  is_default?: boolean
  created_at?: string
}

type PriceAmount = {
  id: string
  price_id: string
  meter_id?: string | null
  currency: string
  unit_amount_cents: number
  minimum_amount_cents?: number | null
  maximum_amount_cents?: number | null
}

type Meter = {
  id: string
  name?: string
  code?: string
}

const formatInterval = (interval: string, count: number) => {
  const base = interval?.toLowerCase() ?? ""
  if (!base) return "-"
  if (!count || count === 1) {
    return `Per ${base}`
  }
  return `Every ${count} ${base}${count === 1 ? "" : "s"}`
}

const formatAmount = (amountCents: number, currency: string) => {
  const safeCurrency = currency?.toUpperCase() || "USD"
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: safeCurrency,
    }).format(amountCents / 100)
  } catch {
    return `${amountCents} ${safeCurrency}`
  }
}

const formatDateShort = (value?: string) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
  }).format(date)
}

const readMetadataValue = (metadata: Product["metadata"], key: string) => {
  if (!metadata || typeof metadata !== "object") return "-"
  const value = (metadata as Record<string, unknown>)[key]
  if (typeof value === "string") {
    const trimmed = value.trim()
    return trimmed.length ? trimmed : "-"
  }
  if (Array.isArray(value)) {
    const items = value.filter((item) => typeof item === "string" && item.trim().length > 0)
    return items.length ? items.join(", ") : "-"
  }
  if (value && typeof value === "object") {
    const keys = Object.keys(value as Record<string, unknown>)
    return keys.length ? keys.join(", ") : "-"
  }
  return "-"
}

const formatUnitLabel = (pricingModel: string) => {
  const normalized = pricingModel?.toUpperCase()
  if (normalized === "FLAT") return "per interval"
  return "per unit"
}

export default function OrgProductDetailPage() {
  const { orgId, productId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [product, setProduct] = useState<Product | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [prices, setPrices] = useState<Price[]>([])
  const [priceAmounts, setPriceAmounts] = useState<PriceAmount[]>([])
  const [meters, setMeters] = useState<Meter[]>([])
  const [pricingLoading, setPricingLoading] = useState(true)
  const [pricingError, setPricingError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  useEffect(() => {
    if (!orgId || !productId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/products/${productId}`)
      .then((response) => {
        if (!isMounted) return
        setProduct(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load product."))
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, productId])

  useEffect(() => {
    if (!orgId || !productId) {
      setPricingLoading(false)
      return
    }

    let isMounted = true
    setPricingLoading(true)
    setPricingError(null)

    Promise.all([
      admin.get("/prices"),
      admin.get("/price_amounts"),
      admin.get("/meters"),
    ])
      .then(([priceRes, amountRes, meterRes]) => {
        if (!isMounted) return
        const allPrices = priceRes.data?.data ?? []
        const filteredPrices = allPrices.filter(
          (item: Price) => String(item.product_id) === String(productId)
        )
        setPrices(filteredPrices)
        setPriceAmounts(amountRes.data?.data ?? [])
        setMeters(meterRes.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setPricingError(getErrorMessage(err, "Unable to load pricing."))
      })
      .finally(() => {
        if (!isMounted) return
        setPricingLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, productId])

  const priceAmountsByPriceId = useMemo(() => {
    const map = new Map<string, PriceAmount[]>()
    priceAmounts.forEach((amount) => {
      const key = String(amount.price_id)
      const list = map.get(key) ?? []
      list.push(amount)
      map.set(key, list)
    })
    return map
  }, [priceAmounts])

  const metersById = useMemo(() => {
    const map = new Map<string, Meter>()
    meters.forEach((meter) => {
      map.set(String(meter.id), meter)
    })
    return map
  }, [meters])

  const defaultPrice = useMemo(() => {
    if (!prices.length) return null
    return prices.find((price) => price.is_default) ?? prices[0]
  }, [prices])

  const summaryLine = useMemo(() => {
    if (!defaultPrice) return "No pricing configured yet."
    const amounts = priceAmountsByPriceId.get(String(defaultPrice.id)) ?? []
    if (!amounts.length) return "No pricing configured yet."
    const amount = amounts[0]
    const currency = amount.currency?.toUpperCase() || "USD"
    const amountLabel = formatAmount(amount.unit_amount_cents, currency)
    const unitLabel = formatUnitLabel(defaultPrice.pricing_model)
    const intervalLabel = formatInterval(
      defaultPrice.billing_interval,
      defaultPrice.billing_interval_count
    )
    return `${amountLabel} ${currency} ${unitLabel} | ${intervalLabel}`
  }, [defaultPrice, priceAmountsByPriceId])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading product...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this product." />
  }

  if (!product) {
    return <div className="text-text-muted text-sm">Product not found.</div>
  }

  const productName = product.name || "Product detail"
  const productCode = product.code || "-"
  const productDescription = product.description || "-"
  const isActive = product.active === true
  const isStatusKnown = product.active !== undefined
  const productStatus = isStatusKnown ? (isActive ? "Active" : "Archived") : "Unknown"

  return (
    <div className="space-y-6">
      {orgId && (
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link to={`/orgs/${orgId}/products`}>Products</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>{productName}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      )}

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex flex-wrap items-start gap-4">
          <div className="flex size-12 items-center justify-center rounded-lg bg-bg-subtle">
            <Box className="size-6 text-text-muted" />
          </div>
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold">{productName}</h1>
              <Badge
                variant={isActive ? "secondary" : "outline"}
                className={
                  isActive
                    ? "border-status-success/30 bg-status-success/10 text-status-success"
                    : "text-text-muted"
                }
              >
                {productStatus}
              </Badge>
            </div>
            <div className="text-text-muted text-sm">{summaryLine}</div>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2" />
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Pricing</CardTitle>
                <CardDescription>Manage all prices attached to this product.</CardDescription>
              </div>
              {orgId && productId && (
                <Button asChild variant="outline" size="icon" aria-label="Add price" disabled={!canManage}>
                  <Link to={`/orgs/${orgId}/products/${productId}/prices/create`}>
                    <Plus className="size-4" />
                  </Link>
                </Button>
              )}
            </CardHeader>
            <CardContent className="space-y-4">
              {pricingLoading && (
                <div className="text-text-muted text-sm">Loading pricing...</div>
              )}
              {pricingError && <div className="text-status-error text-sm">{pricingError}</div>}
              {!pricingLoading && !pricingError && prices.length === 0 && (
                <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                  No prices configured yet.
                </div>
              )}
              {!pricingLoading && !pricingError && prices.length > 0 && (
                <div className="rounded-lg border">
                  <Table className="min-w-[720px]">
                    <TableHeader>
                      <TableRow>
                        <TableHead>Price</TableHead>
                        <TableHead>Description</TableHead>
                        <TableHead>Subscriptions</TableHead>
                        <TableHead>Created</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {prices.map((price) => {
                        const amounts =
                          priceAmountsByPriceId.get(String(price.id)) ?? []
                        const mainAmount = amounts[0]
                        const currency = mainAmount?.currency?.toUpperCase() || "USD"
                        const amountLabel = mainAmount
                          ? `${formatAmount(mainAmount.unit_amount_cents, currency)} ${currency} ${formatUnitLabel(price.pricing_model)}`
                          : "No amount"
                        const intervalLabel = formatInterval(
                          price.billing_interval,
                          price.billing_interval_count
                        )
                        const meter = mainAmount?.meter_id
                          ? metersById.get(String(mainAmount.meter_id))
                          : null
                        const meterLabel = meter
                          ? meter.name || meter.code || meter.id
                          : null

                        return (
                          <TableRow key={price.id}>
                            <TableCell className="font-medium">
                              <div className="flex flex-col gap-1">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span>{price.name || price.code || "Price"}</span>
                                  {price.is_default && (
                                    <Badge
                                      variant="outline"
                                      className="border-accent-primary/30 bg-accent-primary/10 text-accent-primary"
                                    >
                                      Default
                                    </Badge>
                                  )}
                                </div>
                                <span className="text-text-muted text-xs">
                                  {amountLabel} | {intervalLabel}
                                  {meterLabel ? ` | Meter: ${meterLabel}` : ""}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell className="text-text-muted text-sm">
                              {price.description || "-"}
                            </TableCell>
                            <TableCell className="text-text-muted text-sm">
                              0 active
                            </TableCell>
                            <TableCell className="text-text-muted text-sm">
                              {formatDateShort(price.created_at)}
                            </TableCell>
                            <TableCell className="text-right">
                              {orgId ? (
                                <Button asChild variant="ghost" size="sm">
                                  <Link to={`/orgs/${orgId}/prices/${price.id}`}>
                                    Detail
                                  </Link>
                                </Button>
                              ) : (
                                <Button variant="ghost" size="sm" disabled>
                                  Detail
                                </Button>
                              )}
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>

        </div>

        <Card>
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div>
              <div className="text-text-muted">Product ID</div>
              <div className="font-medium">{product.id}</div>
            </div>
            <div>
              <div className="text-text-muted">Product code</div>
              <div className="font-medium">{productCode}</div>
            </div>
            <div>
              <div className="text-text-muted">Product tax code</div>
              <div className="font-medium">{readMetadataValue(product.metadata, "tax_code")}</div>
            </div>
            <div>
              <div className="text-text-muted">Marketing feature list</div>
              <div className="font-medium">
                {readMetadataValue(product.metadata, "marketing_features")}
              </div>
            </div>
            <div>
              <div className="text-text-muted">Description</div>
              <div className="font-medium">{productDescription}</div>
            </div>
            <div>
              <div className="text-text-muted">Attributes</div>
              <div className="font-medium">
                {readMetadataValue(product.metadata, "attributes")}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
