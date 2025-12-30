import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { Plus } from "lucide-react"

import { admin } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"

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

const formatTimestamp = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

const formatAggregation = (value: string) => {
  const trimmed = value.trim()
  if (!trimmed) return "-"
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1).toLowerCase()
}

export default function OrgMeterDetailPage() {
  const { orgId, meterId } = useParams()
  const [meter, setMeter] = useState<Meter | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!orgId || !meterId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)

    admin
      .get(`/meters/${meterId}`)
      .then((response) => {
        if (!isMounted) return
        setMeter(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load meter.")
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId, meterId])

  const statusBadge = useMemo(() => {
    if (!meter) return null
    return (
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
    )
  }, [meter])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading meter...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (!meter) {
    return <div className="text-text-muted text-sm">Meter not found.</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-sm text-text-muted">
            Billing · Usage-based ·{" "}
            <Link className="text-accent-primary hover:underline" to={`/orgs/${orgId}/meter`}>
              Meters
            </Link>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-semibold">{meter.name}</h1>
            {statusBadge}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline">Copy meter to live mode</Button>
          <Button variant="default">Edit meter</Button>
          <Button variant="outline">Create alert</Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Live meter events</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                No events.
              </div>
              <div className="flex justify-end">
                <Button variant="outline">Pause feed</Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Products</CardTitle>
              <div className="flex items-center gap-2">
                <Button variant="outline">Manage</Button>
                <Button variant="outline">
                  <Plus className="size-4" />
                  Create
                </Button>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-text-muted">
              Products with usage-based prices attached to this meter.
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div>
              <div className="text-text-muted">Meter ID</div>
              <div className="font-medium">{meter.id}</div>
            </div>
            <div>
              <div className="text-text-muted">Meter created</div>
              <div className="font-medium">{formatTimestamp(meter.created_at)}</div>
            </div>
            <div>
              <div className="text-text-muted">Meter last updated</div>
              <div className="font-medium">{formatTimestamp(meter.updated_at)}</div>
            </div>
            <Separator />
            <div>
              <div className="text-text-muted">Display name</div>
              <div className="font-medium">{meter.name}</div>
            </div>
            <div>
              <div className="text-text-muted">Event name</div>
              <div className="font-medium">{meter.code}</div>
            </div>
            <div>
              <div className="text-text-muted">Event ingestion</div>
              <div className="font-medium">Raw</div>
            </div>
            <div>
              <div className="text-text-muted">Aggregation method</div>
              <div className="font-medium">{formatAggregation(meter.aggregation)}</div>
            </div>
            <div>
              <div className="text-text-muted">Unit</div>
              <div className="font-medium">{meter.unit}</div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
