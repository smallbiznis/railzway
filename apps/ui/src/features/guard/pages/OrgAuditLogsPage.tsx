import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import {
  IconExternalLink,
  IconSearch,
} from "@tabler/icons-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
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

type AuditLog = Record<string, unknown>

const formatTimestamp = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

const formatDateInput = (value: string) => {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return date.toISOString()
}

const readField = <T extends Record<string, unknown>>(
  item: T | undefined,
  fields: Array<keyof T | string>
) => {
  if (!item) return undefined
  for (const field of fields) {
    if (field in item) {
      return item[field as keyof T]
    }
  }
  return undefined
}

const getMetadata = (log?: AuditLog) => {
  const raw = readField(log, ["metadata", "Metadata"])
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    return raw as Record<string, unknown>
  }
  return {}
}

const humanizeAction = (action: string) => {
  if (!action) return "-"
  return action
    .split(".")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

const buildSummary = (log: AuditLog) => {
  const metadata = getMetadata(log)
  const error = metadata.error
  if (typeof error === "string" && error.trim()) {
    return `Error: ${error}`
  }
  const reason = metadata.reason
  if (typeof reason === "string" && reason.trim()) {
    return `Reason: ${reason}`
  }
  const status = metadata.status
  if (typeof status === "string" && status.trim()) {
    return `Status: ${status}`
  }
  const invoiceNumber = metadata.invoice_number
  if (typeof invoiceNumber === "number" || typeof invoiceNumber === "string") {
    return `Invoice #${invoiceNumber}`
  }
  const periodStart = metadata.period_start
  const periodEnd = metadata.period_end
  if (typeof periodStart === "string" && typeof periodEnd === "string") {
    return `${formatTimestamp(periodStart)} → ${formatTimestamp(periodEnd)}`
  }
  return "—"
}

const actorLabel = (log: AuditLog) => {
  const actorType = String(readField(log, ["actor_type", "ActorType"]) ?? "").toUpperCase()
  const actorID = String(readField(log, ["actor_id", "ActorID"]) ?? "")
  if (!actorType) return "-"
  if (actorID) {
    return `${actorType} · ${actorID}`
  }
  return actorType
}

const resourceTypeLabel = (log: AuditLog) => {
  const targetType = String(readField(log, ["target_type", "TargetType"]) ?? "")
  if (!targetType) return "-"
  return targetType
}

const targetIDLabel = (log: AuditLog) => {
  const targetID = String(readField(log, ["target_id", "TargetID"]) ?? "")
  if (!targetID) return "-"
  return targetID
}

const buildTargetLink = (orgId: string | undefined, log: AuditLog) => {
  if (!orgId) return null
  const targetType = String(readField(log, ["target_type", "TargetType"]) ?? "")
  const targetID = String(readField(log, ["target_id", "TargetID"]) ?? "")
  if (!targetType || !targetID) return null
  switch (targetType) {
    case "invoice":
      return `/orgs/${orgId}/invoices/${targetID}`
    case "subscription":
      return `/orgs/${orgId}/subscriptions`
    case "api_key":
      return `/orgs/${orgId}/api-keys`
    default:
      return null
  }
}

const actorTypeOptions = [
  { value: "user", label: "User" },
  { value: "system", label: "System" },
  { value: "api_key", label: "API Key" },
]

const ALL_FILTER_VALUE = "__all__"

const resourceTypeOptions = [
  { value: "product", label: "Product" },
  { value: "price", label: "Price" },
  { value: "price_amount", label: "Price amount" },
  { value: "customer", label: "Customer" },
  { value: "subscription", label: "Subscription" },
  { value: "billing_cycle", label: "Billing cycle" },
  { value: "invoice", label: "Invoice" },
  { value: "payment_provider_config", label: "Payment provider config" },
  { value: "api_key", label: "API key" },
  { value: "user", label: "User" },
]

export default function OrgAuditLogsPage() {
  const { orgId } = useParams()
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [pageToken, setPageToken] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)

  const [filters, setFilters] = useState({
    action: "",
    resourceType: "",
    resourceID: "",
    actorType: "",
    startAt: "",
    endAt: "",
  })
  const [appliedFilters, setAppliedFilters] = useState(filters)

  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)

  const loadLogs = useCallback(
    async (nextToken?: string | null, append = false) => {
      if (!orgId) {
        setIsLoading(false)
        return
      }

      setIsLoading(true)
      setError(null)
      setIsForbidden(false)

      const params: Record<string, string | number> = {
        page_size: 50,
      }
      if (appliedFilters.action.trim()) {
        params.action = appliedFilters.action.trim()
      }
      if (appliedFilters.resourceType) {
        params.resource_type = appliedFilters.resourceType
      }
      if (appliedFilters.resourceID.trim()) {
        params.resource_id = appliedFilters.resourceID.trim()
      }
      if (appliedFilters.actorType) {
        params.actor_type = appliedFilters.actorType
      }
      if (appliedFilters.startAt) {
        const startAt = formatDateInput(appliedFilters.startAt)
        if (startAt) {
          params.from = startAt
        }
      }
      if (appliedFilters.endAt) {
        const endAt = formatDateInput(appliedFilters.endAt)
        if (endAt) {
          params.to = endAt
        }
      }
      if (nextToken) {
        params.page_token = nextToken
      }

      try {
        const res = await admin.get("/audit-logs", { params })
        const payload = res.data ?? {}
        const data = Array.isArray(payload.data) ? payload.data : []
        const info = payload.page_info ?? {}
        const next = readField(info, [
          "next_page_token",
          "NextPageToken",
          "nextPageToken",
        ])
        const more = readField(info, ["has_more", "HasMore", "hasMore"])
        setPageToken(typeof next === "string" ? next : null)
        setHasMore(Boolean(more))
        setLogs((prev) => (append ? [...prev, ...data] : data))
      } catch (err: any) {
        if (isForbiddenError(err)) {
          setIsForbidden(true)
        } else {
          setError(getErrorMessage(err, "Unable to load audit logs."))
        }
        if (!append) {
          setLogs([])
        }
      } finally {
        setIsLoading(false)
      }
    },
    [appliedFilters, orgId]
  )

  useEffect(() => {
    void loadLogs(null, false)
  }, [loadLogs])

  const handleApply = () => {
    setAppliedFilters(filters)
  }

  const handleClear = () => {
    const cleared = {
      action: "",
      resourceType: "",
      resourceID: "",
      actorType: "",
      startAt: "",
      endAt: "",
    }
    setFilters(cleared)
    setAppliedFilters(cleared)
  }

  const detailMetadata = useMemo(() => getMetadata(selectedLog ?? undefined), [selectedLog])

  const detailRequestID = useMemo(() => {
    const value = detailMetadata.request_id
    return typeof value === "string" ? value : ""
  }, [detailMetadata])

  const detailSubscriptionID = useMemo(() => {
    const value = detailMetadata.subscription_id
    return typeof value === "string" ? value : ""
  }, [detailMetadata])

  const detailBillingCycleID = useMemo(() => {
    const value = detailMetadata.billing_cycle_id
    return typeof value === "string" ? value : ""
  }, [detailMetadata])

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to audit logs." />
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Audit log</h1>
        <p className="text-text-muted text-sm">
          Trace billing, security, and admin activity across this organization.
        </p>
      </div>

      <div className="rounded-lg border border-border-subtle bg-bg-primary p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <Input
              className="w-full min-w-50 max-w-xs"
              placeholder="Action (e.g. invoice.finalize)"
              value={filters.action}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, action: event.target.value }))
              }
            />
            <Input
              className="w-full min-w-50 max-w-xs"
              placeholder="Resource ID"
              value={filters.resourceID}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, resourceID: event.target.value }))
              }
            />
            <Select
              value={filters.resourceType || ALL_FILTER_VALUE}
              onValueChange={(value) =>
                setFilters((prev) => ({
                  ...prev,
                  resourceType: value === ALL_FILTER_VALUE ? "" : value,
                }))
              }
            >
              <SelectTrigger className="w-45">
                <SelectValue placeholder="Resource type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_FILTER_VALUE}>All resources</SelectItem>
                {resourceTypeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={filters.actorType || ALL_FILTER_VALUE}
              onValueChange={(value) =>
                setFilters((prev) => ({
                  ...prev,
                  actorType: value === ALL_FILTER_VALUE ? "" : value,
                }))
              }
            >
              <SelectTrigger className="w-40">
                <SelectValue placeholder="Actor type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_FILTER_VALUE}>All actors</SelectItem>
                {actorTypeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              type="datetime-local"
              className="w-50"
              value={filters.startAt}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, startAt: event.target.value }))
              }
            />
            <Input
              type="datetime-local"
              className="w-50"
              value={filters.endAt}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, endAt: event.target.value }))
              }
            />
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleClear}>
              Clear
            </Button>
            <Button size="sm" onClick={handleApply}>
              <IconSearch />
              Apply
            </Button>
          </div>
        </div>
      </div>

      {isLoading && (
        <div className="text-text-muted text-sm">Loading audit logs...</div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}

      {!isLoading && !error && logs.length === 0 && (
        <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
          No audit log entries match the selected filters.
        </div>
      )}

      {!error && logs.length > 0 && (
        <div className="rounded-lg border border-border-subtle bg-bg-primary">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Resource</TableHead>
                <TableHead>Target ID</TableHead>
                <TableHead>Summary</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log) => {
                const createdAt = String(readField(log, ["created_at", "CreatedAt"]) ?? "")
                const action = String(readField(log, ["action", "Action"]) ?? "")
                const targetHref = buildTargetLink(orgId, log)
                return (
                  <TableRow
                    key={String(readField(log, ["id", "ID"]) ?? createdAt)}
                    className="cursor-pointer"
                    onClick={() => setSelectedLog(log)}
                  >
                    <TableCell className="text-text-muted text-xs">
                      {formatTimestamp(createdAt)}
                    </TableCell>
                    <TableCell className="text-text-muted text-xs">{actorLabel(log)}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">{humanizeAction(action)}</Badge>
                        <span className="text-text-muted text-xs">{action || "-"}</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-text-muted text-xs">{resourceTypeLabel(log)}</TableCell>
                    <TableCell className="text-text-muted text-xs">
                      {targetHref ? (
                        <Link
                          to={targetHref}
                          className="inline-flex items-center gap-1 text-accent-primary hover:underline"
                          onClick={(event) => event.stopPropagation()}
                        >
                          {targetIDLabel(log)}
                          <IconExternalLink className="h-3 w-3" />
                        </Link>
                      ) : (
                        targetIDLabel(log)
                      )}
                    </TableCell>
                    <TableCell className="text-text-muted text-xs">{buildSummary(log)}</TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      {hasMore && (
        <div className="flex items-center justify-center">
          <Button
            variant="outline"
            size="sm"
            disabled={isLoading}
            onClick={() => loadLogs(pageToken, true)}
          >
            Load more
          </Button>
        </div>
      )}

      <Dialog open={!!selectedLog} onOpenChange={(open) => !open && setSelectedLog(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Audit log detail</DialogTitle>
            <DialogDescription>
              Review metadata, correlation IDs, and request context.
            </DialogDescription>
          </DialogHeader>

          {selectedLog && (
            <div className="space-y-4 text-sm">
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <div className="text-text-muted">Timestamp</div>
                  <div>{formatTimestamp(String(readField(selectedLog, ["created_at", "CreatedAt"]) ?? ""))}</div>
                </div>
                <div>
                  <div className="text-text-muted">Action</div>
                  <div>{String(readField(selectedLog, ["action", "Action"]) ?? "-")}</div>
                </div>
                <div>
                  <div className="text-text-muted">Actor</div>
                  <div>{actorLabel(selectedLog)}</div>
                </div>
                <div>
                  <div className="text-text-muted">Resource</div>
                  <div>{resourceTypeLabel(selectedLog)}</div>
                </div>
                <div>
                  <div className="text-text-muted">Target ID</div>
                  <div>{targetIDLabel(selectedLog)}</div>
                </div>
                <div>
                  <div className="text-text-muted">Request ID</div>
                  <div>{detailRequestID || "-"}</div>
                </div>
                <div>
                  <div className="text-text-muted">Subscription ID</div>
                  <div>{detailSubscriptionID || "-"}</div>
                </div>
                <div>
                  <div className="text-text-muted">Billing cycle ID</div>
                  <div>{detailBillingCycleID || "-"}</div>
                </div>
                <div>
                  <div className="text-text-muted">IP address</div>
                  <div>{String(readField(selectedLog, ["ip_address", "IPAddress"]) ?? "-")}</div>
                </div>
                <div className="sm:col-span-2">
                  <div className="text-text-muted">User agent</div>
                  <div className="break-words">
                    {String(readField(selectedLog, ["user_agent", "UserAgent"]) ?? "-")}
                  </div>
                </div>
              </div>

              <div>
                <div className="text-text-muted mb-2">Metadata</div>
                <pre className="max-h-64 overflow-auto rounded-md border border-border-subtle bg-bg-subtle/40 p-3 text-xs text-text-muted">
                  {JSON.stringify(detailMetadata, null, 2)}
                </pre>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
