import { useCallback, useEffect } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { admin } from "@/api/client"
import { TableSkeleton } from "@/components/loading-skeletons"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useCursorPagination } from "@/hooks/useCursorPagination"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type Product = {
  id: number | string
  name?: string
  code?: string
  active?: boolean
  created_at?: string
  updated_at?: string
  metadata?: Record<string, unknown> | null
}

const formatDate = (value?: string) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  }).format(date)
}

const readMetadataValue = (
  metadata: Product["metadata"],
  key: string
) => {
  if (!metadata || typeof metadata !== "object") return "-"
  const value = (metadata as Record<string, unknown>)[key]
  if (typeof value === "string" && value.trim().length > 0) return value
  return "-"
}

const PAGE_SIZE = 25

export default function OrgProductsPage() {
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [searchParams, setSearchParams] = useSearchParams()
  const nameFilter = searchParams.get("name") ?? ""
  const activeParam = searchParams.get("active")
  const sortBy = searchParams.get("sort_by") ?? "created_at"
  const orderBy = searchParams.get("order_by") ?? "asc"
  const activeFilter =
    activeParam === "true" ? true : activeParam === "false" ? false : undefined

  useEffect(() => {
    const next = new URLSearchParams(searchParams)
    let changed = false
    if (!searchParams.get("sort_by")) {
      next.set("sort_by", "created_at")
      changed = true
    }
    if (!searchParams.get("order_by")) {
      next.set("order_by", "asc")
      changed = true
    }
    if (changed) {
      setSearchParams(next, { replace: true })
    }
  }, [searchParams, setSearchParams])

  const fetchProducts = useCallback(
    async (cursor: string | null) => {
      const response = await admin.get("/products", {
        params: {
          name: nameFilter || undefined,
          active: activeFilter,
          sort_by: sortBy,
          order_by: orderBy,
          cursor: cursor || undefined,
          page_token: cursor || undefined,
          page_size: PAGE_SIZE,
        },
      })

      const payload = response.data?.data
      const items = Array.isArray(payload?.items)
        ? payload.items
        : Array.isArray(payload)
          ? payload
          : []
      const pageInfo = payload?.page_info ?? response.data?.page_info ?? null

      return { items, page_info: pageInfo }
    },
    [activeFilter, nameFilter, orderBy, sortBy]
  )

  const {
    items: products,
    error,
    isLoading,
    isLoadingMore,
    hasPrev,
    hasNext,
    loadNext,
    loadPrev,
  } = useCursorPagination<Product>(fetchProducts, {
    enabled: Boolean(orgId),
    mode: "replace",
    dependencies: [orgId, nameFilter, activeFilter, sortBy, orderBy],
  })

  const totalCount = products.length
  const activeCount = products.filter((product) => product.active).length
  const archivedCount = totalCount - activeCount
  const isForbidden = error ? isForbiddenError(error) : false
  const errorMessage =
    error && !isForbidden
      ? getErrorMessage(error, "Unable to load products.")
      : null

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to products." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Products</h1>
          <p className="text-text-muted text-sm">
            Create digital plans and usage-based services. Configure pricing and meters inside each product.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {orgId && canManage && (
            <Button asChild size="sm">
              <Link data-testid="products-create" to={`/orgs/${orgId}/products/create`}>
                Create product
              </Link>
            </Button>
          )}
          {orgId && !canManage && (
            <Button size="sm" disabled>
              Create product
            </Button>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Input
          className="w-full max-w-md"
          placeholder="Filter by product name"
          aria-label="Filter products by name"
          value={nameFilter}
          onChange={(event) => {
            const next = new URLSearchParams(searchParams)
            const value = event.target.value.trim()
            if (value) {
              next.set("name", value)
            } else {
              next.delete("name")
            }
            setSearchParams(next, { replace: true })
          }}
        />
      </div>

      <div className="grid gap-3 md:grid-cols-3">
        <Card>
          <CardContent className="flex flex-col gap-1">
            <span className="text-text-muted text-sm">This page</span>
            <span className="text-2xl font-semibold">{totalCount}</span>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex flex-col gap-1">
            <span className="text-text-muted text-sm">Active on page</span>
            <span className="text-2xl font-semibold">{activeCount}</span>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex flex-col gap-1">
            <span className="text-text-muted text-sm">Archived on page</span>
            <span className="text-2xl font-semibold">{archivedCount}</span>
          </CardContent>
        </Card>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              const next = new URLSearchParams(searchParams)
              const nextOrder = orderBy === "asc" ? "desc" : "asc"
              next.set("sort_by", "created_at")
              next.set("order_by", nextOrder)
              setSearchParams(next, { replace: true })
            }}
          >
            Sort: Created ({orderBy === "asc" ? "Oldest" : "Newest"})
          </Button>
          <Select
            value={
              activeFilter === undefined
                ? "all"
                : activeFilter
                  ? "active"
                  : "archived"
            }
            onValueChange={(value) => {
              const next = new URLSearchParams(searchParams)
              if (value === "all") {
                next.delete("active")
              } else {
                next.set("active", value === "active" ? "true" : "false")
              }
              setSearchParams(next, { replace: true })
            }}
          >
            <SelectTrigger className="h-8 w-[160px]">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Status: All</SelectItem>
              <SelectItem value="active">Status: Active</SelectItem>
              <SelectItem value="archived">Status: Archived</SelectItem>
            </SelectContent>
          </Select>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              const next = new URLSearchParams(searchParams)
              next.delete("name")
              next.delete("active")
              setSearchParams(next, { replace: true })
            }}
          >
            Clear filters
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-2">
        </div>
      </div>

      {isLoading && products.length === 0 && (
        <TableSkeleton
          rows={7}
          columnTemplate="grid-cols-[1.6fr_1.1fr_1fr_1fr_1fr_auto]"
          headerWidths={["w-24", "w-20", "w-20", "w-16", "w-16", "w-6"]}
          cellWidths={["w-[70%]", "w-[60%]", "w-[60%]", "w-[60%]", "w-[60%]", "w-3"]}
        />
      )}
      {errorMessage && <div className="text-status-error text-sm">{errorMessage}</div>}
      {!isLoading && !errorMessage && products.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No products yet</EmptyTitle>
            <EmptyDescription>
              Create a product to start billing for usage and subscriptions.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            {orgId && (
              <Button asChild>
                <Link to={`/orgs/${orgId}/products/create`}>Create product</Link>
              </Button>
            )}
          </EmptyContent>
        </Empty>
      )}
      {products.length > 0 && (
        <>
          <div className="rounded-lg border">
            <Table className="min-w-[720px]">
              <TableHeader className="[&_th]:sticky [&_th]:top-0 [&_th]:z-10 [&_th]:bg-bg-surface">
              <TableRow>
                <TableHead>Product</TableHead>
                <TableHead>Pricing model</TableHead>
                <TableHead>Usage meter</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
              </TableHeader>
              <TableBody>
              {products.map((product) => {
                const pricingModel = readMetadataValue(
                  product.metadata,
                  "pricing_summary"
                )
                const usageMeter = readMetadataValue(
                  product.metadata,
                  "usage_meter"
                )
                return (
                  <TableRow key={product.id}>
                    <TableCell className="font-medium">
                      <div className="flex flex-col gap-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link
                            to={`/orgs/${orgId}/products/${product.id}`}
                            className="hover:text-accent-primary"
                          >
                            {product.name ?? "Untitled product"}
                          </Link>
                          <Badge variant={product.active ? "secondary" : "outline"}>
                            {product.active ? "Active" : "Archived"}
                          </Badge>
                        </div>
                        <span className="text-text-muted text-xs">
                          {product.code ?? "-"}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>{pricingModel}</TableCell>
                    <TableCell>{usageMeter}</TableCell>
                    <TableCell>{formatDate(product.created_at)}</TableCell>
                    <TableCell>{formatDate(product.updated_at)}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label="Open product actions"
                      >
                        ...
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
              {isLoadingMore && (
                <TableRow>
                  <TableCell colSpan={6}>
                    <div className="text-text-muted flex items-center gap-2 text-sm">
                      <Spinner className="size-4" />
                      Loading products...
                    </div>
                  </TableCell>
                </TableRow>
              )}
              </TableBody>
            </Table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
            <span className="text-text-muted">
              Showing {products.length} products
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!hasPrev || isLoadingMore}
                onClick={() => void loadPrev()}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasNext || isLoadingMore}
                onClick={() => void loadNext()}
              >
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
