import { expect, test } from "@playwright/test"

type Invoice = {
  status?: string
  Status?: string
  paid_at?: string
  PaidAt?: string
  due_at?: string
  DueAt?: string
}

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const invoicesPath = `/orgs/${orgId}/invoices`

const readField = (invoice: Invoice, keys: (keyof Invoice)[]) => {
  for (const key of keys) {
    const value = invoice[key]
    if (typeof value === "string" && value.trim()) {
      return value
    }
  }
  return ""
}

const deriveStatus = (invoice: Invoice) => {
  const raw = readField(invoice, ["status", "Status"]).toUpperCase() || "UNKNOWN"
  const paidAt = readField(invoice, ["paid_at", "PaidAt"])
  const dueAt = readField(invoice, ["due_at", "DueAt"])
  const dueDate = dueAt ? new Date(dueAt) : null
  if (paidAt || raw === "PAID") return "PAID"
  if (raw === "VOID") return "VOID"
  if (raw === "UNCOLLECTIBLE") return "UNCOLLECTIBLE"
  if (raw === "DRAFT") return "DRAFT"
  if (raw === "OPEN" || raw === "ISSUED") {
    if (dueDate && dueDate.getTime() < Date.now()) return "PAST_DUE"
    return "OPEN"
  }
  if (dueDate && dueDate.getTime() < Date.now()) return "PAST_DUE"
  return raw
}

const formatStatus = (status?: string) => {
  switch (status) {
    case "PAST_DUE":
      return "Past due"
    case "DRAFT":
      return "Draft"
    case "OPEN":
      return "Open"
    case "PAID":
      return "Paid"
    case "VOID":
      return "Void"
    case "UNCOLLECTIBLE":
      return "Uncollectible"
    default:
      return status || "-"
  }
}

test("renders invoices list and empty state", async ({ page }) => {
  const customersResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/customers") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })
  const invoicesResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/invoices") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(invoicesPath)

  await expect(
    page.getByRole("heading", { name: "Invoices", level: 1 })
  ).toBeVisible()
  await expect(
    page.getByRole("button", { name: /create invoice/i }).first()
  ).toBeVisible()

  await customersResponsePromise
  const invoicesResponse = await invoicesResponsePromise
  const payload = await invoicesResponse.json().catch(() => ({}))
  const invoices = Array.isArray(payload?.data) ? payload.data : []

  if (invoices.length > 0) {
    await expect(page.getByRole("table")).toBeVisible()
    await expect(
      page.getByRole("columnheader", { name: /total/i })
    ).toBeVisible()
    await expect(
      page.getByRole("columnheader", { name: /invoice number/i })
    ).toBeVisible()
    await expect(
      page.getByRole("columnheader", { name: /customer/i })
    ).toBeVisible()

    const statusLabel = formatStatus(deriveStatus(invoices[0] as Invoice))
    if (statusLabel && statusLabel !== "-") {
      await expect(page.getByText(statusLabel)).toBeVisible()
    }
  } else {
    await expect(page.getByText(/no invoices yet/i)).toBeVisible()
  }
})
