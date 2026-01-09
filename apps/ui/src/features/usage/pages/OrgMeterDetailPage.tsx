import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"

import { Plus } from "lucide-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
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
  const navigate = useNavigate()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [meter, setMeter] = useState<Meter | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)
  const [editName, setEditName] = useState("")
  const [editAggregation, setEditAggregation] = useState("")
  const [editUnit, setEditUnit] = useState("")
  const [editActive, setEditActive] = useState(true)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  useEffect(() => {
    if (!orgId || !meterId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/meters/${meterId}`)
      .then((response) => {
        if (!isMounted) return
        setMeter(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load meter."))
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

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this meter." />
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
          <Button
            variant="default"
            onClick={() => {
              setEditName(meter.name)
              setEditAggregation(meter.aggregation)
              setEditUnit(meter.unit)
              setEditActive(meter.active)
              setEditError(null)
              setIsEditOpen(true)
            }}
            disabled={!canManage}
          >
            Edit meter
          </Button>
          <Button
            variant="outline"
            onClick={() => {
              setDeleteError(null)
              setIsDeleteOpen(true)
            }}
            disabled={!canManage}
          >
            Delete meter
          </Button>
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

      <Dialog
        open={isEditOpen}
        onOpenChange={(open) => {
          setIsEditOpen(open)
          if (!open) {
            setEditError(null)
          }
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Edit meter</DialogTitle>
            <DialogDescription>
              Changes apply immediately and affect future usage ingestion.
            </DialogDescription>
          </DialogHeader>
          {editError && <div className="text-status-error text-sm">{editError}</div>}
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="meter-edit-name">Display name</Label>
              <Input
                id="meter-edit-name"
                value={editName}
                onChange={(event) => setEditName(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="meter-edit-aggregation">Aggregation</Label>
              <Select value={editAggregation} onValueChange={setEditAggregation}>
                <SelectTrigger id="meter-edit-aggregation">
                  <SelectValue placeholder="Select aggregation method" />
                </SelectTrigger>
                <SelectContent>
                  {["SUM", "COUNT", "MAX", "MIN", "AVG"].map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="meter-edit-unit">Unit</Label>
              <Input
                id="meter-edit-unit"
                value={editUnit}
                onChange={(event) => setEditUnit(event.target.value)}
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-1">
                <Label htmlFor="meter-edit-active">Active</Label>
                <p className="text-text-muted text-xs">
                  Disable to stop ingesting usage events.
                </p>
              </div>
              <Switch
                id="meter-edit-active"
                checked={editActive}
                onCheckedChange={setEditActive}
              />
            </div>
          </div>
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setIsEditOpen(false)}
              disabled={isSaving}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={isSaving}
              onClick={async () => {
                if (!meterId) return
                setIsSaving(true)
                setEditError(null)
                try {
                  const payload = {
                    name: editName.trim(),
                    aggregation_type: editAggregation.trim(),
                    unit: editUnit.trim(),
                    active: editActive,
                  }
                  const res = await admin.patch(`/meters/${meterId}`, payload)
                  setMeter(res.data?.data ?? meter)
                  setIsEditOpen(false)
                } catch (err) {
                  setEditError(getErrorMessage(err, "Unable to update meter."))
                } finally {
                  setIsSaving(false)
                }
              }}
            >
              {isSaving ? "Saving..." : "Confirm update"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={isDeleteOpen}
        onOpenChange={(open) => {
          setIsDeleteOpen(open)
          if (!open) {
            setDeleteError(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete meter</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently deletes the meter and cannot be undone. Billing events tied to this meter will stop ingesting.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && <div className="text-status-error text-sm">{deleteError}</div>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={isDeleting}
              onClick={async () => {
                if (!meterId || !orgId) return
                setIsDeleting(true)
                setDeleteError(null)
                try {
                  await admin.delete(`/meters/${meterId}`)
                  navigate(`/orgs/${orgId}/meter`, { replace: true })
                } catch (err) {
                  setDeleteError(getErrorMessage(err, "Unable to delete meter."))
                  setIsDeleting(false)
                }
              }}
            >
              {isDeleting ? "Deleting..." : "Delete meter"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
