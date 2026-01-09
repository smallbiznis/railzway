import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
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

type Price = {
  id: string
  product_id: string
  name?: string
  code?: string
  pricing_model: string
  billing_interval: string
  billing_interval_count: number
  billing_mode?: string
  active?: boolean
  version?: number
  created_at?: string
}

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

export default function OrgPricesPage() {
  const { orgId } = useParams()
  const [prices, setPrices] = useState<Price[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  useEffect(() => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get("/prices")
      .then((response) => {
        if (!isMounted) return
        setPrices(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load prices."))
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  const hasPrices = prices.length > 0
  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"
  const rows = useMemo(() => prices, [prices])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading prices...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to prices." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Prices</h1>
          <p className="text-text-muted text-sm">
            Versioned pricing entries connected to products.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild size="sm">
            <Link to={`${orgBasePath}/products`}>Create price</Link>
          </Button>
        </div>
      </div>

      <PricingNav />

      {!hasPrices && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No prices yet</EmptyTitle>
            <EmptyDescription>
              Create a price from a product to begin versioning.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}

      {hasPrices && (
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
              {rows.map((price) => (
                <TableRow key={price.id}>
                  <TableCell className="font-medium">
                    {price.name || "Untitled"}
                  </TableCell>
                  <TableCell className="text-text-muted">{price.code || "-"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{formatModel(price.pricing_model)}</Badge>
                  </TableCell>
                  <TableCell>{formatInterval(price.billing_interval, price.billing_interval_count)}</TableCell>
                  <TableCell>
                    <Badge variant={price.active ? "secondary" : "outline"}>
                      {price.active ? "Active" : "Inactive"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="ghost" size="sm">
                      <Link to={`${orgBasePath}/prices/${price.id}`}>View</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}
