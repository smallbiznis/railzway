import { useCallback } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { admin } from "@/api/client"
import { TableSkeleton } from "@/components/loading-skeletons"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useCursorPagination } from "@/hooks/useCursorPagination"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type Subscription = {
  id?: string | number
  ID?: string | number
  customer_id?: string | number
  CustomerID?: string | number
  status?: string
  Status?: string
  collection_mode?: string
  CollectionMode?: string
  start_at?: string
  StartAt?: string
  created_at?: string
  CreatedAt?: string
  updated_at?: string
  UpdatedAt?: string
}

const statusTabs = [
  { value: "ACTIVE", label: "Active" },
  { value: "PAUSED", label: "Paused" },
  { value: "DRAFT", label: "Draft" },
  { value: "CANCELED", label: "Canceled" },
  { value: "ENDED", label: "Ended" },
  { value: "ALL", label: "All" },
]

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

const readField = (
  subscription: Subscription,
  keys: (keyof Subscription)[],
  fallback = "-"
) => {
  for (const key of keys) {
    const value = subscription[key]
    if (value === undefined || value === null) continue
    if (typeof value === "string") {
      if (value.trim().length > 0) return value
      continue
    }
    return String(value)
  }
  return fallback
}

const formatStatus = (value?: string) => {
  if (!value) return "-"
  switch (value) {
    case "ACTIVE":
      return "Active"
    case "PAUSED":
      return "Paused"
    case "CANCELED":
      return "Canceled"
    case "ENDED":
      return "Ended"
    case "DRAFT":
      return "Draft"
    default:
      return value
  }
}

const statusVariant = (value?: string) => {
  switch (value) {
    case "ACTIVE":
      return "secondary"
    case "PAUSED":
      return "outline"
    case "CANCELED":
    case "ENDED":
    case "DRAFT":
      return "outline"
    default:
      return "secondary"
  }
}

const formatCollectionMode = (value?: string) => {
  if (!value) return "-"
  const normalized = value.replace(/_/g, " ").toLowerCase()
  return normalized.replace(/\b\w/g, (letter) => letter.toUpperCase())
}

const PAGE_SIZE = 20

export default function OrgSubscriptionsPage() {
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [searchParams, setSearchParams] = useSearchParams()
  const createPath = orgId ? `/orgs/${orgId}/subscriptions/create` : "/orgs"
  const statusParam = searchParams.get("status") ?? "ALL"
  const statusFilter = statusTabs.some((tab) => tab.value === statusParam)
    ? statusParam
    : "ALL"
  const customerIdFilter = searchParams.get("customer_id") ?? ""
  const createdFrom = searchParams.get("created_from") ?? ""
  const createdTo = searchParams.get("created_to") ?? ""
  const hasFilters = Boolean(
    statusFilter !== "ALL" || customerIdFilter || createdFrom || createdTo
  )

  const handleStatusChange = (value: string) => {
    const next = new URLSearchParams(searchParams)
    if (value === "ALL") {
      next.delete("status")
    } else {
      next.set("status", value)
    }
    setSearchParams(next, { replace: true })
  }

  const handleClearFilters = () => {
    const next = new URLSearchParams(searchParams)
    next.delete("status")
    next.delete("customer_id")
    next.delete("created_from")
    next.delete("created_to")
    setSearchParams(next, { replace: true })
  }

  const fetchSubscriptions = useCallback(
    async (cursor: string | null) => {
      const response = await admin.get("/subscriptions", {
        params: {
          status: statusFilter === "ALL" ? undefined : statusFilter,
          customer_id: customerIdFilter || undefined,
          created_from: createdFrom || undefined,
          created_to: createdTo || undefined,
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
    [createdFrom, createdTo, customerIdFilter, statusFilter]
  )

  const {
    items: subscriptions,
    error,
    isLoading,
    isLoadingMore,
    hasPrev,
    hasNext,
    loadNext,
    loadPrev,
  } = useCursorPagination<Subscription>(fetchSubscriptions, {
    enabled: Boolean(orgId),
    mode: "replace",
    dependencies: [orgId, createdFrom, createdTo, customerIdFilter, statusFilter],
  })

  const isForbidden = error ? isForbiddenError(error) : false
  const errorMessage =
    error && !isForbidden
      ? getErrorMessage(error, "Unable to load subscriptions.")
      : null

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to subscriptions." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Subscriptions</h1>
          <p className="text-text-muted text-sm">
            Monitor recurring revenue, lifecycle states, and customer plans.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {canManage ? (
            <Button size="sm" asChild>
              <Link to={createPath}>Create subscription</Link>
            </Button>
          ) : (
            <Button size="sm" disabled>
              Create subscription
            </Button>
          )}
        </div>
      </div>

      <Tabs value={statusFilter} onValueChange={handleStatusChange}>
        <TabsList className="flex w-full flex-wrap justify-start">
          {statusTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-start">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Filters</CardTitle>
              <CardDescription>Filter by customer and created date.</CardDescription>
            </div>
            <CardAction>
              <Button
                variant="ghost"
                size="sm"
                disabled={!hasFilters}
                onClick={handleClearFilters}
              >
                Clear filters
              </Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-2">
                <Label htmlFor="subscription-filter-customer">Customer ID</Label>
                <Input
                  id="subscription-filter-customer"
                  placeholder="e.g. 1234567890"
                  value={customerIdFilter}
                  onChange={(event) => {
                    const next = new URLSearchParams(searchParams)
                    const value = event.target.value.trim()
                    if (value) {
                      next.set("customer_id", value)
                    } else {
                      next.delete("customer_id")
                    }
                    setSearchParams(next, { replace: true })
                  }}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="subscription-filter-created-from">Created from</Label>
                <Input
                  id="subscription-filter-created-from"
                  type="date"
                  value={createdFrom}
                  onChange={(event) => {
                    const next = new URLSearchParams(searchParams)
                    const value = event.target.value
                    if (value) {
                      next.set("created_from", value)
                    } else {
                      next.delete("created_from")
                    }
                    setSearchParams(next, { replace: true })
                  }}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="subscription-filter-created-to">Created to</Label>
                <Input
                  id="subscription-filter-created-to"
                  type="date"
                  value={createdTo}
                  onChange={(event) => {
                    const next = new URLSearchParams(searchParams)
                    const value = event.target.value
                    if (value) {
                      next.set("created_to", value)
                    } else {
                      next.delete("created_to")
                    }
                    setSearchParams(next, { replace: true })
                  }}
                />
              </div>
            </div>
          </CardContent>
        </Card>
        <div />
      </div>

      {isLoading && subscriptions.length === 0 && (
        <TableSkeleton
          rows={6}
          columnTemplate="grid-cols-[1.6fr_1.2fr_0.8fr_1fr_1fr_1fr_auto]"
          headerWidths={["w-24", "w-20", "w-12", "w-20", "w-16", "w-16", "w-6"]}
          cellWidths={["w-[70%]", "w-[60%]", "w-[50%]", "w-[60%]", "w-[60%]", "w-[60%]", "w-3"]}
        />
      )}
      {errorMessage && <div className="text-status-error text-sm">{errorMessage}</div>}
      {!isLoading && !errorMessage && subscriptions.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No subscriptions yet</EmptyTitle>
            <EmptyDescription>
              Start billing by creating your first customer subscription.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button asChild>
              <Link to={createPath}>Create subscription</Link>
            </Button>
          </EmptyContent>
        </Empty>
      )}
      {subscriptions.length > 0 && (
        <>
          <div className="rounded-lg border">
            <Table className="min-w-[720px]">
              <TableHeader className="[&_th]:sticky [&_th]:top-0 [&_th]:z-10 [&_th]:bg-bg-surface">
              <TableRow>
                <TableHead>Subscription</TableHead>
                <TableHead>Customer</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Collection</TableHead>
                <TableHead>Start date</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {subscriptions.map((subscription, index) => {
                const id = readField(subscription, ["id", "ID"], "")
                const displayID = id || "-"
                const rowKey = id || `subscription-${index}`
                const customerID = readField(subscription, [
                  "customer_id",
                  "CustomerID",
                ])
                const status = readField(subscription, ["status", "Status"], "")
                const normalizedStatus = status ? status.toUpperCase() : ""
                const collectionMode = readField(
                  subscription,
                  ["collection_mode", "CollectionMode"],
                  ""
                )
                const startAt = readField(subscription, ["start_at", "StartAt"], "")
                const updatedAt = readField(
                  subscription,
                  ["updated_at", "UpdatedAt"],
                  ""
                )

                return (
                  <TableRow key={rowKey}>
                    <TableCell className="font-medium">
                      <div className="flex flex-col gap-1">
                        <span className="text-sm">Subscription</span>
                        {id ? (
                          <Link
                            className="text-text-muted text-xs hover:text-accent-primary"
                            to={`/orgs/${orgId}/subscriptions/${id}`}
                          >
                            {displayID}
                          </Link>
                        ) : (
                          <span className="text-text-muted text-xs">{displayID}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="text-sm">{customerID}</span>
                        <span className="text-text-muted text-xs">
                          Customer
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(normalizedStatus)}>
                        {formatStatus(normalizedStatus)}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatCollectionMode(collectionMode)}</TableCell>
                    <TableCell>{formatDate(startAt)}</TableCell>
                    <TableCell>{formatDate(updatedAt)}</TableCell>
                    <TableCell className="text-right">
                      {id ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          asChild
                          aria-label="Open subscription actions"
                        >
                          <Link to={`/orgs/${orgId}/subscriptions/${id}`}>
                            View
                          </Link>
                        </Button>
                      ) : (
                        <Button variant="ghost" size="icon-sm" aria-label="Open subscription actions">
                          ...
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
              {isLoadingMore && (
                <TableRow>
                  <TableCell colSpan={7}>
                    <div className="text-text-muted flex items-center gap-2 text-sm">
                      <Spinner className="size-4" />
                      Loading subscriptions...
                    </div>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="text-text-muted text-sm">
              Showing {subscriptions.length} subscriptions
            </div>
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
