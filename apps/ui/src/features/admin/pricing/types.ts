export type Price = {
  id: string
  name?: string | null
  code?: string | null
  description?: string | null
  pricing_model?: string | null
  billing_unit?: string | null
  billing_interval?: string | null
  billing_interval_count?: number | null
  created_at?: string
  metadata?: Record<string, unknown> | null
}

export type PriceAmount = {
  id: string
  price_id: string
  meter_id: string
  currency: string
  amount?: number | null
  unit_amount_cents?: number | null
  effective_from: string
  effective_to?: string | null
  created_at?: string
}

export type Currency = {
  code: string
  name: string
  symbol?: string | null
  minor_unit?: number | null
}
