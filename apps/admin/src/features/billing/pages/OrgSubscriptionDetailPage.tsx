import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import {
  Activity,
  Calendar,
  CheckCircle,
  Clock,
  CreditCard,
  Pause,
  Play,
  User,
  XCircle,
} from "lucide-react"

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
import { Separator } from "@/components/ui/separator"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { cn } from "@/lib/utils"

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
      return "secondary" // Green-ish usually in shadcn themes if configured, or secondary default
    case "PAUSED":
      return "outline"
    case "CANCELED":
    case "ENDED":
      return "destructive"
    case "DRAFT":
      return "secondary"
    default:
      return "outline"
  }
}

// Helper Components
function StatusStepper({ currentStatus }: { currentStatus: string }) {
  const currentUpper = currentStatus.toUpperCase()
  const currentIndex = statusOrder.indexOf(currentUpper)

  return (
    <div className="flex w-full overflow-x-auto pb-2">
      <div className="flex min-w-max items-center">
        {statusOrder.map((status, index) => {
          const isCompleted = index < currentIndex
          const isCurrent = index === currentIndex
          // const isUpcoming = index > currentIndex

          return (
            <div key={status} className="flex items-center">
              <div className="flex flex-col items-center gap-2">
                <div
                  className={cn(
                    "flex h-8 w-8 items-center justify-center rounded-full border-2 text-xs font-bold transition-colors",
                    isCompleted
                      ? "border-primary bg-primary text-primary-foreground"
                      : isCurrent
                        ? "border-primary text-primary"
                        : "border-muted text-muted-foreground"
                  )}
                >
                  {isCompleted ? <CheckCircle className="h-4 w-4" /> : index + 1}
                </div>
                <span
                  className={cn(
                    "text-xs font-medium uppercase",
                    isCurrent ? "text-foreground" : "text-muted-foreground"
                  )}
                >
                  {formatStatus(status)}
                </span>
              </div>
              {index < statusOrder.length - 1 && (
                <div
                  className={cn(
                    "mx-4 h-[2px] w-12 sm:w-20",
                    index < currentIndex ? "bg-primary" : "bg-muted"
                  )}
                />
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function DetailItem({
  icon: Icon,
  label,
  value,
  children,
}: {
  icon: React.ElementType
  label: string
  value?: string | React.ReactNode
  children?: React.ReactNode
}) {
  return (
    <div className="flex items-start gap-3 rounded-md border p-3 shadow-sm">
      <div className="rounded-md bg-muted p-2">
        <Icon className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="flex flex-col gap-1">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        {children ? (
          children
        ) : (
          <span className="text-sm font-semibold text-foreground">{value || "-"}</span>
        )}
      </div>
    </div>
  )
}

function VerticalTimeline({ items }: { items: { label: string; value: string }[] }) {
  if (items.length === 0) {
    return <div className="text-sm text-text-muted">No lifecycle events yet.</div>
  }

  // Sort items by date descending (newest first)
  // Assuming the `value` is a date string that can be parsed
  const sortedItems = [...items].sort((a, b) => {
    return new Date(b.value).getTime() - new Date(a.value).getTime()
  })

  return (
    <div className="relative space-y-0 pl-4 before:absolute before:left-[5px] before:top-2 before:h-[calc(100%-16px)] before:w-[2px] before:bg-muted">
      {sortedItems.map((item) => (
        <div key={item.label} className="relative pb-6 last:pb-0">
          <div className="absolute left-[-15px] top-1 h-3 w-3 rounded-full border-2 border-background bg-primary ring-2 ring-background" />
          <div className="flex flex-col gap-1 pl-2">
            <span className="text-xs font-medium text-muted-foreground">{item.label}</span>
            <span className="text-sm font-semibold">{formatDateTime(item.value)}</span>
          </div>
        </div>
      ))}
    </div>
  )
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
    <div className="mx-auto max-w-6xl space-y-8">
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

      <div className="space-y-6">
        {/* Header Section */}
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div className="space-y-1">
            <h1 className="flex items-center gap-3 text-3xl font-bold tracking-tight">
              Subscription
              <Badge variant={statusVariant(subscriptionStatus)} className="ml-2 text-sm">
                {formatStatus(subscriptionStatus)}
              </Badge>
            </h1>
            <p className="text-muted-foreground">
              Manage subscription details and lifecycle for {customerName}.
            </p>
          </div>
          <div className="flex gap-2">
            {availableActions.includes("activate") && (
              <Button onClick={() => setAction("activate")} className="gap-2">
                <Play className="h-4 w-4" /> Activate
              </Button>
            )}
            {availableActions.includes("pause") && (
              <Button variant="outline" onClick={() => setAction("pause")} className="gap-2">
                <Pause className="h-4 w-4" /> Pause
              </Button>
            )}
            {availableActions.includes("resume") && (
              <Button onClick={() => setAction("resume")} className="gap-2">
                <Play className="h-4 w-4" /> Resume
              </Button>
            )}
            {availableActions.includes("cancel") && (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setAction("cancel")}
                className="gap-2"
              >
                <XCircle className="h-4 w-4" /> Cancel
              </Button>
            )}
          </div>
        </div>

        <Separator />

        {/* Status Stepper */}
        <Card className="bg-muted/30">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base font-medium text-muted-foreground">
              <Activity className="h-4 w-4" /> Subscription Lifecycle
            </CardTitle>
          </CardHeader>
          <CardContent>
            <StatusStepper currentStatus={subscriptionStatus} />
          </CardContent>
        </Card>

        <div className="grid gap-6 lg:grid-cols-3">
          {/* Main Details - Spans 2 columns */}
          <div className="space-y-6 lg:col-span-2">
            <Card>
              <CardHeader>
                <CardTitle>Subscription Details</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-4 sm:grid-cols-2">
                <DetailItem icon={User} label="Customer">
                  {customerId ? (
                    <Link
                      className="text-sm font-semibold text-primary hover:underline"
                      to={`/orgs/${orgId}/customers/${customerId}`}
                    >
                      {customerName}
                    </Link>
                  ) : (
                    <span className="text-sm font-semibold">{customerName}</span>
                  )}
                </DetailItem>
                <DetailItem
                  icon={CreditCard}
                  label="Collection Mode"
                  value={collectionMode}
                />
                <DetailItem
                  icon={Clock}
                  label="Billing Cycle"
                  value={billingCycleType}
                />
                <DetailItem
                  icon={Calendar}
                  label="Start Date"
                  value={formatDateTime(startAt)}
                />
                <DetailItem
                  icon={Calendar}
                  label="Created At"
                  value={formatDateTime(createdAt)}
                />
                <DetailItem
                  icon={Calendar}
                  label="Last Updated"
                  value={formatDateTime(updatedAt)}
                />
              </CardContent>
            </Card>
          </div>

          {/* Sidebar / Timeline - Spans 1 column */}
          <div className="space-y-6">
            <Card className="h-full">
              <CardHeader>
                <CardTitle>Lifecycle Timeline</CardTitle>
              </CardHeader>
              <CardContent>
                <VerticalTimeline items={timeline} />
              </CardContent>
            </Card>
          </div>
        </div>
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
