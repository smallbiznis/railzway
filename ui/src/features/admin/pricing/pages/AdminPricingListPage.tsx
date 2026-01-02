import { useMemo } from "react"
import { useNavigate } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"

import { admin } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

import type { Price } from "@/features/admin/pricing/types"
import { formatPricingModel, formatUnit } from "@/features/admin/pricing/utils"

const fetchPrices = async () => {
  const response = await admin.get("/prices")
  const payload = response.data?.data
  if (Array.isArray(payload)) {
    return payload as Price[]
  }
  if (payload && typeof payload === "object") {
    const list = (payload as { prices?: Price[] }).prices
    return Array.isArray(list) ? list : []
  }
  return []
}

export default function AdminPricingListPage() {
  const navigate = useNavigate()
  const { data, isLoading, error } = useQuery({
    queryKey: ["prices"],
    queryFn: fetchPrices,
  })

  const prices = useMemo(() => data ?? [], [data])

  return (
    <div className="space-y-6 px-4 py-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Pricing</h1>
        <p className="text-text-muted text-sm">
          Configure versioned prices for your billing catalog.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Prices</CardTitle>
          <CardDescription>
            Pricing is append-only. Select a price to view its history.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading && (
            <div className="flex items-center gap-2 text-text-muted text-sm">
              <Spinner />
              Loading prices
            </div>
          )}
          {error && (
            <div className="text-status-error text-sm">
              {(error as Error)?.message ?? "Unable to load prices."}
            </div>
          )}
          {!isLoading && !error && prices.length === 0 && (
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">#</EmptyMedia>
                <EmptyTitle>No prices yet</EmptyTitle>
                <EmptyDescription>
                  Create a price in your catalog to begin versioning.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
          {!isLoading && !error && prices.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Code</TableHead>
                  <TableHead>Pricing model</TableHead>
                  <TableHead>Unit</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {prices.map((price) => (
                  <TableRow
                    key={price.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/admin/prices/${price.id}`)}
                  >
                    <TableCell className="font-medium">
                      {price.name ?? "Untitled"}
                    </TableCell>
                    <TableCell className="text-text-muted">
                      {price.code ?? "-"}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {formatPricingModel(price.pricing_model)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-text-muted">
                      {formatUnit(price.billing_unit)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
