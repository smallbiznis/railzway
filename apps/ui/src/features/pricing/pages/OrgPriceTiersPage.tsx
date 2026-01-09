import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { PricingNav } from "@/features/pricing/components/PricingNav"

type PriceTier = {
  id: string
  price_id: string
  tier_mode: number
  start_quantity: number
  end_quantity?: number | null
  unit_amount_cents?: number | null
  flat_amount_cents?: number | null
  unit: string
}

type Price = {
  id: string
  name?: string
  code?: string
}

const formatRange = (start: number, end?: number | null) => {
  if (!end && end !== 0) return `${start}+`
  return `${start} - ${end}`
}

const formatTierMode = (value: number) => `Mode ${value}`

const formatCents = (amount: number) => `${amount}¢`

export default function OrgPriceTiersPage() {
  const { orgId } = useParams()
  const [tiers, setTiers] = useState<PriceTier[]>([])
  const [prices, setPrices] = useState<Price[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const [priceId, setPriceId] = useState("")
  const [tierMode, setTierMode] = useState("1")
  const [startQuantity, setStartQuantity] = useState("0")
  const [endQuantity, setEndQuantity] = useState("")
  const [unitAmount, setUnitAmount] = useState("")
  const [flatAmount, setFlatAmount] = useState("")
  const [unit, setUnit] = useState("unit")

  const loadData = async () => {
    if (!orgId) return
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)
    try {
      const [tierRes, priceRes] = await Promise.all([
        admin.get("/price_tiers"),
        admin.get("/prices"),
      ])
      setTiers(tierRes.data?.data ?? [])
      setPrices(priceRes.data?.data ?? [])
    } catch (err) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
      } else {
        setError(getErrorMessage(err, "Unable to load price tiers."))
      }
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    void loadData()
  }, [orgId])

  const priceLookup = useMemo(() => {
    const map = new Map<string, Price>()
    prices.forEach((price) => {
      map.set(price.id, price)
    })
    return map
  }, [prices])

  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading price tiers...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to price tiers." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Price tiers</h1>
          <p className="text-text-muted text-sm">
            Define slab ranges for tiered billing.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => setIsCreateOpen(true)}>
            Create tier
          </Button>
        </div>
      </div>

      <PricingNav />

      {tiers.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No price tiers yet</EmptyTitle>
            <EmptyDescription>
              Add tiers for tiered pricing models.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow>
                <TableHead>Price</TableHead>
                <TableHead>Range</TableHead>
                <TableHead>Unit amount</TableHead>
                <TableHead>Flat amount</TableHead>
                <TableHead>Mode</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tiers.map((tier) => {
                const price = priceLookup.get(tier.price_id)
                return (
                  <TableRow key={tier.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="font-medium">{price?.name || tier.price_id}</span>
                        <span className="text-text-muted text-xs">{price?.code || tier.price_id}</span>
                      </div>
                    </TableCell>
                    <TableCell>{formatRange(tier.start_quantity, tier.end_quantity)}</TableCell>
                    <TableCell>
                      {tier.unit_amount_cents !== null && tier.unit_amount_cents !== undefined
                        ? formatCents(tier.unit_amount_cents)
                        : "-"}
                    </TableCell>
                    <TableCell>
                      {tier.flat_amount_cents !== null && tier.flat_amount_cents !== undefined
                        ? formatCents(tier.flat_amount_cents)
                        : "-"}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{formatTierMode(tier.tier_mode)}</Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button asChild variant="ghost" size="sm">
                        <Link to={`${orgBasePath}/price-tiers/${tier.id}`}>View</Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog
        open={isCreateOpen}
        onOpenChange={(open) => {
          setIsCreateOpen(open)
          if (!open) {
            setCreateError(null)
          }
        }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Create tier</DialogTitle>
            <DialogDescription>
              Tier definitions are append-only and affect future billing.
            </DialogDescription>
          </DialogHeader>
          {createError && <div className="text-status-error text-sm">{createError}</div>}
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="tier-price">Price</Label>
              <Select value={priceId} onValueChange={setPriceId}>
                <SelectTrigger id="tier-price">
                  <SelectValue placeholder="Select price" />
                </SelectTrigger>
                <SelectContent>
                  {prices.map((price) => (
                    <SelectItem key={price.id} value={price.id}>
                      {price.name || price.code || price.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-mode">Tier mode</Label>
              <Select value={tierMode} onValueChange={setTierMode}>
                <SelectTrigger id="tier-mode">
                  <SelectValue placeholder="Select mode" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">Mode 1</SelectItem>
                  <SelectItem value="2">Mode 2</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-start">Start quantity</Label>
              <Input
                id="tier-start"
                type="number"
                value={startQuantity}
                onChange={(event) => setStartQuantity(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-end">End quantity (optional)</Label>
              <Input
                id="tier-end"
                type="number"
                value={endQuantity}
                onChange={(event) => setEndQuantity(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-unit-amount">Unit amount (cents)</Label>
              <Input
                id="tier-unit-amount"
                type="number"
                value={unitAmount}
                onChange={(event) => setUnitAmount(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-flat-amount">Flat amount (cents)</Label>
              <Input
                id="tier-flat-amount"
                type="number"
                value={flatAmount}
                onChange={(event) => setFlatAmount(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tier-unit">Unit</Label>
              <Input
                id="tier-unit"
                value={unit}
                onChange={(event) => setUnit(event.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setIsCreateOpen(false)}
              disabled={isCreating}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={isCreating}
              onClick={async () => {
                if (!priceId) {
                  setCreateError("Price is required.")
                  return
                }
                const start = Number(startQuantity)
                const end = endQuantity.trim() ? Number(endQuantity) : undefined
                const unitAmountCents = unitAmount.trim() ? Number(unitAmount) : undefined
                const flatAmountCents = flatAmount.trim() ? Number(flatAmount) : undefined

                if (Number.isNaN(start)) {
                  setCreateError("Start quantity must be a number.")
                  return
                }

                setIsCreating(true)
                setCreateError(null)
                try {
                  await admin.post("/price_tiers", {
                    price_id: priceId,
                    tier_mode: Number(tierMode),
                    start_quantity: start,
                    end_quantity: end,
                    unit_amount_cents: unitAmountCents,
                    flat_amount_cents: flatAmountCents,
                    unit: unit.trim() || "unit",
                  })
                  await loadData()
                  setIsCreateOpen(false)
                } catch (err) {
                  setCreateError(getErrorMessage(err, "Unable to create tier."))
                } finally {
                  setIsCreating(false)
                }
              }}
            >
              {isCreating ? "Creating..." : "Confirm create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
