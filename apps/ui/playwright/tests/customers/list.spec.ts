import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const customersPath = `/orgs/${orgId}/customers`

test("renders customers list and empty state", async ({ page }) => {
  const customersResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/customers") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(customersPath)

  await expect(
    page.getByRole("heading", { name: "Customers", level: 1 })
  ).toBeVisible()
  await expect(
    page.getByRole("textbox", { name: /search customers/i })
  ).toBeVisible()
  await expect(
    page.getByRole("textbox", { name: /filter by balance/i })
  ).toBeVisible()

  const customersResponse = await customersResponsePromise
  const payload = await customersResponse.json().catch(() => ({}))
  const customers = Array.isArray(payload?.data?.customers)
    ? payload.data.customers
    : []

  if (customers.length > 0) {
    const table = page.getByRole("table")
    await expect(table).toBeVisible()
    const rows = table.getByRole("row")
    expect(await rows.count()).toBeGreaterThan(1)
    const firstName = customers[0]?.name
    if (firstName) {
      await expect(page.getByText(firstName)).toBeVisible()
    }
  } else {
    await expect(page.getByText(/no customers yet/i)).toBeVisible()
  }
})
