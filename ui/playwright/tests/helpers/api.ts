import { expect } from "@playwright/test"
import type { APIResponse } from "@playwright/test"

export const buildSuffix = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

export const buildCode = (prefix: string) => `${prefix}-${buildSuffix()}`

export const ensureBaseURL = (value?: string | null) => {
  if (!value) {
    throw new Error("Playwright baseURL is required for API requests.")
  }
  return value
}

export const expectStatus = async (
  response: APIResponse,
  status: number,
  context: string
) => {
  const actual = response.status()
  if (actual !== status) {
    const body = await response.text().catch(() => "")
    throw new Error(`${context} failed (status ${actual}). ${body}`)
  }
}

export const readJson = async <T = any>(response: APIResponse): Promise<T> => {
  const text = await response.text()
  if (!text) return {} as T
  try {
    return JSON.parse(text) as T
  } catch (err) {
    throw new Error(`Unable to parse JSON response: ${(err as Error).message}`)
  }
}

export const poll = async <T>(
  action: () => Promise<T>,
  validate: (value: T) => boolean,
  label: string,
  timeoutMs = 60_000
) => {
  return expect.poll(async () => {
    const value = await action()
    if (!validate(value)) {
      throw new Error(`${label} not ready`)
    }
    return value
  }, { timeout: timeoutMs }).toBeTruthy()
}

export const sleep = (ms: number) =>
  new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
