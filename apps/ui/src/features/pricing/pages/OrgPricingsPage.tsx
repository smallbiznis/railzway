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
import { Switch } from "@/components/ui/switch"
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

type Product = {
  id: string
  name?: string
  code?: string
}

type Pricing = {
  id: string
  product_id: string
  name?: string
  code?: string
  pricing_model: string
  billing_interval: string
  billing_interval_count: number
  billing_mode: string
  billing_unit?: string | null
  tax_behavior: string
  active?: boolean
}

const pricingModelOptions = [
  { label: "Flat", value: "FLAT" },
  { label: "Per unit", value: "PER_UNIT" },
  { label: "Tiered volume", value: "TIERED_VOLUME" },
  { label: "Tiered graduated", value: "TIERED_GRADUATED" },
]

const billingIntervalOptions = [
  { label: "Day", value: "DAY" },
  { label: "Week", value: "WEEK" },
  { label: "Month", value: "MONTH" },
  { label: "Year", value: "YEAR" },
]

const billingUnitOptions = ["API_CALL", "GB", "GiB", "MB", "MiB", "SECOND", "MINUTE", "HOUR", "SEAT"]

const taxBehaviorOptions = ["INCLUSIVE", "EXCLUSIVE", "INLINE"]

const formatInterval = (interval: string, count: number) => {
  if (!interval) return "-"
  const normalized = interval.toLowerCase()
  if (!count || count === 1) return `Every ${normalized}`
  return `Every ${count} ${normalized}${count === 1 ? "" : "s"}`
}

const formatModel = (value?: string) => {
  if (!value) return "-"
  return value
    .toLowerCase()
    .replace(/_/g, " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}

export default function OrgPricingsPage() {
  const { orgId } = useParams()
  const [pricings, setPricings] = useState<Pricing[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const [productId, setProductId] = useState("")
  const [code, setCode] = useState("")
  const [name, setName] = useState("")
  const [pricingModel, setPricingModel] = useState("FLAT")
  const [billingInterval, setBillingInterval] = useState("MONTH")
  const [billingIntervalCount, setBillingIntervalCount] = useState("1")
  const [billingUnit, setBillingUnit] = useState("API_CALL")
  const [taxBehavior, setTaxBehavior] = useState("INCLUSIVE")
  const [isActive, setIsActive] = useState(true)

  const loadData = async () => {
    if (!orgId) return
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)
    try {
      const [pricingRes, productRes] = await Promise.all([
        admin.get("/pricings"),
        admin.get("/products"),
      ])
      setPricings(pricingRes.data?.data ?? [])
      setProducts(productRes.data?.data ?? [])
    } catch (err) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
      } else {
        setError(getErrorMessage(err, "Unable to load pricing models."))
      }
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    void loadData()
  }, [orgId])

  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"
  const isMetered = pricingModel !== "FLAT"

  const productLabel = useMemo(() => {
    const match = products.find((product) => product.id === productId)
    if (!match) return "Select product"
    return match.name || match.code || match.id
  }, [productId, products])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading pricing models...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to pricing models." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Pricing models</h1>
          <p className="text-text-muted text-sm">
            Define pricing models before attaching amounts or tiers.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => setIsCreateOpen(true)}>
            Create pricing model
          </Button>
        </div>
      </div>

      <PricingNav />

      {pricings.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No pricing models yet</EmptyTitle>
            <EmptyDescription>
              Create a pricing model to describe how usage should be billed.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Code</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Interval</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pricings.map((pricing) => (
                <TableRow key={pricing.id}>
                  <TableCell className="font-medium">{pricing.name || "Untitled"}</TableCell>
                  <TableCell className="text-text-muted">{pricing.code || "-"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{formatModel(pricing.pricing_model)}</Badge>
                  </TableCell>
                  <TableCell>
                    {formatInterval(pricing.billing_interval, pricing.billing_interval_count)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={pricing.active ? "secondary" : "outline"}>
                      {pricing.active ? "Active" : "Inactive"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="ghost" size="sm">
                      <Link to={`${orgBasePath}/pricings/${pricing.id}`}>View</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
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
            <DialogTitle>Create pricing model</DialogTitle>
            <DialogDescription>
              Pricing models describe billing behavior. Add amounts or tiers after creation.
            </DialogDescription>
          </DialogHeader>
          {createError && <div className="text-status-error text-sm">{createError}</div>}
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="pricing-product">Product</Label>
              <Select value={productId} onValueChange={setProductId}>
                <SelectTrigger id="pricing-product">
                  <SelectValue placeholder={productLabel} />
                </SelectTrigger>
                <SelectContent>
                  {products.map((product) => (
                    <SelectItem key={product.id} value={product.id}>
                      {product.name || product.code || product.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="pricing-code">Code</Label>
              <Input
                id="pricing-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder="starter-flat"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="pricing-name">Name</Label>
              <Input
                id="pricing-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Starter plan"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="pricing-model">Pricing model</Label>
              <Select value={pricingModel} onValueChange={setPricingModel}>
                <SelectTrigger id="pricing-model">
                  <SelectValue placeholder="Select model" />
                </SelectTrigger>
                <SelectContent>
                  {pricingModelOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="pricing-interval">Billing interval</Label>
              <Select value={billingInterval} onValueChange={setBillingInterval}>
                <SelectTrigger id="pricing-interval">
                  <SelectValue placeholder="Select interval" />
                </SelectTrigger>
                <SelectContent>
                  {billingIntervalOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="pricing-interval-count">Interval count</Label>
              <Input
                id="pricing-interval-count"
                type="number"
                min="1"
                value={billingIntervalCount}
                onChange={(event) => setBillingIntervalCount(event.target.value)}
              />
            </div>
            {isMetered && (
              <div className="space-y-2">
                <Label htmlFor="pricing-unit">Billing unit</Label>
                <Select value={billingUnit} onValueChange={setBillingUnit}>
                  <SelectTrigger id="pricing-unit">
                    <SelectValue placeholder="Select unit" />
                  </SelectTrigger>
                  <SelectContent>
                    {billingUnitOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="pricing-tax">Tax behavior</Label>
              <Select value={taxBehavior} onValueChange={setTaxBehavior}>
                <SelectTrigger id="pricing-tax">
                  <SelectValue placeholder="Select tax behavior" />
                </SelectTrigger>
                <SelectContent>
                  {taxBehaviorOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-1">
              <Label htmlFor="pricing-active">Active</Label>
              <p className="text-text-muted text-xs">Inactive prices cannot be used for new subscriptions.</p>
            </div>
            <Switch id="pricing-active" checked={isActive} onCheckedChange={setIsActive} />
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
                if (!productId) {
                  setCreateError("Product is required.")
                  return
                }
                const intervalCount = Number(billingIntervalCount)
                if (!intervalCount || intervalCount < 1) {
                  setCreateError("Billing interval count must be at least 1.")
                  return
                }

                setIsCreating(true)
                setCreateError(null)
                try {
                  await admin.post("/pricings", {
                    product_id: productId,
                    code: code.trim(),
                    name: name.trim(),
                    pricing_model: pricingModel,
                    billing_mode: pricingModel === "FLAT" ? "LICENSED" : "METERED",
                    billing_interval: billingInterval,
                    billing_interval_count: intervalCount,
                    aggregate_usage: pricingModel === "FLAT" ? undefined : "SUM",
                    billing_unit: pricingModel === "FLAT" ? undefined : billingUnit,
                    tax_behavior: taxBehavior,
                    active: isActive,
                  })
                  await loadData()
                  setIsCreateOpen(false)
                } catch (err) {
                  setCreateError(getErrorMessage(err, "Unable to create pricing model."))
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
