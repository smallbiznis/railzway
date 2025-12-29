import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const adminPassword = process.env.E2E_ADMIN_PASSWORD || "admin"
const apiKeysPath = `/orgs/${orgId}/api-keys`

test("creates, rotates, and revokes API keys with password confirmation", async ({ page }) => {
  const uniqueToken = `${Date.now()}-${test.info().workerIndex}`
  const keyName = `E2E Key ${uniqueToken}`

  const listResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.goto(apiKeysPath)

  await expect(
    page.getByRole("heading", { name: /api keys/i, level: 1 })
  ).toBeVisible()

  const listResponse = await listResponsePromise
  const listPayload = await listResponse.json().catch(() => [])
  const existingKeys = Array.isArray(listPayload) ? listPayload : []

  if (existingKeys.length > 0) {
    const table = page.getByRole("table")
    await expect(table).toBeVisible()
    await expect(table.getByText(/\*{4}/).first()).toBeVisible()
  } else {
    await expect(page.getByText(/no api keys yet/i)).toBeVisible()
  }

  const createResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })
  const refreshAfterCreatePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await page.getByRole("button", { name: /create api key/i }).click()
  await page.getByLabel("Name").fill(keyName)
  await page.getByRole("button", { name: /create key/i }).click()

  await createResponsePromise
  await refreshAfterCreatePromise

  const createDialog = page.getByRole("dialog", { name: /create api key/i })
  const createdSecretInput = createDialog.locator("input[readonly]")
  await expect(createdSecretInput).toBeVisible()
  const createdSecret = await createdSecretInput.inputValue()
  expect(createdSecret).not.toEqual("")

  await createDialog.getByRole("button", { name: "Done" }).click()

  const keyRow = page.getByRole("row").filter({ hasText: keyName }).first()
  await expect(keyRow).toBeVisible()

  const keyIdCell = keyRow.getByRole("cell").nth(1)
  const originalKeyId = (await keyIdCell.innerText()).trim()
  expect(originalKeyId.startsWith("****")).toBeTruthy()

  const rotateResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys/") &&
      response.url().includes("/reveal") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })
  const refreshResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/admin/api-keys") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })

  await keyRow.getByRole("button", { name: "Rotate" }).click()
  const revealDialog = page.getByRole("dialog", { name: /reveal/i })
  await expect(revealDialog).toBeVisible()
  await revealDialog.getByRole("button", { name: /confirm & rotate/i }).click()
  await expect(revealDialog.getByText("Password is required.")).toBeVisible()

  await revealDialog.getByLabel("Password").fill(adminPassword)
  await revealDialog.getByRole("button", { name: /confirm & rotate/i }).click()
  await rotateResponsePromise

  await expect(revealDialog.locator("input[readonly]")).toBeVisible()
  await revealDialog.getByRole("button", { name: "Done" }).click()

  await refreshResponsePromise

  const revokedRow = page.getByRole("row").filter({ hasText: originalKeyId })
  await expect(revokedRow.getByText("Revoked")).toBeVisible()
})
