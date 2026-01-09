import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { useFieldArray, useForm } from "react-hook-form"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Form,
  FormControl,
  FormDescription as FormHint,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type PricingModel = "FLAT" | "USAGE_BASED"
type BillingInterval = "DAY" | "WEEK" | "MONTH" | "YEAR"
type AggregationType = "SUM" | "MAX" | "AVG"

type UsageRate = {
  meter_id: string
  unit_amount: number | null
  minimum_amount: number | null
  maximum_amount: number | null
  aggregation: AggregationType
}

type MeterOption = {
  id: string
  name?: string
  code?: string
}

type ProductSummary = {
  id: string
  name?: string
  code?: string
  active?: boolean
}

type CreatePriceFormValues = {
  price: {
    name: string
    pricing_model: PricingModel
    billing_interval: BillingInterval
    billing_interval_count: number
    currency: string
  }
  flat: {
    unit_amount: number | null
  }
  usage: {
    rates: UsageRate[]
  }
}

type OrchestrationError =
  | { stage: "price"; message: string; detail?: string }
  | { stage: "amount"; message: string; detail?: string }

const pricingModelOptions: Array<{ label: string; value: PricingModel }> = [
  { label: "Flat", value: "FLAT" },
  { label: "Usage-based", value: "USAGE_BASED" },
]

const billingIntervalOptions: Array<{ label: string; value: BillingInterval }> = [
  { label: "Day", value: "DAY" },
  { label: "Week", value: "WEEK" },
  { label: "Month", value: "MONTH" },
  { label: "Year", value: "YEAR" },
]

const aggregationOptions: Array<{ label: string; value: AggregationType }> = [
  { label: "Sum", value: "SUM" },
  { label: "Max", value: "MAX" },
  { label: "Avg", value: "AVG" },
]

const buildPriceCode = (productCode: string, pricingModel: PricingModel) =>
  `${productCode}-${pricingModel.toLowerCase().replace(/_/g, "-")}`

const getErrorMessage = (err: unknown, fallback: string) => {
  if (typeof err === "object" && err !== null) {
    const errorMessage = (err as any)?.response?.data?.error?.message
    if (errorMessage) return errorMessage
    const message = (err as any)?.message
    if (message) return message
  }
  return fallback
}

const toNumberOrNull = (value: string) => {
  if (value.trim() === "") return null
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return null
  return parsed
}

export default function CreatePrice() {
  const { orgId, productId } = useParams()
  const navigate = useNavigate()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [product, setProduct] = useState<ProductSummary | null>(null)
  const [productLoading, setProductLoading] = useState(true)
  const [productError, setProductError] = useState<string | null>(null)
  const [meters, setMeters] = useState<MeterOption[]>([])
  const [metersLoading, setMetersLoading] = useState(false)
  const [metersError, setMetersError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [createdPriceId, setCreatedPriceId] = useState<string | null>(null)
  const [orchestrationError, setOrchestrationError] =
    useState<OrchestrationError | null>(null)

  const form = useForm<CreatePriceFormValues>({
    mode: "onBlur",
    defaultValues: {
      price: {
        name: "",
        pricing_model: "FLAT",
        billing_interval: "MONTH",
        billing_interval_count: 1,
        currency: "USD",
      },
      flat: {
        unit_amount: null,
      },
      usage: {
        rates: [
          {
            meter_id: "",
            unit_amount: null,
            minimum_amount: null,
            maximum_amount: null,
            aggregation: "SUM",
          },
        ],
      },
    },
  })

  const pricingModel = form.watch("price.pricing_model")
  const isUsageBased = pricingModel === "USAGE_BASED"

  const usageFields = useFieldArray({
    control: form.control,
    name: "usage.rates",
  })

  useEffect(() => {
    if (!orgId || !productId) {
      setProductLoading(false)
      return
    }

    let isMounted = true
    setProductLoading(true)
    setProductError(null)

    admin
      .get(`/products/${productId}`)
      .then((response) => {
        if (!isMounted) return
        setProduct(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        setProductError(err?.message ?? "Unable to load product.")
      })
      .finally(() => {
        if (!isMounted) return
        setProductLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, productId])

  useEffect(() => {
    if (!orgId) {
      setMeters([])
      setMetersLoading(false)
      setMetersError(null)
      return
    }

    let isMounted = true
    setMetersLoading(true)
    setMetersError(null)

    admin
      .get("/meters")
      .then((response) => {
        if (!isMounted) return
        setMeters(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        setMetersError(err?.message ?? "Unable to load meters.")
      })
      .finally(() => {
        if (!isMounted) return
        setMetersLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  if (!canManage) {
    return <ForbiddenState description="You do not have access to create prices." />
  }

  const priceCodePreview = useMemo(() => {
    const trimmed = product?.code?.trim()
    if (!trimmed) return "auto-generated after product code"
    return buildPriceCode(trimmed, pricingModel)
  }, [pricingModel, product])

  const createPrice = async (values: CreatePriceFormValues) => {
    if (!orgId || !productId) {
      throw new Error("Missing organization context.")
    }
    if (!product?.code) {
      throw new Error("Product code is required for price generation.")
    }

    const priceCode = buildPriceCode(product.code.trim(), values.price.pricing_model)
    const priceName = values.price.name.trim()
    const usageBased = values.price.pricing_model === "USAGE_BASED"
    const payload = {
      organization_id: orgId,
      product_id: productId,
      code: priceCode,
      name: priceName,
      pricing_model: usageBased ? "PER_UNIT" : "FLAT",
      billing_mode: usageBased ? "METERED" : "LICENSED",
      billing_interval: values.price.billing_interval,
      billing_interval_count: values.price.billing_interval_count,
      tax_behavior: "INCLUSIVE",
      aggregate_usage: usageBased ? "SUM" : undefined,
      billing_unit: usageBased ? "API_CALL" : undefined,
    }
    const response = await admin.post("/prices", payload)
    return response.data?.data
  }

  const createPriceAmounts = async (values: CreatePriceFormValues, priceId: string) => {
    if (!orgId) {
      throw new Error("Missing organization context.")
    }

    const currency = values.price.currency.trim()
    if (!currency) {
      form.setError("price.currency", { message: "Currency is required." })
      throw new Error("Missing currency.")
    }

    if (values.price.pricing_model === "FLAT") {
      if (values.flat.unit_amount == null) {
        form.setError("flat.unit_amount", { message: "Unit price is required." })
        throw new Error("Missing unit amount.")
      }
      await admin.post("/price_amounts", {
        organization_id: orgId,
        price_id: priceId,
        meter_id: null,
        currency,
        unit_amount_cents: Math.round(values.flat.unit_amount),
      })
      return
    }

    if (!values.usage.rates.length) {
      form.setError("usage.rates", { message: "Add at least one meter rate." })
      throw new Error("Missing usage rates.")
    }

    const payloads = values.usage.rates.map((rate, index) => {
      if (!rate.meter_id) {
        form.setError(`usage.rates.${index}.meter_id`, { message: "Meter is required." })
        throw new Error("Missing meter.")
      }
      if (rate.unit_amount == null) {
        form.setError(`usage.rates.${index}.unit_amount`, { message: "Unit price is required." })
        throw new Error("Missing unit amount.")
      }
      if (
        rate.minimum_amount != null &&
        rate.maximum_amount != null &&
        rate.maximum_amount < rate.minimum_amount
      ) {
        form.setError(`usage.rates.${index}.maximum_amount`, {
          message: "Maximum charge must be greater than minimum charge.",
        })
        throw new Error("Invalid minimum/maximum.")
      }

      return {
        organization_id: orgId,
        price_id: priceId,
        meter_id: rate.meter_id,
        currency,
        unit_amount_cents: Math.round(rate.unit_amount),
        minimum_amount_cents: rate.minimum_amount == null ? undefined : Math.round(rate.minimum_amount),
        maximum_amount_cents: rate.maximum_amount == null ? undefined : Math.round(rate.maximum_amount),
        metadata: { aggregation: rate.aggregation },
      }
    })

    await Promise.all(payloads.map((payload) => admin.post("/price_amounts", payload)))
  }

  const handleNavigateToProduct = () => {
    if (!orgId || !productId) return
    navigate(`/orgs/${orgId}/products/${productId}`)
  }

  const runCreateFlow = async (values: CreatePriceFormValues) => {
    setIsSubmitting(true)
    setOrchestrationError(null)

    let priceId = createdPriceId

    try {
      if (!priceId) {
        const price = await createPrice(values)
        priceId = price?.id ?? null
        setCreatedPriceId(priceId)
      }

      if (!priceId) {
        throw new Error("Price created without an ID.")
      }

      await createPriceAmounts(values, priceId)
      handleNavigateToProduct()
    } catch (err) {
      const detail = getErrorMessage(err, "Something went wrong.")
      if (!priceId) {
        setOrchestrationError({
          stage: "price",
          message: "Unable to create price.",
          detail,
        })
      } else {
        setOrchestrationError({
          stage: "amount",
          message: "Price created, but amount setup failed.",
          detail,
        })
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const retryAmounts = async () => {
    if (!createdPriceId) return
    const values = form.getValues()
    setIsSubmitting(true)
    setOrchestrationError(null)

    try {
      await createPriceAmounts(values, createdPriceId)
      handleNavigateToProduct()
    } catch (err) {
      const detail = getErrorMessage(err, "Something went wrong.")
      setOrchestrationError({
        stage: "amount",
        message: "Price created, but amount setup failed.",
        detail,
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  const submitDisabled = isSubmitting || productLoading || !product || !!productError
  const productLabel = product?.name || product?.code || product?.id || "Product"

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Add price</h1>
          <p className="text-text-muted text-sm">
            Attach another price to {productLabel}.
          </p>
        </div>
        {orgId && productId && (
          <Button asChild variant="outline" size="sm">
            <Link to={`/orgs/${orgId}/products/${productId}`}>Back to product</Link>
          </Button>
        )}
      </div>

      {productLoading && (
        <div className="flex items-center gap-2 text-text-muted text-sm">
          <Spinner className="size-4" />
          Loading product...
        </div>
      )}
      {productError && <div className="text-status-error text-sm">{productError}</div>}

      {orchestrationError && (
        <Alert variant="destructive">
          <AlertTitle>{orchestrationError.message}</AlertTitle>
          <AlertDescription>
            {orchestrationError.detail && <p>{orchestrationError.detail}</p>}
            {orchestrationError.stage === "amount" && (
              <div className="pt-2">
                <Button size="sm" onClick={retryAmounts} disabled={isSubmitting}>
                  {isSubmitting ? "Retrying..." : "Retry amount"}
                </Button>
              </div>
            )}
          </AlertDescription>
        </Alert>
      )}

      <Form {...form}>
        <form onSubmit={form.handleSubmit(runCreateFlow)} className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Price setup</CardTitle>
              <CardDescription>Define how this product will be charged.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 md:grid-cols-2">
                <FormField
                  control={form.control}
                  name="price.pricing_model"
                  rules={{ required: "Pricing model is required." }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Pricing model</FormLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="Select a pricing model" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {pricingModelOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="price.name"
                  rules={{ required: "Price name is required." }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Price name</FormLabel>
                      <FormControl>
                        <Input placeholder="Starter monthly" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="price.currency"
                  rules={{ required: "Currency is required." }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Currency</FormLabel>
                      <FormControl>
                        <Input placeholder="USD" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="price.billing_interval"
                  rules={{ required: "Billing interval is required." }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Billing interval</FormLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="Select interval" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {billingIntervalOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="price.billing_interval_count"
                  rules={{
                    required: "Interval count is required.",
                    min: { value: 1, message: "Must be at least 1." },
                  }}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Interval count</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          min={1}
                          value={field.value}
                          onChange={(event) => {
                            const parsed = Number(event.target.value)
                            field.onChange(Number.isNaN(parsed) ? 1 : parsed)
                          }}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="rounded-lg border bg-bg-subtle/30 p-3 text-xs text-text-muted">
                Price code preview:{" "}
                <span className="font-medium text-text-primary">{priceCodePreview}</span>
              </div>

              <div className="space-y-4 rounded-lg border p-4">
                <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                  <div className="space-y-1">
                    <p className="text-sm font-medium">
                      {isUsageBased ? "Usage rates" : "Unit price"}
                    </p>
                    <p className="text-text-muted text-xs">
                      {isUsageBased
                        ? "This price is charged based on usage measured by a meter."
                        : "Set the amount charged per billing interval."}
                    </p>
                  </div>
                </div>

                {isUsageBased ? (
                  <div className="space-y-4">
                    {metersLoading && (
                      <p className="text-text-muted text-sm">Loading meters...</p>
                    )}
                    {metersError && <Alert variant="destructive">{metersError}</Alert>}
                    {!metersLoading && !metersError && meters.length === 0 && (
                      <p className="text-text-muted text-sm">
                        No meters found. Create a meter first.
                      </p>
                    )}
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <p className="text-text-muted text-xs">
                        Add one or more meter rates for usage-based pricing.
                      </p>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          usageFields.append({
                            meter_id: "",
                            unit_amount: null,
                            minimum_amount: null,
                            maximum_amount: null,
                            aggregation: "SUM",
                          })
                        }
                        disabled={metersLoading || meters.length === 0}
                      >
                        Add meter rate
                      </Button>
                    </div>
                    {form.formState.errors.usage?.rates &&
                      !Array.isArray(form.formState.errors.usage.rates) && (
                        <p className="text-status-error text-sm">
                          {(form.formState.errors.usage.rates as { message?: string })
                            ?.message ?? "Check the meter rates."}
                        </p>
                      )}
                    <div className="space-y-3">
                      {usageFields.fields.map((rate, index) => (
                        <div key={rate.id} className="grid gap-4 rounded-lg border p-4 md:grid-cols-2">
                          <FormField
                            control={form.control}
                            name={`usage.rates.${index}.meter_id`}
                            rules={{
                              required: isUsageBased ? "Meter is required." : false,
                            }}
                            render={({ field }) => (
                              <FormItem className="md:col-span-2">
                                <FormLabel>Meter</FormLabel>
                                <Select value={field.value} onValueChange={field.onChange}>
                                  <FormControl>
                                    <SelectTrigger>
                                      <SelectValue placeholder="Select a meter" />
                                    </SelectTrigger>
                                  </FormControl>
                                  <SelectContent>
                                    {meters.map((meter) => (
                                      <SelectItem key={meter.id} value={meter.id}>
                                        {meter.name || meter.code || meter.id}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`usage.rates.${index}.unit_amount`}
                            rules={{
                              required: isUsageBased ? "Unit price is required." : false,
                              min: { value: 0, message: "Amount cannot be negative." },
                            }}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>Unit price (minor unit)</FormLabel>
                                <FormControl>
                                  <Input
                                    type="number"
                                    min={0}
                                    placeholder="200"
                                    value={field.value ?? ""}
                                    onChange={(event) =>
                                      field.onChange(toNumberOrNull(event.target.value))
                                    }
                                  />
                                </FormControl>
                                <FormHint>Example: 200 = $2.00 per unit</FormHint>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`usage.rates.${index}.aggregation`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>Aggregation</FormLabel>
                                <Select value={field.value} onValueChange={field.onChange}>
                                  <FormControl>
                                    <SelectTrigger>
                                      <SelectValue placeholder="Select aggregation" />
                                    </SelectTrigger>
                                  </FormControl>
                                  <SelectContent>
                                    {aggregationOptions.map((option) => (
                                      <SelectItem key={option.value} value={option.value}>
                                        {option.label}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`usage.rates.${index}.minimum_amount`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>Minimum charge (optional)</FormLabel>
                                <FormControl>
                                  <Input
                                    type="number"
                                    min={0}
                                    placeholder="0"
                                    value={field.value ?? ""}
                                    onChange={(event) =>
                                      field.onChange(toNumberOrNull(event.target.value))
                                    }
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`usage.rates.${index}.maximum_amount`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>Maximum charge (optional)</FormLabel>
                                <FormControl>
                                  <Input
                                    type="number"
                                    min={0}
                                    placeholder="10000"
                                    value={field.value ?? ""}
                                    onChange={(event) =>
                                      field.onChange(toNumberOrNull(event.target.value))
                                    }
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <div className="md:col-span-2 flex justify-end">
                            <Button
                              type="button"
                              size="sm"
                              variant="ghost"
                              onClick={() => usageFields.remove(index)}
                              disabled={usageFields.fields.length === 1}
                            >
                              Remove rate
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className="grid gap-4 md:grid-cols-2">
                    <FormField
                      control={form.control}
                      name="flat.unit_amount"
                      rules={{
                        required: pricingModel === "FLAT" ? "Unit price is required." : false,
                        min: { value: 0, message: "Amount cannot be negative." },
                      }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Unit price (minor unit)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              min={0}
                              placeholder="5000"
                              value={field.value ?? ""}
                              onChange={(event) =>
                                field.onChange(toNumberOrNull(event.target.value))
                              }
                            />
                          </FormControl>
                          <FormHint>Example: 5000 = $50.00</FormHint>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          <div className="flex flex-wrap items-center gap-3">
            <Button type="submit" disabled={submitDisabled}>
              {isSubmitting ? "Creating..." : "Create price"}
            </Button>
            <p className="text-text-muted text-xs">
              Price and amount are created in sequence.
            </p>
          </div>
        </form>
      </Form>
    </div>
  )
}
