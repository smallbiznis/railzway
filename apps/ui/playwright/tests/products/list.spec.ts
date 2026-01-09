import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const productsPath = `/orgs/${orgId}/products`

test("renders products list, filters, and empty state", async ({ page }) => {
  const productsResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/products") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(productsPath)

  await expect(
    page.getByRole("heading", { name: "Products", level: 1 })
  ).toBeVisible()
  await expect(
    page.getByRole("textbox", { name: /search products/i })
  ).toBeVisible()
  await expect(page.getByText("All")).toBeVisible()
  await expect(page.getByText("Active")).toBeVisible()
  await expect(page.getByText("Archived")).toBeVisible()
  await expect(
    page.getByRole("link", { name: /create product/i }).first()
  ).toBeVisible()

  const productsResponse = await productsResponsePromise
  const payload = await productsResponse.json().catch(() => ({}))
  const products = Array.isArray(payload?.data) ? payload.data : []

  if (products.length > 0) {
    const table = page.getByRole("table")
    await expect(table).toBeVisible()
    const rows = table.getByRole("row")
    expect(await rows.count()).toBeGreaterThan(1)
  } else {
    await expect(page.getByText(/no products yet/i)).toBeVisible()
  }
})
