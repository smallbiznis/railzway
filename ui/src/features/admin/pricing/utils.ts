import type { Currency, PriceAmount } from "@/features/admin/pricing/types"

const DEFAULT_MINOR_UNIT = 2

export const toLocalDateTimeInputValue = (value: Date) => {
  const pad = (target: number) => String(target).padStart(2, "0")
  return [
    value.getFullYear(),
    pad(value.getMonth() + 1),
    pad(value.getDate()),
  ].join("-")
    + "T"
    + [pad(value.getHours()), pad(value.getMinutes())].join(":")
}

export const toUTCISOStringFromLocalInput = (value: string) => {
  // value: "2026-01-05T17:00"
  const local = new Date(value)
  if (Number.isNaN(local.getTime())) return null
  return local.toISOString()
}


export const parseDate = (value?: string | null) => {
  if (!value) return null

  // Force ISO or explicit timezone
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return null

  return parsed
}

export const formatDateTime = (value?: string | null) => {
  const parsed = parseDate(value)
  if (!parsed) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(parsed)
}

export const formatPricingModel = (model?: string | null) => {
  const normalized = model?.toUpperCase()
  if (!normalized) return "-"
  if (normalized === "FLAT") return "Flat"
  if (normalized === "PER_UNIT") return "Usage"
  if (normalized.startsWith("TIERED")) return "Tiered"
  return normalized.toLowerCase()
}

export const formatUnit = (unit?: string | null) => {
  if (!unit) return "-"
  return unit.toUpperCase().replace(/_/g, " ")
}

export const resolveCurrency = (currencies: Currency[], code?: string | null) => {
  if (!code) return null
  return currencies.find(
    (currency) => currency.code.toUpperCase() === code.toUpperCase()
  ) ?? null
}

export const getMinorUnit = (currency?: Currency | null) => {
  if (typeof currency?.minor_unit === "number") {
    return currency.minor_unit
  }
  return DEFAULT_MINOR_UNIT
}

export const getAmountValue = (
  amount: PriceAmount,
  currency?: Currency | null
) => {
  if (typeof amount.amount === "number") {
    return amount.amount
  }
  if (typeof amount.unit_amount_cents === "number") {
    const minorUnit = getMinorUnit(currency)
    return amount.unit_amount_cents / Math.pow(10, minorUnit)
  }
  return null
}

export const formatCurrencyAmount = (
  amount: PriceAmount,
  currency?: Currency | null
) => {
  const value = getAmountValue(amount, currency)
  if (value == null) return "-"
  const code = currency?.code?.toUpperCase() ?? amount.currency?.toUpperCase()
  if (!code) {
    return String(value)
  }
  const minorUnit = getMinorUnit(currency)
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: code,
      minimumFractionDigits: minorUnit,
      maximumFractionDigits: minorUnit,
    }).format(value)
  } catch {
    return `${value} ${code}`
  }
}

export const getLatestEffectiveFrom = (
  amounts: PriceAmount[],
  currencyCode: string,
  meterId?: string | null
) => {
  const matches = amounts.filter((amount) =>
    amount.currency?.toUpperCase() === currencyCode.toUpperCase()
    && (meterId ? amount.meter_id === meterId : true)
  )

  return matches.reduce<Date | null>((latest, amount) => {
    const parsed = parseDate(amount.effective_from)
    if (!parsed) return latest
    if (!latest || parsed.getTime() > latest.getTime()) {
      return parsed
    }
    return latest
  }, null)
}
