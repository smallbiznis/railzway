import {
  CUSTOMER_ID,
  METER_CODE,
  WEBHOOK_PAYLOAD,
  WEBHOOK_HEADERS,
} from '../config/env.js';

function randomSuffix() {
  return Math.random().toString(36).slice(2, 10);
}

export function buildIdempotencyKey(prefix = 'k6') {
  return `${prefix}-${Date.now()}-${randomSuffix()}`;
}

export function ensureUsageEnv() {
  if (!CUSTOMER_ID || !METER_CODE) {
    throw new Error('CUSTOMER_ID and METER_CODE are required');
  }
}

export function usagePayload({ idempotencyKey, recordedAt, value } = {}) {
  ensureUsageEnv();

  const payload = {
    customer_id: CUSTOMER_ID,
    meter_code: METER_CODE,
    value: typeof value === 'number' ? value : 1,
    recorded_at: recordedAt || new Date().toISOString(),
    idempotency_key: idempotencyKey || buildIdempotencyKey(),
  };

  return JSON.stringify(payload);
}

export function webhookPayload() {
  if (!WEBHOOK_PAYLOAD) {
    throw new Error('WEBHOOK_PAYLOAD is required');
  }
  return WEBHOOK_PAYLOAD;
}

export function webhookHeaders() {
  const base = { 'Content-Type': 'application/json' };
  if (!WEBHOOK_HEADERS) {
    return base;
  }

  let parsed;
  try {
    parsed = JSON.parse(WEBHOOK_HEADERS);
  } catch (err) {
    throw new Error('WEBHOOK_HEADERS must be valid JSON');
  }

  return { ...base, ...parsed };
}
