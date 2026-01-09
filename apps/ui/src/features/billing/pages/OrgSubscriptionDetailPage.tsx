import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
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
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

type Subscription = {
  id?: string | number
  ID?: string | number
  customer_id?: string | number
  CustomerID?: string | number
  status?: string
  Status?: string
  collection_mode?: string
  CollectionMode?: string
  billing_cycle_type?: string
  BillingCycleType?: string
  start_at?: string
  StartAt?: string
  created_at?: string
  CreatedAt?: string
  updated_at?: string
  UpdatedAt?: string
  activated_at?: string | null
  ActivatedAt?: string | null
  paused_at?: string | null
  PausedAt?: string | null
  resumed_at?: string | null
  ResumedAt?: string | null
  canceled_at?: string | null
  CanceledAt?: string | null
  ended_at?: string | null
  EndedAt?: string | null
}

type Customer = {
  id?: string | number
  ID?: string | number
  name?: string
  Name?: string
}

type ActionType = "activate" | "pause" | "resume" | "cancel"

const statusOrder = ["DRAFT", "ACTIVE", "PAUSED", "CANCELED", "ENDED"]

const readField = <T,>(
  item: T | null | undefined,
  keys: (keyof T)[],
  fallback = "-"
) => {
  if (!item) return fallback
  for (const key of keys) {
    const value = item[key]
    if (value === undefined || value === null) continue
    if (typeof value === "string") {
      const trimmed = value.trim()
      if (trimmed) return trimmed
      continue
    }
    return String(value)
  }
  return fallback
}

const formatDateTime = (value?: string | null) => {
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

const formatStatus = (value?: string) => {
  if (!value) return "-"
  switch (value.toUpperCase()) {
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
  switch (value?.toUpperCase()) {
    case "ACTIVE":
      return "secondary"
    case "PAUSED":
      return "outline"
    case "CANCELED":
    case "ENDED":
      return "outline"
    case "DRAFT":
      return "outline"
    default:
      return "outline"
  }
}

export default function OrgSubscriptionDetailPage() {
  const { orgId, subscriptionId } = useParams()
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [action, setAction] = useState<ActionType | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [isActing, setIsActing] = useState(false)

  const loadSubscription = useCallback(async () => {
    if (!subscriptionId) return
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)
    try {
      const res = await admin.get(`/subscriptions/${subscriptionId}`)
      setSubscription(res.data?.data ?? null)
    } catch (err) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
      } else {
        setError(getErrorMessage(err, "Unable to load subscription."))
      }
    } finally {
      setIsLoading(false)
    }
  }, [subscriptionId])

  useEffect(() => {
    void loadSubscription()
  }, [loadSubscription])

  useEffect(() => {
    const customerId = readField(subscription, ["customer_id", "CustomerID"], "")
    if (!customerId) return
    let active = true
    admin
      .get(`/customers/${customerId}`)
      .then((response) => {
        if (!active) return
        setCustomer(response.data?.data ?? null)
      })
      .catch(() => {
        if (!active) return
        setCustomer(null)
      })
    return () => {
      active = false
    }
  }, [subscription])

  const subscriptionStatus = readField(subscription, ["status", "Status"], "DRAFT")
  const customerId = readField(subscription, ["customer_id", "CustomerID"], "")
  const customerName = readField(customer, ["name", "Name"], customerId || "Customer")
  const collectionMode = readField(subscription, ["collection_mode", "CollectionMode"])
  const billingCycleType = readField(subscription, ["billing_cycle_type", "BillingCycleType"])
  const createdAt = readField(subscription, ["created_at", "CreatedAt"], "")
  const updatedAt = readField(subscription, ["updated_at", "UpdatedAt"], "")
  const startAt = readField(subscription, ["start_at", "StartAt"], "")

  const availableActions = useMemo(() => {
    const normalized = subscriptionStatus.toUpperCase()
    if (normalized === "DRAFT") return ["activate", "cancel"] as ActionType[]
    if (normalized === "ACTIVE") return ["pause", "cancel"] as ActionType[]
    if (normalized === "PAUSED") return ["resume", "cancel"] as ActionType[]
    return [] as ActionType[]
  }, [subscriptionStatus])

  const actionCopy = useMemo(() => {
    if (!action) return null
    switch (action) {
      case "activate":
        return {
          title: "Activate subscription",
          description: "Activating will start billing immediately and cannot be undone.",
        }
      case "pause":
        return {
          title: "Pause subscription",
          description: "Pausing stops billing and usage accrual until resumed.",
        }
      case "resume":
        return {
          title: "Resume subscription",
          description: "Resuming restarts billing immediately and continues the current cycle.",
        }
      case "cancel":
        return {
          title: "Cancel subscription",
          description: "Canceling is irreversible and ends all future billing for this subscription.",
        }
      default:
        return null
    }
  }, [action])

  const timeline = useMemo(() => {
    if (!subscription) return []
    return [
      { label: "Created", value: readField(subscription, ["created_at", "CreatedAt"], "") },
      { label: "Activated", value: readField(subscription, ["activated_at", "ActivatedAt"], "") },
      { label: "Paused", value: readField(subscription, ["paused_at", "PausedAt"], "") },
      { label: "Resumed", value: readField(subscription, ["resumed_at", "ResumedAt"], "") },
      { label: "Canceled", value: readField(subscription, ["canceled_at", "CanceledAt"], "") },
      { label: "Ended", value: readField(subscription, ["ended_at", "EndedAt"], "") },
    ].filter((item) => item.value && item.value !== "-")
  }, [subscription])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading subscription...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this subscription." />
  }

  if (!subscription) {
    return <div className="text-text-muted text-sm">Subscription not found.</div>
  }

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to={`/orgs/${orgId}/subscriptions`}>Subscriptions</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{subscriptionId}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold">Subscription {subscriptionId}</h1>
            <Badge variant={statusVariant(subscriptionStatus)}>
              {formatStatus(subscriptionStatus)}
            </Badge>
          </div>
          <p className="text-text-muted text-sm">
            Customer{" "}
            {customerId ? (
              <Link className="text-accent-primary hover:underline" to={`/orgs/${orgId}/customers/${customerId}`}>
                {customerName}
              </Link>
            ) : (
              customerName
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {availableActions.includes("activate") && (
            <Button onClick={() => setAction("activate")}>Activate</Button>
          )}
          {availableActions.includes("pause") && (
            <Button variant="outline" onClick={() => setAction("pause")}>
              Pause
            </Button>
          )}
          {availableActions.includes("resume") && (
            <Button onClick={() => setAction("resume")}>Resume</Button>
          )}
          {availableActions.includes("cancel") && (
            <Button variant="outline" onClick={() => setAction("cancel")}>
              Cancel
            </Button>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>State machine</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            {statusOrder.map((status) => (
              <div key={status} className="flex items-center gap-2">
                <Badge
                  variant={status === subscriptionStatus.toUpperCase() ? "secondary" : "outline"}
                >
                  {formatStatus(status)}
                </Badge>
                {status !== statusOrder[statusOrder.length - 1] && (
                  <span className="text-text-muted">→</span>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-[1fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Subscription details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div>
              <div className="text-text-muted">Customer</div>
              <div className="font-medium">{customerName}</div>
            </div>
            <div>
              <div className="text-text-muted">Collection mode</div>
              <div className="font-medium">{collectionMode}</div>
            </div>
            <div>
              <div className="text-text-muted">Billing cycle</div>
              <div className="font-medium">{billingCycleType}</div>
            </div>
            <div>
              <div className="text-text-muted">Start date</div>
              <div className="font-medium">{formatDateTime(startAt)}</div>
            </div>
            <div>
              <div className="text-text-muted">Created</div>
              <div className="font-medium">{formatDateTime(createdAt)}</div>
            </div>
            <div>
              <div className="text-text-muted">Last updated</div>
              <div className="font-medium">{formatDateTime(updatedAt)}</div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Lifecycle timeline</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {timeline.length === 0 && (
              <div className="text-text-muted">No lifecycle events yet.</div>
            )}
            {timeline.map((event) => (
              <div key={event.label} className="flex items-center justify-between">
                <span className="text-text-muted">{event.label}</span>
                <span className="font-medium">{formatDateTime(event.value)}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      <AlertDialog
        open={Boolean(action)}
        onOpenChange={(open) => {
          if (!open) {
            setAction(null)
            setActionError(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{actionCopy?.title ?? "Confirm action"}</AlertDialogTitle>
            <AlertDialogDescription>
              {actionCopy?.description ?? "This action changes subscription state."}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {actionError && <div className="text-status-error text-sm">{actionError}</div>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isActing}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={isActing}
              onClick={async () => {
                if (!action || !subscriptionId) return
                setIsActing(true)
                setActionError(null)
                try {
                  await admin.post(`/subscriptions/${subscriptionId}/${action}`)
                  setAction(null)
                  await loadSubscription()
                } catch (err) {
                  setActionError(getErrorMessage(err, "Unable to update subscription."))
                } finally {
                  setIsActing(false)
                }
              }}
            >
              {isActing ? "Updating..." : "Confirm"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
