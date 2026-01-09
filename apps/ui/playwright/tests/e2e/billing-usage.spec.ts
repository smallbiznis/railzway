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

test.describe("billing lifecycle @usage", () => {
  test.skip(!e2eMode, "VALORA_E2E_MODE is not enabled.")

  test("usage price invoices metered totals @usage", async ({ page, playwright }, testInfo) => {
    const baseURL = ensureBaseURL(testInfo.project.use.baseURL as string | undefined)
    const { request, orgId } = await createAdminContext(playwright, baseURL, resolveOrgId())
    const usageRequest = await createUsageContext(playwright, baseURL)

    try {
      const product = await createProduct(request, {
        code: buildCode("prod-usage"),
        name: "Usage plan",
      })

      const meter = await createMeter(request, {
        code: buildCode("meter-usage"),
        name: "API calls",
        aggregation: "SUM",
        unit: "call",
      })

      const price = await createPrice(request, {
        productId: product.id,
        code: buildCode("price-usage"),
        name: "Usage per call",
        pricingModel: "PER_UNIT",
        billingMode: "METERED",
        billingInterval: "MONTH",
        billingIntervalCount: 1,
        aggregateUsage: "SUM",
        billingUnit: "API_CALL",
        taxBehavior: "EXCLUSIVE",
      })

      const unitAmountCents = 250
      const amount = await createPriceAmount(request, {
        priceId: price.id,
        meterId: meter.id,
        currency: "USD",
        unitAmountCents,
        effectiveFrom: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
      })

      const customer = await createCustomer(request, {
        name: "Usage customer",
        email: `${buildCode("usage")}@example.com`,
      })

      const subscription = await createSubscription(request, {
        customerId: customer.id,
        collectionMode: "SEND_INVOICE",
        billingCycleType: "MONTHLY",
        items: [{ price_id: price.id, meter_id: meter.id, quantity: 1 }],
      })

      await activateSubscription(request, subscription.id)
      await ensureBillingCycles(request)

      const apiKey = await createUsageApiKey(request, buildCode("usage-key"))
      const usageValue = 12
      await ingestUsage(usageRequest, {
        apiKey: apiKey.api_key,
        customerId: customer.id,
        meterCode: meter.code,
        value: usageValue,
        recordedAt: new Date().toISOString(),
        idempotencyKey: buildIdempotencyKey("usage"),
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

      const expectedSubtotal = usageValue * unitAmountCents
      const invoiceDetail = await getInvoice(request, invoice.id)
      expect(invoiceDetail.subtotal_amount).toBe(expectedSubtotal)

      const rendered = await renderInvoice(request, invoice.id)
      expect(rendered).toContain(formatMoney(expectedSubtotal, amount.currency))
      expect(countInvoiceRows(rendered)).toBeGreaterThanOrEqual(1)

      await page.goto(`/orgs/${orgId}/subscriptions/${subscription.id}`)
      await expect(page.getByRole("heading", { name: /subscription/i })).toBeVisible()
    } finally {
      await request.dispose()
      await usageRequest.dispose()
    }
  })
})
