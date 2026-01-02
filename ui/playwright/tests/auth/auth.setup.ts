import path from "path"
import { fileURLToPath } from "url"
import { mkdir, writeFile } from "fs/promises"

import { test } from "@playwright/test"
import type { StorageState } from "@playwright/test"

const currentFilePath = fileURLToPath(import.meta.url)
const currentDir = path.dirname(currentFilePath)
const storageStatePath = path.join(currentDir, "..", "..", "storage", "admin.json")
const orgId = process.env.E2E_ORG_ID || "2002990275537932288"
const username = process.env.E2E_ADMIN_USERNAME || "admin"
const email =
  process.env.E2E_ADMIN_EMAIL ||
  (username.includes("@") ? username : "admin@valora.cloud")
const password = process.env.E2E_ADMIN_PASSWORD || "admin"
const sessionToken = process.env.E2E_SESSION_TOKEN || ""

const buildAuthState = (user: { id: string; username: string; email?: string }) =>
  JSON.stringify({
    state: {
      user,
      isAuthenticated: true,
    },
    version: 0,
  })

test("authenticate admin and persist storage state", async ({ request, baseURL }) => {
  if (!baseURL) {
    throw new Error("baseURL is required in Playwright config.")
  }

  const origin = new URL(baseURL).origin
  const apiBaseURL = process.env.E2E_API_BASE_URL || baseURL
  const defaultUser = {
    id: process.env.E2E_ADMIN_USER_ID || "admin",
    username,
    email,
  }

  let storageState: StorageState = await request.storageState()
  let user = defaultUser

  if (sessionToken) {
    const useOrgResponse = await request.post(`${apiBaseURL}/auth/user/using/${orgId}`, {
      headers: { Cookie: `_sid=${sessionToken}` },
    })
    if (useOrgResponse.status() !== 200) {
      throw new Error(
        `Unable to set org context (status ${useOrgResponse.status()}). Check E2E_SESSION_TOKEN or org access.`
      )
    }

    const url = new URL(baseURL)
    storageState.cookies = [
      {
        name: "_sid",
        value: sessionToken,
        domain: url.hostname,
        path: "/",
        httpOnly: true,
        secure: url.protocol === "https:",
        sameSite: "Lax",
        expires: -1,
      },
    ]
  } else {
    const loginResponse = await request.post(`${apiBaseURL}/auth/login`, {
      data: { email, password },
      failOnStatusCode: false,
    })

    if (loginResponse.status() !== 200) {
      throw new Error(
        `Admin login failed (${loginResponse.status()}). Set E2E_ADMIN_USERNAME/E2E_ADMIN_PASSWORD or E2E_SESSION_TOKEN.`
      )
    }

    const payload = await loginResponse.json()
    const metadata = payload?.metadata ?? {}
    const displayName = metadata.display_name || metadata.username || email
    user = {
      id: metadata.user_id || defaultUser.id,
      username: displayName || defaultUser.username,
      email: metadata.email || defaultUser.email,
    }

    const useOrgResponse = await request.post(`${apiBaseURL}/auth/user/using/${orgId}`)
    if (useOrgResponse.status() !== 200) {
      throw new Error(
        `Unable to set org context (status ${useOrgResponse.status()}). Check org membership for ${orgId}.`
      )
    }

    storageState = await request.storageState()
  }

  storageState.origins = [
    {
      origin,
      localStorage: [{ name: "valora-auth", value: buildAuthState(user) }],
    },
  ]

  await mkdir(path.dirname(storageStatePath), { recursive: true })
  await writeFile(storageStatePath, JSON.stringify(storageState, null, 2))
})
