import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { api } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyContent,
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

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

type PageInfo = {
  next_page_token?: string
  previous_page_token?: string
  has_more?: boolean
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
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [pageInfo, setPageInfo] = useState<PageInfo | null>(null)
  const [pageTokens, setPageTokens] = useState<string[]>([""])
  const [pageIndex, setPageIndex] = useState(0)
  const [statusFilter, setStatusFilter] = useState(statusTabs[0].value)
  const createPath = orgId ? `/orgs/${orgId}/subscriptions/create` : "/orgs"

  const pageToken = pageTokens[pageIndex] ?? ""
  const activeTabLabel = useMemo(() => {
    return statusTabs.find((tab) => tab.value === statusFilter)?.label ?? "All"
  }, [statusFilter])

  const handleStatusChange = (value: string) => {
    setStatusFilter(value)
    setPageTokens([""])
    setPageIndex(0)
  }

  const handleClearFilters = () => {
    setStatusFilter("ALL")
    setPageTokens([""])
    setPageIndex(0)
  }

  useEffect(() => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)

    api
      .get("/subscriptions", {
        params: {
          status: statusFilter === "ALL" ? undefined : statusFilter,
          page_token: pageToken || undefined,
          page_size: PAGE_SIZE,
        },
      })
      .then((response) => {
        if (!isMounted) return
        setSubscriptions(response.data?.data ?? [])
        setPageInfo(response.data?.page_info ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load subscriptions.")
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, pageToken, statusFilter])

  const hasPrevious = pageIndex > 0
  const hasNext = Boolean(pageInfo?.has_more && pageInfo?.next_page_token)

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
          <Button variant="outline" size="sm">
            Export
          </Button>
          <Button variant="outline" size="sm">
            Analyze
          </Button>
          <Button size="sm" asChild>
            <Link to={createPath}>Create subscription</Link>
          </Button>
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

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm">
            Status: {activeTabLabel}
          </Button>
          <Button variant="outline" size="sm">
            Customer ID
          </Button>
          <Button variant="outline" size="sm">
            Created date
          </Button>
          <Button variant="ghost" size="sm" onClick={handleClearFilters}>
            Clear filters
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm">
            Edit columns
          </Button>
        </div>
      </div>

      {isLoading && (
        <div className="text-text-muted text-sm">
          Loading subscriptions...
        </div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && subscriptions.length === 0 && (
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
      {!isLoading && !error && subscriptions.length > 0 && (
        <div className="rounded-lg border">
          <Table className="min-w-[720px]">
            <TableHeader>
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
                        <span className="text-text-muted text-xs">{displayID}</span>
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
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label="Open subscription actions"
                      >
                        ...
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
      {!isLoading && !error && subscriptions.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-text-muted text-xs">
            Showing {subscriptions.length} subscriptions
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={!hasPrevious || isLoading}
              onClick={() => {
                if (!hasPrevious) return
                setPageIndex((prev) => Math.max(prev - 1, 0))
              }}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={!hasNext || isLoading}
              onClick={() => {
                if (!pageInfo?.next_page_token) return
                setPageTokens((prev) => {
                  const next = prev.slice(0, pageIndex + 1)
                  next.push(pageInfo.next_page_token ?? "")
                  return next
                })
                setPageIndex((prev) => prev + 1)
              }}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
