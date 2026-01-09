import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"

const navigationItems = [
  { label: "Products", path: "products", heading: "Products" },
  { label: "Meters", path: "meter", heading: "Meters" },
  { label: "Customers", path: "customers", heading: "Customers" },
  { label: "Subscriptions", path: "subscriptions", heading: "Subscriptions" },
  { label: "Invoices", path: "invoices", heading: "Invoices" },
  { label: "Settings", path: "settings", heading: "Settings" },
]

test("sidebar navigation routes to each section", async ({ page }) => {
  await page.goto(`/orgs/${orgId}/dashboard`)

  for (const item of navigationItems) {
    await test.step(`navigate to ${item.label}`, async () => {
      await page.getByRole("link", { name: item.label }).click()
      await expect(page).toHaveURL(new RegExp(`/orgs/${orgId}/${item.path}$`))
      await expect(
        page.getByRole("heading", { name: item.heading, level: 1 })
      ).toBeVisible()
    })
  }
})
