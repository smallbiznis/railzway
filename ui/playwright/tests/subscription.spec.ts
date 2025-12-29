import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const subscriptionsPath = `/orgs/${orgId}/subscriptions`

const formatStatus = (value?: string) => {
  if (!value) return "-"
  switch (value.toUpperCase()) {
    case "ACTIVE":
      return "Active"
    case "PAUSED":
      return "Paused"
    case "CANCELED":
      return "Canceled"
    case "ENDED":
      return "Ended"
    case "DRAFT":
      return "Draft"
    default:
      return value
  }
}

test("views subscriptions list with status badges and actions", async ({ page }) => {
  const subscriptionsResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/subscriptions") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(subscriptionsPath)

  await expect(
    page.getByRole("heading", { name: "Subscriptions", level: 1 })
  ).toBeVisible()
  await expect(page.getByRole("link", { name: /create subscription/i })).toBeVisible()
  await expect(page.getByRole("tab", { name: "Active" })).toBeVisible()
  await expect(page.getByRole("tab", { name: "Canceled" })).toBeVisible()

  const subscriptionsResponse = await subscriptionsResponsePromise
  const payload = await subscriptionsResponse.json().catch(() => ({}))
  const subscriptions = Array.isArray(payload?.data) ? payload.data : []

  if (subscriptions.length > 0) {
    const table = page.getByRole("table")
    await expect(table).toBeVisible()
    const rows = table.getByRole("row")
    expect(await rows.count()).toBeGreaterThan(1)

    const firstRow = rows.nth(1)
    await expect(firstRow).toBeVisible()
    await expect(
      firstRow.getByRole("button", { name: /open subscription actions/i })
    ).toBeVisible()

    const rawStatus = subscriptions[0]?.status || subscriptions[0]?.Status || "-"
    const statusLabel = formatStatus(String(rawStatus))
    if (statusLabel && statusLabel !== "-") {
      await expect(firstRow.getByText(statusLabel)).toBeVisible()
    }
  } else {
    await expect(page.getByText(/no subscriptions yet/i)).toBeVisible()
  }
})
