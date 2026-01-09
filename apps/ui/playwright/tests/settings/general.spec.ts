import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const settingsPath = `/orgs/${orgId}/settings`

test("loads workspace settings", async ({ page }) => {
  await page.goto(settingsPath)

  await expect(
    page.getByRole("heading", { name: "Settings", level: 1 })
  ).toBeVisible()
  await expect(page.getByText("Workspace")).toBeVisible()
  await expect(page.getByText(/ID:/i)).toBeVisible()
})
