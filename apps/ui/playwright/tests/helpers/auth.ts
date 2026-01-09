import type { APIRequestContext, Playwright } from "@playwright/test"

import { ensureBaseURL, expectStatus } from "./api"

const defaultOrgId = process.env.E2E_ORG_ID || "2002990275537932288"
const username = process.env.E2E_ADMIN_USERNAME || "admin"
const email =
  process.env.E2E_ADMIN_EMAIL ||
  (username.includes("@") ? username : "admin@valora.cloud")
const password = process.env.E2E_ADMIN_PASSWORD || "admin"
const sessionToken = process.env.E2E_SESSION_TOKEN || ""

export type AdminContext = {
  request: APIRequestContext
  orgId: string
}

export const resolveOrgId = () => defaultOrgId

export const createAdminContext = async (
  playwright: Playwright,
  baseURL?: string | null,
  orgId = defaultOrgId
): Promise<AdminContext> => {
  const resolvedBaseURL = ensureBaseURL(baseURL)
  const extraHeaders: Record<string, string> = {
    "X-Org-Id": orgId,
  }
  if (sessionToken) {
    extraHeaders.Cookie = `_sid=${sessionToken}`
  }

  const request = await playwright.request.newContext({
    baseURL: resolvedBaseURL,
    extraHTTPHeaders: extraHeaders,
  })

  if (!sessionToken) {
    const loginResponse = await request.post("/auth/login", {
      data: { email, password },
      failOnStatusCode: false,
    })
    await expectStatus(loginResponse, 200, "Admin login")
  }

  const orgResponse = await request.post(`/auth/user/using/${orgId}`, {
    failOnStatusCode: false,
  })
  await expectStatus(orgResponse, 200, "Set active org")

  return { request, orgId }
}

export const createUsageContext = async (
  playwright: Playwright,
  baseURL?: string | null
) => {
  const resolvedBaseURL = ensureBaseURL(baseURL)
  return playwright.request.newContext({ baseURL: resolvedBaseURL })
}
