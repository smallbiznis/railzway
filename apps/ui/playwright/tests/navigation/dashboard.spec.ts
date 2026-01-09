import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"

test("loads the dashboard with org context", async ({ page }) => {
  const useOrgResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes(`/auth/user/using/${orgId}`) &&
      response.request().method() === "POST"
    )
  })
  const orgResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes(`/admin/orgs/${orgId}`) &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(`/orgs/${orgId}/dashboard`)

  const orgResponse = await orgResponsePromise
  const orgPayload = await orgResponse.json().catch(() => ({}))
  const orgName = orgPayload?.org?.name || `Org ${orgId}`

  await expect(
    page.getByRole("heading", { name: /welcome/i, level: 1 })
  ).toBeVisible()
  await expect(page.getByRole("navigation").first()).toBeVisible()
  await expect(page.getByRole("link", { name: "Products" })).toBeVisible()
  await expect(page.getByRole("button", { name: orgName })).toBeVisible()

  const useOrgResponse = await useOrgResponsePromise
  expect(useOrgResponse.status()).toBe(200)
})
