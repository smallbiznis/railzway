import { expect, test } from "@playwright/test"

import { buildCode, ensureBaseURL, sleep } from "../helpers/api"
import { createAdminContext, createUsageContext, resolveOrgId } from "../helpers/auth"
import {
  activateSubscription,
  countInvoiceRows,
  createCustomer,
  createMeter,
  createPrice,
  createPriceAmount,
  createProduct,
  createSubscription,
  ensureBillingCycles,
  formatMoney,
  getInvoice,
  getSubscription,
  listInvoicesByCustomer,
  renderInvoice,
  runSchedulerOnce,
  fastForwardSubscriptionCycle,
} from "../helpers/billing"
import { buildIdempotencyKey, createUsageApiKey, ingestUsage } from "../helpers/usage"

const e2eMode = process.env.VALORA_E2E_MODE === "true"
const cycleDelayMs = Number(process.env.E2E_CYCLE_ADVANCE_DELAY_MS || 65_000)

test.describe("billing lifecycle @hybrid", () => {
  test.skip(!e2eMode, "VALORA_E2E_MODE is not enabled.")

  test("flat + usage subscription invoices both components @hybrid", async ({ page, playwright }, testInfo) => {
    const baseURL = ensureBaseURL(testInfo.project.use.baseURL as string | undefined)
    const { request, orgId } = await createAdminContext(playwright, baseURL, resolveOrgId())
    const usageRequest = await createUsageContext(playwright, baseURL)

    try {
      const product = await createProduct(request, {
        code: buildCode("prod-hybrid"),
        name: "Hybrid plan",
      })

      const baseMeter = await createMeter(request, {
        code: buildCode("meter-base"),
        name: "Base entitlement",
        aggregation: "SUM",
        unit: "seat",
      })

      const usageMeter = await createMeter(request, {
        code: buildCode("meter-addon"),
        name: "Addon usage",
        aggregation: "SUM",
        unit: "call",
      })

      const flatPrice = await createPrice(request, {
        productId: product.id,
        code: buildCode("price-base"),
        name: "Base subscription",
        pricingModel: "FLAT",
        billingMode: "LICENSED",
        billingInterval: "MONTH",
        billingIntervalCount: 1,
        taxBehavior: "EXCLUSIVE",
      })

      const usagePrice = await createPrice(request, {
        productId: product.id,
        code: buildCode("price-addon"),
        name: "Addon usage",
        pricingModel: "PER_UNIT",
        billingMode: "METERED",
        billingInterval: "MONTH",
        billingIntervalCount: 1,
        aggregateUsage: "SUM",
        billingUnit: "API_CALL",
        taxBehavior: "EXCLUSIVE",
      })

      const flatAmount = await createPriceAmount(request, {
        priceId: flatPrice.id,
        meterId: baseMeter.id,
        currency: "USD",
        unitAmountCents: 7_500,
        effectiveFrom: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
      })

      const usageUnitAmount = 150
      const usageAmount = await createPriceAmount(request, {
        priceId: usagePrice.id,
        meterId: usageMeter.id,
        currency: "USD",
        unitAmountCents: usageUnitAmount,
        effectiveFrom: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
      })

      const customer = await createCustomer(request, {
        name: "Hybrid customer",
        email: `${buildCode("hybrid")}@example.com`,
      })

      const subscription = await createSubscription(request, {
        customerId: customer.id,
        collectionMode: "SEND_INVOICE",
        billingCycleType: "MONTHLY",
        items: [
          { price_id: flatPrice.id, meter_id: baseMeter.id, quantity: 1 },
          { price_id: usagePrice.id, meter_id: usageMeter.id, quantity: 1 },
        ],
      })

      await activateSubscription(request, subscription.id)
      await ensureBillingCycles(request)

      const apiKey = await createUsageApiKey(request, buildCode("hybrid-key"))
      const usageValue = 20
      await ingestUsage(usageRequest, {
        apiKey: apiKey.api_key,
        customerId: customer.id,
        meterCode: usageMeter.code,
        value: usageValue,
        recordedAt: new Date().toISOString(),
        idempotencyKey: buildIdempotencyKey("hybrid"),
      })

      const activeSubscription = await getSubscription(request, subscription.id)
      expect(activeSubscription.status).toBe("ACTIVE")

      // Allow the usage snapshot worker to enrich events before closing the cycle.
      await sleep(cycleDelayMs)

      await fastForwardSubscriptionCycle(request, subscription.id)
      await runSchedulerOnce(request)

      await expect.poll(
        async () => (await listInvoicesByCustomer(request, customer.id)).length,
        { timeout: 90_000 }
      ).toBeGreaterThan(0)

      const invoices = await listInvoicesByCustomer(request, customer.id)
      const invoice = invoices.sort((a, b) => {
        return new Date(b.created_at || 0).getTime() - new Date(a.created_at || 0).getTime()
      })[0]

      const expectedSubtotal = flatAmount.unit_amount_cents + usageValue * usageUnitAmount
      const invoiceDetail = await getInvoice(request, invoice.id)
      expect(invoiceDetail.subtotal_amount).toBe(expectedSubtotal)

      const rendered = await renderInvoice(request, invoice.id)
      expect(rendered).toContain(formatMoney(flatAmount.unit_amount_cents, flatAmount.currency))
      expect(rendered).toContain(formatMoney(usageValue * usageUnitAmount, usageAmount.currency))
      expect(countInvoiceRows(rendered)).toBeGreaterThanOrEqual(2)

      await page.goto(`/orgs/${orgId}/invoices/${invoice.id}`)
      await expect(page.getByRole("heading", { name: /invoice/i })).toBeVisible()
    } finally {
      await request.dispose()
      await usageRequest.dispose()
    }
  })
})
