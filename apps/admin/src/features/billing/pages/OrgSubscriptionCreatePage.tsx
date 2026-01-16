import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { Plus, Trash2 } from "lucide-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
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

type Customer = {
  id: string | number
  name?: string
  email?: string
}

type CustomerListResponse = {
  customers: Customer[]
}

type Price = {
  id: string
  name?: string
  code?: string
  pricing_model: string
  billing_mode: string
  billing_interval: string
  active: boolean
  retired_at?: string | null
}

type PriceAmount = {
  id: string
  price_id: string
  meter_id?: string | null
  currency: string
  unit_amount_cents: number
  effective_from: string
  effective_to?: string | null
}

type Meter = {
  id: string
  name: string
  code: string
  unit: string
  active: boolean
}

type SubscriptionItemDraft = {
  id: string
  priceId: string
  meterId: string
  quantity: string
}

const cycleLabels: Record<string, string> = {
  DAILY: "Daily",
  WEEKLY: "Weekly",
  MONTHLY: "Monthly",
}

const getCycleFromInterval = (interval?: string | null) => {
  switch ((interval ?? "").toUpperCase()) {
    case "DAY":
      return "DAILY"
    case "WEEK":
      return "WEEKLY"
    case "MONTH":
      return "MONTHLY"
    default:
      return ""
  }
}

const formatCustomerLabel = (customer: Customer) => {
  const name = customer.name?.trim()
  const email = customer.email?.trim()
  if (name && email) return `${name} - ${email}`
  if (name) return name
  if (email) return email
  return `Customer ${customer.id}`
}

const formatPriceLabel = (price: Price) => {
  const name = price.name?.trim()
  const code = price.code?.trim()
  if (name && code && name !== code) return `${name} (${code})`
  if (name) return name
  if (code) return code
  return `Price ${price.id}`
}

const formatMeterLabel = (meter?: Meter, fallbackId?: string) => {
  if (!meter) return fallbackId ? `Meter ${fallbackId}` : "Unknown meter"
  return `${meter.name} (${meter.code})`
}

const buildItem = (): SubscriptionItemDraft => ({
  id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
  priceId: "",
  meterId: "",
  quantity: "1",
})

export default function OrgSubscriptionCreatePage() {
  const { orgId } = useParams()
  const navigate = useNavigate()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const subscriptionsPath = orgId ? `/orgs/${orgId}/subscriptions` : "/orgs"

  const [customers, setCustomers] = useState<Customer[]>([])
  const [prices, setPrices] = useState<Price[]>([])
  const [meters, setMeters] = useState<Meter[]>([])
  const [items, setItems] = useState<SubscriptionItemDraft[]>([buildItem()])
  const [selectedCustomer, setSelectedCustomer] = useState("")
  const [billingCycleType, setBillingCycleType] = useState("MONTHLY")
  const [collectionMode, setCollectionMode] = useState("SEND_INVOICE")
  const [trialDays, setTrialDays] = useState("")
  const [customersLoading, setCustomersLoading] = useState(false)
  const [pricesLoading, setPricesLoading] = useState(false)
  const [customersError, setCustomersError] = useState<string | null>(null)
  const [pricesError, setPricesError] = useState<string | null>(null)
  const [metersError, setMetersError] = useState<string | null>(null)
  const [priceAmountsByPrice, setPriceAmountsByPrice] = useState<
    Record<string, PriceAmount[]>
  >({})
  const [priceAmountsLoading, setPriceAmountsLoading] = useState<
    Record<string, boolean>
  >({})
  const [priceAmountsError, setPriceAmountsError] = useState<Record<string, string>>(
    {}
  )
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isForbidden, setIsForbidden] = useState(false)

  const cycleOptions = useMemo(() => {
    const set = new Set<string>()
    prices.forEach((price) => {
      const cycle = getCycleFromInterval(price.billing_interval)
      if (cycle) set.add(cycle)
    })
    const order = ["MONTHLY", "WEEKLY", "DAILY"]
    const filtered = order.filter((cycle) => set.has(cycle))
    return filtered.length > 0 ? filtered : order
  }, [prices])

  const priceLookup = useMemo(() => {
    const map = new Map<string, Price>()
    prices.forEach((price) => map.set(price.id, price))
    return map
  }, [prices])

  const meterLookup = useMemo(() => {
    const map = new Map<string, Meter>()
    meters.forEach((meter) => map.set(meter.id, meter))
    return map
  }, [meters])

  const filteredPrices = useMemo(() => {
    return prices.filter((price) => {
      if (!price.active || price.retired_at) return false
      const cycle = getCycleFromInterval(price.billing_interval)
      return cycle === billingCycleType
    })
  }, [prices, billingCycleType])

  useEffect(() => {
    if (!cycleOptions.includes(billingCycleType)) {
      setBillingCycleType(cycleOptions[0])
    }
  }, [billingCycleType, cycleOptions])

  // Auto-select meter if only one option exists for the selected price
  useEffect(() => {
    setItems((prevItems) => {
      let hasChanges = false
      const nextItems = prevItems.map((item) => {
        if (!item.priceId || item.meterId) return item

        const price = priceLookup.get(item.priceId)
        if (!price || price.pricing_model === "FLAT") return item

        // Check if amounts are loaded
        const amounts = priceAmountsByPrice[item.priceId]
        if (!amounts) return item

        // Get options
        const uniqueMeters = new Set<string>()
        amounts.forEach((a) => {
          if (a.meter_id) uniqueMeters.add(a.meter_id)
        })

        if (uniqueMeters.size === 1) {
          hasChanges = true
          return { ...item, meterId: Array.from(uniqueMeters)[0] }
        }
        return item
      })
      return hasChanges ? nextItems : prevItems
    })
  }, [priceAmountsByPrice, priceLookup])

  useEffect(() => {
    if (!orgId) {
      setCustomers([])
      return
    }

    let isMounted = true
    setCustomersLoading(true)
    setCustomersError(null)

    admin
      .get("/customers", { params: { page_size: 200 } })
      .then((response) => {
        if (!isMounted) return
        const payload: CustomerListResponse = response.data?.data ?? {
          customers: [],
        }
        setCustomers(Array.isArray(payload.customers) ? payload.customers : [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setCustomersError(getErrorMessage(err, "Unable to load customers."))
        setCustomers([])
      })
      .finally(() => {
        if (!isMounted) return
        setCustomersLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  useEffect(() => {
    if (!orgId) {
      setPrices([])
      return
    }

    let isMounted = true
    setPricesLoading(true)
    setPricesError(null)

    admin
      .get("/prices")
      .then((response) => {
        if (!isMounted) return
        setPrices(Array.isArray(response.data?.data) ? response.data?.data : [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setPricesError(getErrorMessage(err, "Unable to load prices."))
        setPrices([])
      })
      .finally(() => {
        if (!isMounted) return
        setPricesLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  useEffect(() => {
    if (!orgId) {
      setMeters([])
      return
    }

    let isMounted = true
    setMetersError(null)

    admin
      .get("/meters")
      .then((response) => {
        if (!isMounted) return
        setMeters(Array.isArray(response.data?.data) ? response.data?.data : [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setMetersError(getErrorMessage(err, "Unable to load meters."))
        setMeters([])
      })
      .finally(() => {
        if (!isMounted) return
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  const loadPriceAmounts = async (priceId: string) => {
    if (
      !priceId ||
      priceAmountsLoading[priceId] ||
      (priceAmountsByPrice[priceId] && !priceAmountsError[priceId])
    ) {
      return
    }

    setPriceAmountsLoading((prev) => ({ ...prev, [priceId]: true }))
    setPriceAmountsError((prev) => ({ ...prev, [priceId]: "" }))

    try {
      const response = await admin.get("/price_amounts", {
        params: { price_id: priceId },
      })
      setPriceAmountsByPrice((prev) => ({
        ...prev,
        [priceId]: Array.isArray(response.data?.data) ? response.data?.data : [],
      }))
    } catch (err) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
        return
      }
      setPriceAmountsError((prev) => ({
        ...prev,
        [priceId]: getErrorMessage(err, "Unable to load price amounts."),
      }))
      setPriceAmountsByPrice((prev) => ({ ...prev, [priceId]: [] }))
    } finally {
      setPriceAmountsLoading((prev) => ({ ...prev, [priceId]: false }))
    }
  }

  const getMeterOptions = (priceId: string) => {
    const amounts = priceAmountsByPrice[priceId] ?? []
    const unique = new Set<string>()
    amounts.forEach((amount) => {
      if (amount.meter_id) {
        unique.add(amount.meter_id)
      }
    })
    return Array.from(unique)
  }

  const handleBillingCycleChange = (next: string) => {
    setBillingCycleType(next)
    setItems((prev) =>
      prev.map((item) => {
        if (!item.priceId) return item
        const price = priceLookup.get(item.priceId)
        const cycle = price ? getCycleFromInterval(price.billing_interval) : ""
        if (cycle && cycle !== next) {
          return { ...item, priceId: "", meterId: "" }
        }
        return item
      })
    )
  }

  const handlePriceChange = (itemId: string, nextPriceId: string) => {
    // For FLAT pricing, clear the meterId immediately.
    // For usage pricing, we'll try to auto-select if there's only one option
    // but we can't do that synchronously easily unless we already have the price amounts loaded.
    // Since loadPriceAmounts is async, we can't guarantee options are ready.
    // However, if we've already loaded them (cached), we could.
    // For now, let's just clear usage meterId to force selection, unless we want to be smarter.
    setItems((prev) =>
      prev.map((item) =>
        item.id === itemId
          ? {
            ...item,
            priceId: nextPriceId,
            meterId: "", // Always reset meter when price changes
          }
          : item
      )
    )
    if (nextPriceId) {
      void loadPriceAmounts(nextPriceId)
    }
  }

  const handleMeterChange = (itemId: string, nextMeterId: string) => {
    setItems((prev) =>
      prev.map((item) =>
        item.id === itemId ? { ...item, meterId: nextMeterId } : item
      )
    )
  }

  const handleQuantityChange = (itemId: string, nextQuantity: string) => {
    setItems((prev) =>
      prev.map((item) =>
        item.id === itemId ? { ...item, quantity: nextQuantity } : item
      )
    )
  }

  const handleAddItem = () => {
    setItems((prev) => [...prev, buildItem()])
  }

  const handleRemoveItem = (itemId: string) => {
    setItems((prev) => (prev.length > 1 ? prev.filter((item) => item.id !== itemId) : prev))
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!canManage) {
      setError("You do not have permission to create subscriptions.")
      return
    }
    if (!orgId) {
      setError("Organization is missing.")
      return
    }
    if (!selectedCustomer) {
      setError("Select a customer before creating a subscription.")
      return
    }
    if (!billingCycleType) {
      setError("Select a billing cycle.")
      return
    }

    const parsedTrial = trialDays.trim()
    const trialValue = parsedTrial ? Number(parsedTrial) : undefined
    if (trialValue !== undefined && (!Number.isFinite(trialValue) || trialValue < 0)) {
      setError("Trial days must be a positive number.")
      return
    }
    const normalizedTrialDays =
      trialValue !== undefined && trialValue > 0 ? Math.floor(trialValue) : undefined

    const normalizedItems = items.map((item) => {
      const quantityValue = Number.parseInt(item.quantity, 10)
      const clampedQuantity = Number.isFinite(quantityValue) && quantityValue > 0
        ? Math.min(quantityValue, 127)
        : 1
      return {
        price_id: item.priceId.trim(),
        meter_id: item.meterId.trim(),
        quantity: clampedQuantity,
      }
    })

    if (
      normalizedItems.some((item) => {
        const price = priceLookup.get(item.price_id)
        if (!price) return true
        // FLAT prices don't need a meter
        if (price.pricing_model === "FLAT") return false
        // Others need a meter
        return !item.meter_id
      })
    ) {
      setError("Each usage-based line item needs a meter.")
      return
    }

    if (
      normalizedItems.some((item) => {
        const price = priceLookup.get(item.price_id)
        if (!price || price.pricing_model === "FLAT") return false
        return getMeterOptions(item.price_id).length === 0
      })
    ) {
      setError("One or more prices have no meters configured.")
      return
    }

    setError(null)
    setIsSubmitting(true)

    try {
      const payload = {
        customer_id: selectedCustomer,
        collection_mode: collectionMode,
        billing_cycle_type: billingCycleType,
        items: normalizedItems,
        trial_days: normalizedTrialDays,
      }
      const response = await admin.post("/subscriptions", payload)
      const subscriptionId = response.data?.data?.id
      if (subscriptionId) {
        navigate(`/orgs/${orgId}/subscriptions/${subscriptionId}`, { replace: true })
      } else {
        navigate(subscriptionsPath, { replace: true })
      }
    } catch (err) {
      setError(getErrorMessage(err, "Unable to create subscription."))
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!canManage) {
    return <ForbiddenState description="You do not have access to create subscriptions." />
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to subscription settings." />
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="text-sm text-text-muted">
          <Link className="text-accent-primary hover:underline" to={subscriptionsPath}>
            Subscriptions
          </Link>{" "}
          / Create subscription
        </div>
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Create subscription</h1>
          <p className="text-text-muted text-sm">
            Attach active prices and meters to a customer billing cycle.
          </p>
        </div>
      </div>

      <form className="space-y-6" onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Customer</CardTitle>
            <CardDescription>Choose who will be billed.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <Label htmlFor="subscription-customer">Customer</Label>
            <Select
              value={selectedCustomer}
              onValueChange={setSelectedCustomer}
              disabled={customersLoading}
            >
              <SelectTrigger id="subscription-customer" className="w-full">
                <SelectValue
                  placeholder={customersLoading ? "Loading customers..." : "Select a customer"}
                />
              </SelectTrigger>
              <SelectContent>
                {customersLoading && (
                  <SelectItem value="loading" disabled>
                    Loading customers...
                  </SelectItem>
                )}
                {!customersLoading && customers.length === 0 && (
                  <SelectItem value="empty" disabled>
                    No customers found
                  </SelectItem>
                )}
                {customers.map((customer) => (
                  <SelectItem key={customer.id} value={String(customer.id)}>
                    {formatCustomerLabel(customer)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {customersError && (
              <p className="text-status-error text-sm">{customersError}</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Subscription settings</CardTitle>
            <CardDescription>Define billing cadence and collection mode.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="billing-cycle-type">Billing cycle</Label>
              <Select value={billingCycleType} onValueChange={handleBillingCycleChange}>
                <SelectTrigger id="billing-cycle-type">
                  <SelectValue placeholder="Select a billing cycle" />
                </SelectTrigger>
                <SelectContent>
                  {cycleOptions.map((cycle) => (
                    <SelectItem key={cycle} value={cycle}>
                      {cycleLabels[cycle] ?? cycle}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="collection-mode">Collection mode</Label>
              <Select value={collectionMode} onValueChange={setCollectionMode}>
                <SelectTrigger id="collection-mode">
                  <SelectValue placeholder="Select a collection mode" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SEND_INVOICE">Send invoice</SelectItem>
                  <SelectItem value="CHARGE_AUTOMATICALLY">Charge automatically</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="trial-days">Trial days (optional)</Label>
              <Input
                id="trial-days"
                type="number"
                min={1}
                value={trialDays}
                onChange={(event) => setTrialDays(event.target.value)}
                placeholder="0"
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Subscription items</CardTitle>
            <CardDescription>
              Each item must include an active price and a meter that matches its price
              amounts.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="overflow-x-auto rounded-lg border">
              <Table className="min-w-[780px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>Price</TableHead>
                    <TableHead>Meter</TableHead>
                    <TableHead className="w-[120px]">Quantity</TableHead>
                    <TableHead className="w-[80px]" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => {
                    const price = priceLookup.get(item.priceId)
                    const meterOptions = item.priceId ? getMeterOptions(item.priceId) : []
                    const priceAmountError = item.priceId
                      ? priceAmountsError[item.priceId]
                      : ""
                    const isLoadingMeters =
                      item.priceId && priceAmountsLoading[item.priceId]

                    // Only show meter error if price is NOT FLAT
                    const isFlat = price?.pricing_model === "FLAT"
                    const meterError =
                      !isFlat && item.priceId && !isLoadingMeters && meterOptions.length === 0
                        ? "No meters available for this price."
                        : ""

                    // Helper to get display price
                    const getDisplayPrice = () => {
                      if (!item.priceId || !priceAmountsByPrice[item.priceId]) return null
                      const amounts = priceAmountsByPrice[item.priceId]
                      if (amounts.length === 0) return null
                      // Should we show range or just the first one? 
                      // For FLAT there is usually one.
                      // For USAGE there might be multiple rates but usually we want to show something indicative.
                      const amt = amounts[0]
                      const formatter = new Intl.NumberFormat("en-US", {
                        style: "currency",
                        currency: amt.currency,
                      })
                      return formatter.format(amt.unit_amount_cents / 100)
                    }

                    // 1. Prevent duplicate price selection
                    // Filter out prices that are already selected in other rows
                    // But keep the current item's price in the list so it's not hidden
                    const currentSelectedPriceIds = items
                      .map((i) => i.priceId)
                      .filter((id) => id && id !== item.priceId)

                    const availablePrices = filteredPrices.filter(
                      (p) => !currentSelectedPriceIds.includes(p.id)
                    )

                    return (
                      <TableRow key={item.id}>
                        <TableCell className="min-w-[240px] align-top">
                          <div className="space-y-1">
                            <Select
                              value={item.priceId}
                              onValueChange={(value) => handlePriceChange(item.id, value)}
                              disabled={pricesLoading}
                            >
                              <SelectTrigger className="w-full">
                                <SelectValue
                                  placeholder={
                                    pricesLoading ? "Loading prices..." : "Select a price"
                                  }
                                />
                              </SelectTrigger>
                              <SelectContent>
                                {pricesLoading && (
                                  <SelectItem value="loading" disabled>
                                    Loading prices...
                                  </SelectItem>
                                )}
                                {!pricesLoading && filteredPrices.length === 0 && (
                                  <SelectItem value="empty" disabled>
                                    No prices for this billing cycle
                                  </SelectItem>
                                )}
                                {availablePrices.map((priceItem) => (
                                  <SelectItem key={priceItem.id} value={priceItem.id}>
                                    {formatPriceLabel(priceItem)}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            {price && (
                              <div className="flex items-center justify-between text-xs text-text-muted">
                                <span>{price.pricing_model} · {price.billing_mode}</span>
                                {getDisplayPrice() && (
                                  <span className="font-medium text-text-primary">
                                    {getDisplayPrice()}
                                  </span>
                                )}
                              </div>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="min-w-[220px] align-top">
                          {/* 2. Hide Meter field for Flat pricing */}
                          {price && price.pricing_model === "FLAT" ? null : (
                            <div className="space-y-1">
                              {(() => {
                                const price = item.priceId ? priceLookup.get(item.priceId) : undefined
                                const isFlat = price?.pricing_model === "FLAT"
                                // price.pricing_model for usage is usually PER_UNIT or USAGE_BASED depending on type defs

                                const hasSingleMeter = item.priceId &&
                                  getMeterOptions(item.priceId).length === 1 &&
                                  !isLoadingMeters

                                // Redundant check but keeps logic clear if refactored later
                                if (isFlat) {
                                  return null
                                }

                                if (price && hasSingleMeter && item.meterId) {
                                  // Hidden if auto-selected and single option, 
                                  // just show the meter name or nothing? 
                                  // User said "field meter tidak perlu ada".
                                  // Let's show the meter name as text for confirmation, effectively "removing" the field input.
                                  const meter = meterLookup.get(item.meterId)
                                  return (
                                    <div className="flex h-10 items-center text-sm text-text-primary">
                                      {meter ? meter.name : item.meterId}
                                    </div>
                                  )
                                }

                                return (
                                  <Select
                                    value={item.meterId}
                                    onValueChange={(value) => handleMeterChange(item.id, value)}
                                    disabled={!item.priceId || Boolean(isLoadingMeters)}
                                  >
                                    <SelectTrigger className="w-full">
                                      <SelectValue
                                        placeholder={
                                          !item.priceId
                                            ? "Select a price first"
                                            : isLoadingMeters
                                              ? "Loading meters..."
                                              : "Select a meter"
                                        }
                                      />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {!item.priceId && (
                                        <SelectItem value="empty" disabled>
                                          Select a price first
                                        </SelectItem>
                                      )}
                                      {isLoadingMeters && (
                                        <SelectItem value="loading" disabled>
                                          Loading meters...
                                        </SelectItem>
                                      )}
                                      {!isLoadingMeters &&
                                        item.priceId &&
                                        meterOptions.length === 0 && (
                                          <SelectItem value="empty" disabled>
                                            No meters configured
                                          </SelectItem>
                                        )}
                                      {!isLoadingMeters &&
                                        meterOptions.map((meterId) => (
                                          <SelectItem key={meterId} value={meterId}>
                                            {formatMeterLabel(meterLookup.get(meterId), meterId)}
                                          </SelectItem>
                                        ))}
                                    </SelectContent>
                                  </Select>
                                )
                              })()}
                              {priceAmountError && (
                                <p className="text-status-error text-xs">
                                  {priceAmountError}
                                </p>
                              )}
                              {meterError && (
                                <p className="text-status-error text-xs">{meterError}</p>
                              )}
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="align-top">
                          {/* 3. Disable Quantity input when pricing_model != FLAT */}
                          <Input
                            type="number"
                            min={1}
                            max={127}
                            value={price && price.pricing_model !== "FLAT" ? "1" : item.quantity}
                            onChange={(e) => handleQuantityChange(item.id, e.target.value)}
                            disabled={!item.priceId || (price && price.pricing_model !== "FLAT")}
                            className={price && price.pricing_model !== "FLAT" ? "bg-background-subtle text-text-muted cursor-not-allowed" : ""}
                          />
                        </TableCell>
                        <TableCell className="align-top text-right">
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            aria-label="Remove item"
                            onClick={() => handleRemoveItem(item.id)}
                            disabled={items.length === 1}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" variant="outline" size="sm" onClick={handleAddItem}>
                <Plus className="size-4" />
                Add item
              </Button>
              {pricesError && <p className="text-status-error text-sm">{pricesError}</p>}
              {metersError && <p className="text-status-error text-sm">{metersError}</p>}
            </div>
          </CardContent>
        </Card>

        {error && <Alert variant="destructive">{error}</Alert>}

        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating..." : "Create subscription"}
          </Button>
          <Button variant="outline" asChild>
            <Link to={subscriptionsPath}>Cancel</Link>
          </Button>
        </div>
      </form>
    </div>
  )
}
