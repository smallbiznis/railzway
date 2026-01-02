import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL } from '../config/env.js';
import { apiHeaders, requireApiKey } from '../helpers/auth.js';
import { usagePayload, ensureUsageEnv } from '../helpers/payloads.js';

const vus = parseInt(__ENV.INGEST_VUS || '20', 10);
const duration = __ENV.INGEST_DURATION || '5m';

export const options = {
  scenarios: {

    // 1️⃣ Normal production traffic
    ingest_steady: {
      executor: 'constant-vus',
      vus: 10,
      duration: '5m',
      tags: { scenario: 'steady' },
    },

    // 2️⃣ Burst traffic (spike mendadak)
    ingest_burst: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50 },
        { duration: '30s', target: 0 },
      ],
      tags: { scenario: 'burst' },
    },

    // 3️⃣ Rate-limit pressure test
    ingest_rate_limit: {
      executor: 'constant-arrival-rate',
      rate: 100,          // req/s
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 50,
      maxVUs: 100,
      tags: { scenario: 'rate_limit' },
    },

    // 4️⃣ Concurrency conflict test (same customer+meter)
    ingest_concurrency: {
      executor: 'constant-vus',
      vus: 20,
      duration: '2m',
      tags: { scenario: 'concurrency' },
    },
  },
  thresholds: {
    // ✅ SLA hanya untuk response sukses (2xx)
    'http_req_duration{status:2xx,name:usage_ingest_load}': ['p(95)<300'],

    // ❌ Request gagal non-429 tidak boleh ada
    'http_req_failed{name:usage_ingest_load}': ['rate<0.01'],

    // ✅ Rate-limit diharapkan, tapi harus terkendali
    'http_reqs{status:429,name:usage_ingest_load}': ['rate<0.3'],
  },
};

export function setup() {
  requireApiKey();
  ensureUsageEnv();
}

const random = Math.floor(Math.random() * 1000) + 1;

export default function () {
  const payload = usagePayload({value: random});

  const res = http.post(`${BASE_URL}/api/usage`, payload, {
    headers: apiHeaders(),
    tags: {
      name: 'usage_ingest_load',
    },
  });

  check(res, {
    'status is 2xx or 429': (r) =>
      r.status >= 200 && r.status < 300 || r.status === 429,
  });
}
