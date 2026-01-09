import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const adminUsername = process.env.E2E_ADMIN_USERNAME || "admin"
const adminEmail =
  process.env.E2E_ADMIN_EMAIL ||
  (adminUsername.includes("@") ? adminUsername : "admin@valora.cloud")
const adminPassword = process.env.E2E_ADMIN_PASSWORD || "admin"

const criticalNavItems = [
  { label: "Subscriptions", path: "subscriptions", heading: "Subscriptions" },
  { label: "API Keys", path: "api-keys", heading: "API keys" },
  { label: "Audit Logs", path: "audit-logs", heading: "Audit log" },
]

test.describe("login flow", () => {
  test.use({ storageState: { cookies: [], origins: [] } })

  test("signs in with admin credentials and reaches dashboard", async ({ page }) => {
    await page.goto("/login")

    await expect(page.getByTestId("login-email")).toBeVisible()
    await expect(page.getByTestId("login-password")).toBeVisible()
    await expect(page.getByTestId("login-submit")).toBeEnabled()
    await expect(page.getByRole("link", { name: /sign up/i })).toHaveCount(0)

    await page.getByTestId("login-email").fill(adminEmail)
    await page.getByTestId("login-password").fill(adminPassword)
    await page.getByTestId("login-submit").click()

    await expect(page).toHaveURL(new RegExp(`/orgs/${orgId}/dashboard$`))
    await expect(
      page.getByRole("heading", { name: /welcome/i, level: 1 })
    ).toBeVisible()
    await expect(page.getByText("m@example.com")).toBeVisible()
  })
})

test("admin can reach critical navigation areas", async ({ page }) => {
  await page.goto(`/orgs/${orgId}/dashboard`)

  for (const item of criticalNavItems) {
    await test.step(`open ${item.label}`, async () => {
      await page.getByRole("link", { name: item.label }).click()
      await expect(page).toHaveURL(new RegExp(`/orgs/${orgId}/${item.path}$`))
      await expect(
        page.getByRole("heading", { name: item.heading, level: 1 })
      ).toBeVisible()
    })
  }
})
