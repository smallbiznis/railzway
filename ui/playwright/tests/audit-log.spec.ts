import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const apiKeysPath = `/orgs/${orgId}/api-keys`
const auditLogsPath = `/orgs/${orgId}/audit-logs`

test("records admin actions in the audit log", async ({ page }) => {
  const uniqueToken = `${Date.now()}-${test.info().workerIndex}`
  const keyName = `Audit Key ${uniqueToken}`

  const createResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })

  await page.goto(apiKeysPath)
  await expect(
    page.getByRole("heading", { name: /api keys/i, level: 1 })
  ).toBeVisible()

  await page.getByRole("button", { name: /create api key/i }).click()
  await page.getByLabel("Name").fill(keyName)
  await page.getByRole("button", { name: /create key/i }).click()
  await createResponsePromise
  const createDialog = page.getByRole("dialog", { name: /create api key/i })
  await createDialog.getByRole("button", { name: "Done" }).click()

  await page.goto(auditLogsPath)
  await expect(
    page.getByRole("heading", { name: /audit log/i, level: 1 })
  ).toBeVisible()

  await page
    .getByPlaceholder("Action (e.g. invoice.finalize)")
    .fill("api_key.created")

  const auditResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/audit-logs") &&
      response.url().includes("action=api_key.created") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.getByRole("button", { name: "Apply" }).click()
  await auditResponsePromise

  const row = page.getByRole("row").filter({ hasText: "api_key.created" }).first()
  await expect(row).toBeVisible()
  await expect(row.getByText(/user|system|api key/i)).toBeVisible()
})
