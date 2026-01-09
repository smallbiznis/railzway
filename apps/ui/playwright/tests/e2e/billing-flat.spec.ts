import { expect, test } from "@playwright/test"

import { buildCode, ensureBaseURL, sleep } from "../helpers/api"
import { createAdminContext, resolveOrgId } from "../helpers/auth"
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

const e2eMode = process.env.VALORA_E2E_MODE === "true"
const cycleDelayMs = Number(process.env.E2E_CYCLE_ADVANCE_DELAY_MS || 65_000)

test.describe("billing lifecycle @flat", () => {
  test.skip(!e2eMode, "VALORA_E2E_MODE is not enabled.")

  test("flat price generates invoice after cycle close @flat", async ({ page, playwright }, testInfo) => {
    const baseURL = ensureBaseURL(testInfo.project.use.baseURL as string | undefined)
    const { request, orgId } = await createAdminContext(playwright, baseURL, resolveOrgId())

    try {
      const product = await createProduct(request, {
        code: buildCode("prod-flat"),
        name: "Flat plan",
      })

      const meter = await createMeter(request, {
        code: buildCode("meter-flat"),
        name: "Flat entitlement",
        aggregation: "SUM",
        unit: "seat",
      })

      const price = await createPrice(request, {
        productId: product.id,
        code: buildCode("price-flat"),
        name: "Monthly flat",
        pricingModel: "FLAT",
        billingMode: "LICENSED",
        billingInterval: "MONTH",
        billingIntervalCount: 1,
        taxBehavior: "EXCLUSIVE",
      })

      const amount = await createPriceAmount(request, {
        priceId: price.id,
        meterId: meter.id,
        currency: "USD",
        unitAmountCents: 5_000,
        effectiveFrom: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
      })

      const customer = await createCustomer(request, {
        name: "Flat customer",
        email: `${buildCode("flat")}@example.com`,
      })

      const subscription = await createSubscription(request, {
        customerId: customer.id,
        collectionMode: "SEND_INVOICE",
        billingCycleType: "MONTHLY",
        items: [{ price_id: price.id, meter_id: meter.id, quantity: 1 }],
      })

      await activateSubscription(request, subscription.id)
      await ensureBillingCycles(request)

      const activated = await getSubscription(request, subscription.id)
      expect(activated.status).toBe("ACTIVE")

      // Billing cycles are month-based; wait long enough to safely fast-forward the period end.
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

      const invoiceDetail = await getInvoice(request, invoice.id)
      expect(invoiceDetail.customer_id).toBe(customer.id)
      expect(invoiceDetail.subtotal_amount).toBe(amount.unit_amount_cents)

      const rendered = await renderInvoice(request, invoice.id)
      expect(rendered).toContain(formatMoney(amount.unit_amount_cents, amount.currency))
      expect(countInvoiceRows(rendered)).toBeGreaterThanOrEqual(1)

      await page.goto(`/orgs/${orgId}/invoices/${invoice.id}`)
      await expect(page.getByRole("heading", { name: /invoice/i })).toBeVisible()
    } finally {
      await request.dispose()
    }
  })
})
