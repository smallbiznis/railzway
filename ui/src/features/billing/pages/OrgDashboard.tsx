import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, Navigate } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useOrgStore } from "@/stores/orgStore"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { CircleCheck, CreditCard, Receipt } from "lucide-react"

type CustomerBalance = {
  customer_id: string
  name: string
  balance: number
  currency: string
  last_invoice_id?: string
  payment_status: string
}

type BillingCycleSummary = {
  cycle_id: string
  period: string
  total_revenue: number
  invoice_count: number
  status: string
}

type BillingActivity = {
  action: string
  message: string
  occurred_at: string
}

type BillingGroup = {
  title: string
  activities: BillingActivity[]
}

type InvoiceRecord = Record<string, unknown>

type DashboardResponse<T> = T[] | null | undefined

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

const formatCurrency = (amount: number | null | undefined, currency?: string) => {
  if (amount === null || amount === undefined) return "-"
  const safeCurrency = currency?.toUpperCase() || "USD"
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: safeCurrency,
    }).format(amount / 100)
  } catch {
    return `${(amount / 100).toFixed(2)} ${safeCurrency}`
  }
}

const formatBalance = (balance: number, currency: string) => {
  if (balance === 0) {
    return { label: formatCurrency(0, currency), tone: "text-text-muted" }
  }
  if (balance > 0) {
    return {
      label: `${formatCurrency(balance, currency)} due`,
      tone: "text-status-error",
    }
  }
  return {
    label: `${formatCurrency(Math.abs(balance), currency)} credit`,
    tone: "text-status-success",
  }
}

const formatDate = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  }).format(date)
}

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

const statusLabel = (status?: string) => {
  switch (status?.toLowerCase()) {
    case "open":
      return "Open"
    case "closing":
      return "Closing"
    case "closed":
      return "Closed"
    default:
      return status || "-"
  }
}

const statusVariant = (status?: string) => {
  switch (status?.toLowerCase()) {
    case "open":
      return "secondary"
    case "closing":
      return "outline"
    case "closed":
      return "default"
    default:
      return "secondary"
  }
}

type BadgeVariant = "default" | "destructive" | "outline" | "secondary"
type BadgeStyle = { variant: BadgeVariant; className: string }

const paymentBadgeStyle = (status?: string): BadgeStyle => {
  switch (status) {
    case "due":
      return { variant: "destructive", className: "" }
    case "credit":
      return { variant: "outline", className: "border-status-success/30 text-status-success" }
    default:
      return { variant: "outline", className: "text-text-muted" }
  }
}

const paymentLabel = (status?: string) => {
  switch (status) {
    case "due":
      return "Due"
    case "credit":
      return "Credit"
    case "settled":
      return "Settled"
    default:
      return status || "-"
  }
}

const invoiceStatusLabel = (status?: string) => {
  switch (status?.toUpperCase()) {
    case "DRAFT":
      return "Draft"
    case "FINALIZED":
      return "Finalized"
    case "VOID":
      return "Void"
    default:
      return status || "-"
  }
}

const invoiceStatusVariant = (status?: string) => {
  switch (status?.toUpperCase()) {
    case "FINALIZED":
      return "secondary"
    case "DRAFT":
    case "VOID":
      return "outline"
    default:
      return "secondary"
  }
}

const invoiceLabel = (invoice: InvoiceRecord) => {
  const number = readField(invoice, ["invoice_number", "InvoiceNumber"])
  if (typeof number === "number" && number > 0) {
    return `${number}`
  }
  if (typeof number === "string" && number.trim()) {
    return `${number.trim()}`
  }
  const id = String(readField(invoice, ["id", "ID"]) ?? "")
  if (!id) return "-"
  return `${id.slice(0, 8)}`
}

export default function OrgDashboard() {
  const org = useOrgStore((s) => s.currentOrg)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [customerBalances, setCustomerBalances] = useState<CustomerBalance[]>([])
  const [billingCycles, setBillingCycles] = useState<BillingCycleSummary[]>([])
  const [invoices, setInvoices] = useState<InvoiceRecord[]>([])
  const [activity, setActivity] = useState<BillingGroup[]>([])

  const loadDashboard = useCallback(async () => {
    if (!org) {
      setIsLoading(false)
      return
    }
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)
    try {
      const [customersRes, cyclesRes, invoicesRes, activityRes] = await Promise.all([
        admin.get("/billing/customers"),
        admin.get("/billing/cycles"),
        admin.get("/invoices"),
        admin.get("/billing/activity"),
      ])

      const customersPayload: DashboardResponse<CustomerBalance> =
        customersRes.data?.customers
      const cyclesPayload: DashboardResponse<BillingCycleSummary> =
        cyclesRes.data?.cycles
      const invoicePayload: DashboardResponse<InvoiceRecord> =
        invoicesRes.data?.data
      const activityPayload: DashboardResponse<BillingGroup> =
        activityRes.data?.activity

      setCustomerBalances(Array.isArray(customersPayload) ? customersPayload : [])
      setBillingCycles(Array.isArray(cyclesPayload) ? cyclesPayload : [])
      setInvoices(Array.isArray(invoicePayload) ? invoicePayload : [])
      setActivity(Array.isArray(activityPayload) ? activityPayload : [])
    } catch (err: any) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
      } else {
        setError(getErrorMessage(err, "Unable to load billing dashboard."))
      }
      setCustomerBalances([])
      setBillingCycles([])
      setInvoices([])
      setActivity([])
    } finally {
      setIsLoading(false)
    }
  }, [org])

  useEffect(() => {
    void loadDashboard()
  }, [loadDashboard])

  const currencyFallback = useMemo(() => {
    return (
      customerBalances.find((item) => item.currency)?.currency || "USD"
    )
  }, [customerBalances])
  const hasBalanceCurrency = useMemo(
    () => customerBalances.some((item) => Boolean(item.currency)),
    [customerBalances]
  )

  const invoiceLookup = useMemo(() => {
    const map = new Map<string, InvoiceRecord>()
    for (const invoice of invoices) {
      const id = String(readField(invoice, ["id", "ID"]) ?? "")
      if (id) {
        map.set(id, invoice)
      }
    }
    return map
  }, [invoices])

  const latestInvoices = useMemo(() => invoices.slice(0, 6), [invoices])

  const currentCycle = useMemo(() => {
    if (billingCycles.length === 0) return null
    const openCycle = billingCycles.find((cycle) =>
      ["open", "closing"].includes(cycle.status?.toLowerCase())
    )
    return openCycle ?? billingCycles[0]
  }, [billingCycles])

  const IconAction = (act: string): any => {
    switch (act) {
      case "invoice.finalize":
        return <CircleCheck className="text-emerald-500" />
      case "invoice.generate":
        return <Receipt className="text-gray-500" />
      case "payment.received":
        return <CreditCard />
      default:
        return <></>
    }
  }

  if (!org) {
    return <Navigate to="/orgs" replace />
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to billing dashboards." />
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Billing dashboard</h1>
        <p className="text-text-muted text-sm">
          Track customer balances, revenue by cycle, and recent billing activity.
        </p>
      </div>

      {error ? (
        <Alert variant="destructive">{error}</Alert>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[1.35fr_1fr]">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Customer balances</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Customer</TableHead>
                    <TableHead>Balance</TableHead>
                    <TableHead>Last Invoice</TableHead>
                    <TableHead>Payment Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-text-muted">
                        Loading balances...
                      </TableCell>
                    </TableRow>
                  ) : customerBalances.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-text-muted">
                        No customer balances yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    customerBalances.map((customer) => {
                      const balanceView = formatBalance(customer.balance, customer.currency || currencyFallback)
                      const lastInvoice = customer.last_invoice_id
                        ? invoiceLookup.get(customer.last_invoice_id)
                        : null
                      const invoiceText = lastInvoice ? invoiceLabel(lastInvoice) : "-"
                      const invoiceLink = customer.last_invoice_id
                        ? `/orgs/${org.id}/invoices/${customer.last_invoice_id}`
                        : ""

                      return (
                        <TableRow key={customer.customer_id}>
                          <TableCell>{customer.name}</TableCell>
                          <TableCell className={balanceView.tone}>{balanceView.label}</TableCell>
                          <TableCell>
                            {invoiceLink ? (
                              <Link className="text-accent-primary hover:underline" to={invoiceLink}>
                                {invoiceText}
                              </Link>
                            ) : (
                              "-"
                            )}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={paymentBadgeStyle(customer.payment_status).variant}
                              className={paymentBadgeStyle(customer.payment_status).className}
                            >
                              {paymentLabel(customer.payment_status)}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Billing cycle dashboard</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap items-center justify-between gap-2 pb-3 text-xs text-text-muted">
                <span>Each row is a single billing cycle snapshot.</span>
                <span>Period format: YYYY-MM.</span>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Period</TableHead>
                    <TableHead className="text-right">Revenue</TableHead>
                    <TableHead className="text-right">Invoice Count</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-text-muted">
                        Loading billing cycles...
                      </TableCell>
                    </TableRow>
                  ) : billingCycles.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-text-muted">
                        No billing cycles yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    billingCycles.map((cycle) => (
                      <TableRow key={cycle.cycle_id}>
                        <TableCell className="font-mono text-xs">{cycle.period || "-"}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {formatCurrency(cycle.total_revenue, currencyFallback)}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {cycle.invoice_count}
                        </TableCell>
                        <TableCell>
                          <Badge variant={statusVariant(cycle.status)}>
                            {statusLabel(cycle.status)}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
              <div className="pt-3 text-xs text-text-muted">
                Revenue shown in {currencyFallback}.{" "}
                {hasBalanceCurrency
                  ? "Currency inferred from customer balances."
                  : "Currency defaults to USD when none are present."}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between gap-3">
              <CardTitle>Invoice overview</CardTitle>
              <Button asChild variant="secondary" size="sm">
                <Link to={`/orgs/${org.id}/invoices`}>View all</Link>
              </Button>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Invoice</TableHead>
                    <TableHead>Customer</TableHead>
                    {/* <TableHead>Subtotal</TableHead> */}
                    <TableHead>Total</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Due Date</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-text-muted">
                        Loading invoices...
                      </TableCell>
                    </TableRow>
                  ) : latestInvoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-text-muted">
                        No invoices yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    latestInvoices.map((invoice) => {
                      const id = String(readField(invoice, ["id", "ID"]) ?? "")
                      const customerID = String(
                        readField(invoice, ["customer_id", "CustomerID"]) ?? ""
                      )
                      const currency =
                        String(readField(invoice, ["currency", "Currency"]) ?? "") ||
                        currencyFallback
                      const subtotalRaw = readField(invoice, ["subtotal_amount", "SubtotalAmount"])
                      const subtotal = typeof subtotalRaw === "number" ? subtotalRaw : null
                      const status = String(readField(invoice, ["status", "Status"]) ?? "")
                      const dueAt = String(readField(invoice, ["due_at", "DueAt"]) ?? "")

                      const customerDetailLink = customerID ? `/orgs/${org.id}/customers/${customerID}` : ""
                      const detailLink = id ? `/orgs/${org.id}/invoices/${id}` : ""

                      return (
                        <TableRow key={id || invoiceLabel(invoice)}>
                          <TableCell>
                            {detailLink ? (
                              <Link className="text-accent-primary hover:underline" to={detailLink}>
                                {invoiceLabel(invoice)}
                              </Link>
                            ) : (
                              invoiceLabel(invoice)
                            )}
                          </TableCell>
                          <TableCell>
                            <Link className="text-accent-primary hover:underline" to={customerDetailLink}>
                              {customerID}
                            </Link>
                          </TableCell>
                          {/* <TableCell>{formatCurrency(subtotal, currency)}</TableCell> */}
                          <TableCell>{formatCurrency(subtotal, currency)}</TableCell>
                          <TableCell>
                            <Badge variant={invoiceStatusVariant(status)}>
                              {invoiceStatusLabel(status)}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDate(dueAt)}</TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Recent activity</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="rounded-lg border border-border-subtle bg-bg-surface-strong px-4 py-3">
                <p className="text-xs uppercase tracking-wide text-text-muted">Current cycle</p>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="text-base font-semibold">
                      {currentCycle ? currentCycle.period : "No active cycle"}
                    </p>
                    <p className="text-sm text-text-muted">
                      {currentCycle
                        ? `Status: ${statusLabel(currentCycle.status)}`
                        : "Waiting for the next billing period."}
                    </p>
                  </div>
                  {currentCycle ? (
                    <Badge variant={statusVariant(currentCycle.status)}>
                      {statusLabel(currentCycle.status)}
                    </Badge>
                  ) : null}
                </div>
              </div>

              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">Recent activity</p>
                  <Link className="text-xs text-accent-primary hover:underline" to={`/orgs/${org.id}/audit-logs`}>
                    View audit log
                  </Link>
                </div>
                <Separator />
                {isLoading ? (
                  <p className="text-sm text-text-muted">Loading activity...</p>
                ) : activity.length === 0 ? (
                  <p className="text-sm text-text-muted">No recent billing activity.</p>
                ) : (
                  // <ul className="space-y-3">
                  //   {activity.slice(0, 6).map((item, index) => (
                  //     <li key={`${item.message}-${index}`} className="flex items-start justify-between gap-4">
                  //       <div>
                  //         <p className="text-sm font-medium">{item.message}</p>
                  //         <p className="text-xs text-text-muted">
                  //           {formatDateTime(item.occurred_at)}
                  //         </p>
                  //       </div>
                  //     </li>
                  //   ))}
                  // </ul>
                  <>
                    {activity.map((group) => (
                      <div key={group.title}>
                        <h4 className="mb-2 text-xs font-semibold uppercase text-text-muted">
                          {group.title}
                        </h4>

                        <ul className="space-y-3">
                          {group.activities.slice(0, 6).map((item, index) => (
                            <li
                              key={`${item.message}-${index}`}
                              className="flex items-start justify-between gap-4"
                            >
                              <div className="grid gap-1.5">
                                <p className="flex text-sm font-medium">
                                  <span className="mr-2">{IconAction(item.action)}</span>
                                  {item.message}
                                </p>
                                <p className="text-xs text-text-muted">
                                  {formatDateTime(item.occurred_at)}
                                </p>
                              </div>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}
                  </>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
