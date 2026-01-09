import type { APIRequestContext } from "@playwright/test"

import { expectStatus, readJson } from "./api"

export type APIKeySecret = {
  key_id: string
  api_key: string
}

export const createUsageApiKey = async (
  request: APIRequestContext,
  name: string
) => {
  const response = await request.post("/admin/api-keys", {
    data: { name, scopes: ["usage:ingest"] },
  })
  await expectStatus(response, 200, "Create usage API key")
  const body = await readJson<APIKeySecret>(response)
  return body
}

export const buildIdempotencyKey = (prefix: string) => {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  return `${prefix}-${suffix}`
}

export const ingestUsage = async (
  request: APIRequestContext,
  payload: {
    apiKey: string
    customerId: string
    meterCode: string
    value: number
    recordedAt: string
    idempotencyKey: string
  }
) => {
  const response = await request.post("/usage", {
    headers: {
      Authorization: `Bearer ${payload.apiKey}`,
    },
    data: {
      customer_id: payload.customerId,
      meter_code: payload.meterCode,
      value: payload.value,
      recorded_at: payload.recordedAt,
      idempotency_key: payload.idempotencyKey,
    },
  })
  await expectStatus(response, 200, "Ingest usage")
  return readJson(response)
}
