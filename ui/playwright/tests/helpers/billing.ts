import type { APIRequestContext } from "@playwright/test"

import { expectStatus, readJson } from "./api"

export type Product = {
  id: string
  code: string
  name: string
  active: boolean
}

export type Meter = {
  id: string
  code: string
  name: string
  aggregation: string
  unit: string
  active: boolean
}

export type Price = {
  id: string
  code: string
  name?: string
  pricing_model: string
  billing_mode: string
  billing_interval: string
  billing_interval_count: number
  active: boolean
}

export type PriceAmount = {
  id: string
  price_id: string
  meter_id?: string | null
  currency: string
  unit_amount_cents: number
  effective_from: string
  effective_to?: string | null
}

export type Customer = {
  id: string
  name: string
  email: string
}

export type Subscription = {
  id: string
  status: string
  customer_id: string
  billing_cycle_type: string
}

export type Invoice = {
  id: string
  status: string
  subtotal_amount: number
  currency: string
  customer_id: string
  subscription_id: string
  created_at?: string
}

export const createProduct = async (
  request: APIRequestContext,
  payload: { code: string; name: string; description?: string }
) => {
  const response = await request.post("/admin/products", {
    data: {
      code: payload.code,
      name: payload.name,
      description: payload.description,
      active: true,
    },
  })
  await expectStatus(response, 200, "Create product")
  const body = await readJson<{ data: Product }>(response)
  return body.data
}

export const createMeter = async (
  request: APIRequestContext,
  payload: { code: string; name: string; aggregation: string; unit: string }
) => {
  const response = await request.post("/admin/meters", {
    data: {
      code: payload.code,
      name: payload.name,
      aggregation_type: payload.aggregation,
      unit: payload.unit,
      active: true,
    },
  })
  await expectStatus(response, 200, "Create meter")
  const body = await readJson<{ data: Meter }>(response)
  return body.data
}

export const createPrice = async (
  request: APIRequestContext,
  payload: {
    productId: string
    code: string
    name: string
    pricingModel: string
    billingMode: string
    billingInterval: string
    billingIntervalCount?: number
    taxBehavior?: string
    aggregateUsage?: string
    billingUnit?: string
  }
) => {
  const response = await request.post("/admin/prices", {
    data: {
      product_id: payload.productId,
      code: payload.code,
      name: payload.name,
      pricing_model: payload.pricingModel,
      billing_mode: payload.billingMode,
      billing_interval: payload.billingInterval,
      billing_interval_count: payload.billingIntervalCount ?? 1,
      aggregate_usage: payload.aggregateUsage,
      billing_unit: payload.billingUnit,
      tax_behavior: payload.taxBehavior ?? "EXCLUSIVE",
      active: true,
    },
  })
  await expectStatus(response, 200, "Create price")
  const body = await readJson<{ data: Price }>(response)
  return body.data
}

export const createPriceAmount = async (
  request: APIRequestContext,
  payload: {
    priceId: string
    meterId: string
    currency: string
    unitAmountCents: number
    effectiveFrom?: string
  }
) => {
  const response = await request.post("/admin/price_amounts", {
    data: {
      price_id: payload.priceId,
      meter_id: payload.meterId,
      currency: payload.currency,
      unit_amount_cents: payload.unitAmountCents,
      effective_from: payload.effectiveFrom,
    },
  })
  await expectStatus(response, 200, "Create price amount")
  const body = await readJson<{ data: PriceAmount }>(response)
  return body.data
}

export const createCustomer = async (
  request: APIRequestContext,
  payload: { name: string; email: string }
) => {
  const response = await request.post("/admin/customers", { data: payload })
  await expectStatus(response, 200, "Create customer")
  const body = await readJson<{ data: Customer }>(response)
  return body.data
}

export const createSubscription = async (
  request: APIRequestContext,
  payload: {
    customerId: string
    collectionMode: string
    billingCycleType: string
    items: Array<{ price_id: string; meter_id: string; quantity: number }>
  }
) => {
  const response = await request.post("/admin/subscriptions", {
    data: {
      customer_id: payload.customerId,
      collection_mode: payload.collectionMode,
      billing_cycle_type: payload.billingCycleType,
      items: payload.items,
    },
  })
  await expectStatus(response, 200, "Create subscription")
  const body = await readJson<{ data: Subscription }>(response)
  return body.data
}

export const activateSubscription = async (
  request: APIRequestContext,
  subscriptionId: string
) => {
  const response = await request.post(`/admin/subscriptions/${subscriptionId}/activate`)
  await expectStatus(response, 204, "Activate subscription")
}

export const getSubscription = async (
  request: APIRequestContext,
  subscriptionId: string
) => {
  const response = await request.get(`/admin/subscriptions/${subscriptionId}`)
  await expectStatus(response, 200, "Get subscription")
  const body = await readJson<{ data: Subscription }>(response)
  return body.data
}

export const ensureBillingCycles = async (request: APIRequestContext) => {
  const response = await request.post("/dev/billing/scheduler/ensure-cycles")
  await expectStatus(response, 200, "Ensure billing cycles")
}

export const fastForwardSubscriptionCycle = async (
  request: APIRequestContext,
  subscriptionId: string
) => {
  const response = await request.post(
    `/dev/billing/subscriptions/${subscriptionId}/fast-forward-cycle`
  )
  await expectStatus(response, 200, "Fast-forward subscription cycle")
}

export const runSchedulerOnce = async (request: APIRequestContext) => {
  const response = await request.post("/dev/billing/scheduler/run-once")
  await expectStatus(response, 200, "Run scheduler once")
}

export const listInvoicesByCustomer = async (
  request: APIRequestContext,
  customerId: string
) => {
  const response = await request.get("/admin/invoices", {
    params: { customer_id: customerId },
  })
  await expectStatus(response, 200, "List invoices")
  const body = await readJson<{ data: Invoice[] }>(response)
  return Array.isArray(body.data) ? body.data : []
}

export const getInvoice = async (request: APIRequestContext, invoiceId: string) => {
  const response = await request.get(`/admin/invoices/${invoiceId}`)
  await expectStatus(response, 200, "Get invoice")
  const body = await readJson<{ data: Invoice }>(response)
  return body.data
}

export const renderInvoice = async (
  request: APIRequestContext,
  invoiceId: string
) => {
  const response = await request.get(`/admin/invoices/${invoiceId}/render`)
  await expectStatus(response, 200, "Render invoice")
  const body = await readJson<{ data: { rendered_html: string } }>(response)
  return body.data?.rendered_html || ""
}

export const countInvoiceRows = (html: string) => {
  const match = html.match(/<tbody>([\s\S]*?)<\/tbody>/i)
  if (!match) return 0
  const rows = match[1].match(/<tr>/gi)
  return rows ? rows.length : 0
}

export const formatMoney = (amountCents: number, currency: string) => {
  const normalized = currency.toUpperCase()
  const value = (amountCents / 100).toFixed(2)
  return `${normalized} ${value}`
}
