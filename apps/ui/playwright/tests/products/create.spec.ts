import { expect, test } from "@playwright/test"

const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const createPath = `/orgs/${orgId}/products/create`

test("creates a product with flat pricing", async ({ page }) => {
  const uniqueToken = `${Date.now()}-${test.info().workerIndex}`
  const productName = `E2E Product ${uniqueToken}`
  const productCode = `e2e-product-${uniqueToken}`
  const priceName = `E2E Price ${uniqueToken}`

  const metersResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/meters") &&
      response.request().method() === "GET" &&
      response.status() === 200
    )
  })
  const createProductResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/products") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })
  const createPriceResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/prices") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })
  const createAmountResponsePromise = page.waitForResponse((response) => {
    return (
      response.url().includes("/api/price_amounts") &&
      response.request().method() === "POST" &&
      response.status() === 200
    )
  })

  await page.goto(createPath)

  await expect(
    page.getByRole("heading", { name: /create product/i, level: 1 })
  ).toBeVisible()
  await metersResponsePromise

  await page.getByTestId("product-name").fill(productName)
  await page.getByTestId("product-code").fill(productCode)
  await page.getByTestId("price-name").fill(priceName)
  await page.getByTestId("product-amount").fill("5000")
  await page.getByTestId("product-submit").click()

  const [productResponse, priceResponse, amountResponse] = await Promise.all([
    createProductResponsePromise,
    createPriceResponsePromise,
    createAmountResponsePromise,
  ])

  expect(priceResponse.status()).toBe(200)
  expect(amountResponse.status()).toBe(200)

  const productPayload = await productResponse.json().catch(() => ({}))
  const productId = productPayload?.data?.id
  expect(productId).toBeTruthy()

  await expect(page).toHaveURL(
    new RegExp(`/orgs/${orgId}/products/${productId}$`)
  )
  await expect(
    page.getByRole("heading", { name: productName, level: 1 })
  ).toBeVisible()
})
