import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { Plus } from "lucide-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type Meter = {
  id: string
  code: string
  name: string
  aggregation: string
  unit: string
  active: boolean
  created_at: string
  updated_at: string
}

const formatAggregation = (value: string) => {
  const trimmed = value.trim()
  if (!trimmed) return "-"
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1).toLowerCase()
}

const formatTimestamp = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "numeric",
    day: "numeric",
    year: "2-digit",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

export default function OrgMeterPage() {
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [meters, setMeters] = useState<Meter[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  const rows = useMemo(() => meters, [meters])

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
      .get("/meters")
      .then((response) => {
        if (!isMounted) return
        setMeters(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load meters."))
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to meters." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Meters</h1>
          <p className="text-text-muted text-sm">
            Define usage meters for this organization.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {orgId && canManage && (
            <Button size="sm" asChild>
              <Link to={`/orgs/${orgId}/meter/create`}>
                <Plus className="size-4" />
                Create meter
              </Link>
            </Button>
          )}
          {orgId && !canManage && (
            <Button size="sm" disabled>
              <Plus className="size-4" />
              Create meter
            </Button>
          )}
        </div>
      </div>

      <div className="border-b border-border-subtle">
        <nav className="flex items-center gap-6 text-sm font-medium">
          <span className="border-b-2 border-accent-primary pb-3 text-accent-primary">Meters</span>
          <span className="pb-3 text-text-muted">Alerts</span>
          <span className="pb-3 text-text-muted">Credits</span>
        </nav>
      </div>

      <div className="rounded-lg border border-border-subtle bg-bg-surface">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Display name</TableHead>
              <TableHead>Event name</TableHead>
              <TableHead>Aggregation method</TableHead>
              <TableHead>Event ingestion</TableHead>
              <TableHead>Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={5} className="text-text-muted">
                  Loading meters...
                </TableCell>
              </TableRow>
            )}
            {error && (
              <TableRow>
                <TableCell colSpan={5} className="text-status-error">
                  {error}
                </TableCell>
              </TableRow>
            )}
            {!isLoading && !error && rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-text-muted">
                  No meters yet.
                </TableCell>
              </TableRow>
            )}
            {!isLoading &&
              !error &&
              rows.map((meter) => (
                <TableRow key={meter.id}>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <Link
                        className="font-medium text-text-primary hover:underline"
                        to={`/orgs/${orgId}/meter/${meter.id}`}
                      >
                        {meter.name}
                      </Link>
                      <Badge
                        variant={meter.active ? "secondary" : "outline"}
                        className={
                          meter.active
                            ? "border-status-success/30 bg-status-success/10 text-status-success"
                            : "text-text-muted"
                        }
                      >
                        {meter.active ? "Active" : "Deactivated"}
                      </Badge>
                    </div>
                  </TableCell>
                  <TableCell className="text-text-muted">{meter.code}</TableCell>
                  <TableCell className="text-text-muted">
                    {formatAggregation(meter.aggregation)}
                  </TableCell>
                  <TableCell className="text-text-muted">Raw</TableCell>
                  <TableCell className="text-text-muted">
                    {formatTimestamp(meter.created_at)}
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
