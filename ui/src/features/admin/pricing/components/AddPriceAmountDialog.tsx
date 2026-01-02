import { useMemo, useState } from "react"
import { IconPlus } from "@tabler/icons-react"
import { useMutation, useQueryClient } from "@tanstack/react-query"

import { admin } from "@/api/client"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"

import type { Currency, PriceAmount } from "@/features/admin/pricing/types"
import {
  formatDateTime,
  getLatestEffectiveFrom,
  parseDate,
  resolveCurrency,
  toLocalDateTimeInputValue,
} from "@/features/admin/pricing/utils"

type AddPriceAmountDialogProps = {
  priceId: string
  priceName?: string | null
  currencies: Currency[]
  priceAmounts: PriceAmount[]
}

type ValidationErrors = {
  currency?: string
  amount?: string
  effectiveFrom?: string
  confirmation?: string
}

export function AddPriceAmountDialog({
  priceId,
  priceName,
  currencies,
  priceAmounts,
}: AddPriceAmountDialogProps) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [amount, setAmount] = useState("")
  const [effectiveFrom, setEffectiveFrom] = useState("")
  const [confirmed, setConfirmed] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const now = useMemo(() => new Date(), [open])
  const minDateValue = useMemo(() => toLocalDateTimeInputValue(now), [now])
  const inferredCurrencyCode = useMemo(() => {
    const fromAmounts = priceAmounts[0]?.currency
    if (fromAmounts) return fromAmounts.toUpperCase()
    const usd = currencies.find(
      (currency) => currency.code.toUpperCase() === "USD"
    )
    if (usd) return usd.code.toUpperCase()
    if (currencies.length > 0) {
      return currencies[0].code.toUpperCase()
    }
    return ""
  }, [currencies, priceAmounts])
  const selectedCurrency = useMemo(
    () => resolveCurrency(currencies, inferredCurrencyCode),
    [currencies, inferredCurrencyCode]
  )
  const parsedAmount = useMemo(() => {
    const trimmed = amount.trim()
    if (!/^\d+$/.test(trimmed)) return null
    const value = Number(trimmed)
    if (Number.isNaN(value)) return null
    return value
  }, [amount])
  const effectiveDate = useMemo(() => parseDate(effectiveFrom), [effectiveFrom])
  const latestEffective = useMemo(() => {
    if (!inferredCurrencyCode) return null
    return getLatestEffectiveFrom(priceAmounts, inferredCurrencyCode)
  }, [inferredCurrencyCode, priceAmounts])

  const validation = useMemo<ValidationErrors>(() => {
    const errors: ValidationErrors = {}
    if (!inferredCurrencyCode) {
      errors.currency = "Currency is required to create a price version."
    }
    if (!amount.trim()) {
      errors.amount = "Amount is required."
    } else if (!/^\d+$/.test(amount.trim())) {
      errors.amount = "Amount must be a whole number in minor units."
    } else if (parsedAmount == null || parsedAmount <= 0) {
      errors.amount = "Amount must be greater than zero."
    }
    if (!effectiveDate) {
      errors.effectiveFrom = "Effective from is required."
    } else if (effectiveDate.getTime() < now.getTime()) {
      errors.effectiveFrom = "Effective from must be now or later."
    } else if (
      latestEffective &&
      effectiveDate.getTime() <= latestEffective.getTime()
    ) {
      errors.effectiveFrom = `Effective from must be after ${formatDateTime(
        latestEffective.toISOString()
      )} to avoid overlap.`
    }
    if (!confirmed) {
      errors.confirmation = "Please confirm to continue."
    }
    return errors
  }, [
    amount,
    confirmed,
    effectiveDate,
    inferredCurrencyCode,
    latestEffective,
    now,
    parsedAmount,
  ])

  const scheduleWarning =
    effectiveDate && effectiveDate.getTime() > now.getTime()
      ? `This scheduled price will activate on ${formatDateTime(
          effectiveDate.toISOString()
        )}.`
      : null

  const mutation = useMutation({
    mutationFn: async () => {
      if (!priceId) {
        throw new Error("Missing price context.")
      }
      if (!effectiveDate) {
        throw new Error("Missing effective date.")
      }
      if (parsedAmount == null) {
        throw new Error("Amount is required.")
      }
      if (!inferredCurrencyCode) {
        throw new Error("Currency is required.")
      }

      // New amounts are append-only: we only add a future/current version.
      return admin.post("/price_amounts", {
        price_id: priceId,
        currency: inferredCurrencyCode.toUpperCase(),
        unit_amount_cents: parsedAmount,
        effective_from: effectiveDate.toISOString(),
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["prices"] })
      await queryClient.invalidateQueries({
        queryKey: ["price_amounts", priceId],
      })
      await queryClient.invalidateQueries({ queryKey: ["price", priceId] })
      setAmount("")
      setEffectiveFrom("")
      setConfirmed(false)
      setFormError(null)
      setOpen(false)
    },
    onError: (error: any) => {
      setFormError(error?.message ?? "Unable to add price amount.")
    },
  })

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault()
    setFormError(null)
    if (Object.keys(validation).length > 0) {
      return
    }
    mutation.mutate()
  }

  const disableSubmit =
    Object.keys(validation).length > 0 || mutation.isPending

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) {
          setFormError(null)
          setConfirmed(false)
        }
      }}
    >
      <DialogTrigger asChild>
        <Button size="sm">
          <IconPlus />
          Add new price version
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Add new price version</DialogTitle>
          <DialogDescription>
            Create a new effective price for{" "}
            <span className="font-medium">{priceName ?? "this price"}</span>.
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={handleSubmit}>
          {formError && (
            <Alert variant="destructive">
              <AlertTitle>Unable to save</AlertTitle>
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <Alert>
            <AlertTitle>Pricing versioning</AlertTitle>
            <AlertDescription>
              This will create a new price version. Existing subscriptions will
              only be affected for usage after the effective date.
            </AlertDescription>
          </Alert>
          {scheduleWarning && (
            <Alert>
              <AlertTitle>Scheduled price</AlertTitle>
              <AlertDescription>{scheduleWarning}</AlertDescription>
            </Alert>
          )}
          <div className="space-y-2">
            <div className="text-text-muted text-xs">Currency</div>
            <div className="text-sm font-medium">
              {selectedCurrency
                ? `${selectedCurrency.code} - ${selectedCurrency.name}`
                : inferredCurrencyCode || "-"}
            </div>
            {validation.currency && (
              <div className="text-status-error text-xs">
                {validation.currency}
              </div>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="amount">Amount (minor unit)</Label>
            <Input
              id="amount"
              type="number"
              inputMode="numeric"
              min={1}
              step={1}
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              placeholder="5000"
            />
            {validation.amount && (
              <div className="text-status-error text-xs">
                {validation.amount}
              </div>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="effective-from">Effective from</Label>
            <Input
              id="effective-from"
              type="datetime-local"
              min={minDateValue}
              value={effectiveFrom}
              onChange={(event) => setEffectiveFrom(event.target.value)}
            />
            {latestEffective && (
              <p className="text-text-muted text-xs">
                Latest version starts on {formatDateTime(latestEffective.toISOString())}.
              </p>
            )}
            {validation.effectiveFrom && (
              <div className="text-status-error text-xs">
                {validation.effectiveFrom}
              </div>
            )}
          </div>
          <div className="flex items-start gap-2">
            <Checkbox
              id="confirm-versioning"
              checked={confirmed}
              onCheckedChange={(value) => setConfirmed(Boolean(value))}
            />
            <Label
              htmlFor="confirm-versioning"
              className="text-sm leading-relaxed"
            >
              I understand this creates a new version and does not edit
              historical prices.
            </Label>
          </div>
          {validation.confirmation && (
            <div className="text-status-error text-xs">
              {validation.confirmation}
            </div>
          )}
          <DialogFooter className="gap-2 sm:gap-0">
            <Button type="submit" disabled={disableSubmit}>
              {mutation.isPending ? (
                <>
                  <Spinner />
                  Saving
                </>
              ) : (
                "Confirm new price version"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
