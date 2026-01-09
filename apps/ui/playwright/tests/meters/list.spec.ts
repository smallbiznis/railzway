import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const metersPath = `/orgs/${orgId}/meter`

test("renders meters list and empty state", async ({ page }) => {
  const metersResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/meters") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(metersPath)

  await expect(
    page.getByRole("heading", { name: "Meters", level: 1 })
  ).toBeVisible()
  await expect(
    page.getByRole("link", { name: /create meter/i })
  ).toBeVisible()

  const metersResponse = await metersResponsePromise
  const payload = await metersResponse.json().catch(() => ({}))
  const meters = Array.isArray(payload?.data) ? payload.data : []

  if (meters.length > 0) {
    const table = page.getByRole("table")
    await expect(table).toBeVisible()
    const rows = table.getByRole("row")
    expect(await rows.count()).toBeGreaterThan(1)
  } else {
    await expect(page.getByText(/no meters yet/i)).toBeVisible()
  }
})
