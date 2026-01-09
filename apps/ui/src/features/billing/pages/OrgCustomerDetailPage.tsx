import { useEffect, useMemo, useState } from "react"
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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

type Customer = {
  id?: string | number
  ID?: string | number
  name?: string
  Name?: string
  email?: string
  Email?: string
  currency?: string
  Currency?: string
  created_at?: string
  CreatedAt?: string
  metadata?: Record<string, unknown>
  Metadata?: Record<string, unknown>
}

type Subscription = {
  id?: string | number
  ID?: string | number
  status?: string
  Status?: string
  collection_mode?: string
  CollectionMode?: string
  start_at?: string
  StartAt?: string
  updated_at?: string
  UpdatedAt?: string
}

type Invoice = {
  id?: string | number
  ID?: string | number
  invoice_number?: string | number
  InvoiceNumber?: string | number
  status?: string
  Status?: string
  total_amount?: number | string
  TotalAmount?: number | string
  currency?: string
  Currency?: string
  created_at?: string
  CreatedAt?: string
}

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

const formatStatus = (value?: string) => {
  if (!value) return "-"
  const normalized = value.toUpperCase()
  switch (normalized) {
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
    case "FINALIZED":
      return "Finalized"
    case "VOID":
      return "Void"
    default:
      return normalized
  }
}

const statusVariant = (value?: string) => {
  const normalized = value?.toUpperCase()
  if (normalized === "ACTIVE" || normalized === "FINALIZED") return "secondary"
  if (normalized === "VOID" || normalized === "CANCELED" || normalized === "ENDED") {
    return "outline"
  }
  return "outline"
}

const formatCurrency = (amount: number, currency: string) => {
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

const readAmount = (item: Invoice) => {
  const raw = item.total_amount ?? item.TotalAmount
  if (typeof raw === "number") return raw
  if (typeof raw === "string") {
    const parsed = Number(raw)
    return Number.isNaN(parsed) ? null : parsed
  }
  return null
}

export default function OrgCustomerDetailPage() {
  const { orgId, customerId } = useParams()
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [isLoadingRelated, setIsLoadingRelated] = useState(true)
  const [relatedError, setRelatedError] = useState<string | null>(null)

  useEffect(() => {
    if (!customerId) {
      setIsLoading(false)
      return
    }
    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/customers/${customerId}`)
      .then((response) => {
        if (!active) return
        setCustomer(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load customer."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [customerId])

  useEffect(() => {
    if (!customerId || isForbidden) return
    let active = true
    setIsLoadingRelated(true)
    setRelatedError(null)

    Promise.all([
      admin.get("/subscriptions", { params: { customer_id: customerId } }),
      admin.get("/invoices", { params: { customer_id: customerId } }),
    ])
      .then(([subscriptionsRes, invoicesRes]) => {
        if (!active) return
        setSubscriptions(subscriptionsRes.data?.data ?? [])
        setInvoices(invoicesRes.data?.data ?? [])
      })
      .catch((err) => {
        if (!active) return
        setRelatedError(getErrorMessage(err, "Unable to load linked records."))
        setSubscriptions([])
        setInvoices([])
      })
      .finally(() => {
        if (!active) return
        setIsLoadingRelated(false)
      })

    return () => {
      active = false
    }
  }, [customerId, isForbidden])

  const customerName = useMemo(() => readField(customer, ["name", "Name"], "Customer"), [customer])
  const customerEmail = useMemo(() => readField(customer, ["email", "Email"]), [customer])
  const customerCurrency = useMemo(
    () => readField(customer, ["currency", "Currency"], "-"),
    [customer]
  )
  const createdAt = useMemo(
    () => readField(customer, ["created_at", "CreatedAt"], ""),
    [customer]
  )
  const customerIdDisplay = readField(customer, ["id", "ID"])

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading customer...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to this customer." />
  }

  if (!customer) {
    return <div className="text-text-muted text-sm">Customer not found.</div>
  }

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to={`/orgs/${orgId}/customers`}>Customers</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{customerName}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">{customerName}</h1>
        <p className="text-text-muted text-sm">
          Billing profile and active subscriptions for this customer.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-text-muted">Customer ID</CardTitle>
          </CardHeader>
          <CardContent className="text-sm font-medium">{customerIdDisplay}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-text-muted">Email</CardTitle>
          </CardHeader>
          <CardContent className="text-sm font-medium">{customerEmail}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-text-muted">Currency</CardTitle>
          </CardHeader>
          <CardContent className="text-sm font-medium">{customerCurrency}</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Customer overview</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm md:grid-cols-2">
          <div>
            <div className="text-text-muted">Created</div>
            <div className="font-medium">{formatDate(createdAt)}</div>
          </div>
          <div>
            <div className="text-text-muted">Metadata</div>
            <div className="font-medium">
              {Object.keys(customer.metadata ?? customer.Metadata ?? {}).length || 0} fields
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Subscriptions</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoadingRelated && (
            <div className="text-text-muted text-sm">Loading subscriptions...</div>
          )}
          {relatedError && <div className="text-status-error text-sm">{relatedError}</div>}
          {!isLoadingRelated && !relatedError && subscriptions.length === 0 && (
            <div className="text-text-muted text-sm">No subscriptions for this customer.</div>
          )}
          {!isLoadingRelated && !relatedError && subscriptions.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Subscription</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Collection</TableHead>
                  <TableHead>Start date</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {subscriptions.map((subscription, index) => {
                  const subscriptionId = readField(subscription, ["id", "ID"], "")
                  const status = readField(subscription, ["status", "Status"], "")
                  const collectionMode = readField(
                    subscription,
                    ["collection_mode", "CollectionMode"],
                    "-"
                  )
                  const startAt = readField(subscription, ["start_at", "StartAt"], "")
                  const updatedAt = readField(subscription, ["updated_at", "UpdatedAt"], "")
                  return (
                    <TableRow key={subscriptionId || `subscription-${index}`}>
                      <TableCell className="font-medium">
                        {subscriptionId ? (
                          <Link
                            className="text-accent-primary hover:underline"
                            to={`/orgs/${orgId}/subscriptions/${subscriptionId}`}
                          >
                            {subscriptionId}
                          </Link>
                        ) : (
                          "-"
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(status)}>{formatStatus(status)}</Badge>
                      </TableCell>
                      <TableCell>{collectionMode}</TableCell>
                      <TableCell>{formatDate(startAt)}</TableCell>
                      <TableCell>{formatDate(updatedAt)}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Invoices</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoadingRelated && (
            <div className="text-text-muted text-sm">Loading invoices...</div>
          )}
          {relatedError && <div className="text-status-error text-sm">{relatedError}</div>}
          {!isLoadingRelated && !relatedError && invoices.length === 0 && (
            <div className="text-text-muted text-sm">No invoices for this customer.</div>
          )}
          {!isLoadingRelated && !relatedError && invoices.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Invoice</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Total</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invoices.map((invoice, index) => {
                  const invoiceId = readField(invoice, ["id", "ID"], "")
                  const invoiceNumber = readField(invoice, ["invoice_number", "InvoiceNumber"], "-")
                  const status = readField(invoice, ["status", "Status"], "")
                  const total = readAmount(invoice)
                  const currency = readField(invoice, ["currency", "Currency"], "USD")
                  const createdAtInvoice = readField(invoice, ["created_at", "CreatedAt"], "")
                  return (
                    <TableRow key={invoiceId || `invoice-${index}`}>
                      <TableCell className="font-medium">
                        {invoiceId ? (
                          <Link
                            className="text-accent-primary hover:underline"
                            to={`/orgs/${orgId}/invoices/${invoiceId}`}
                          >
                            {invoiceNumber}
                          </Link>
                        ) : (
                          invoiceNumber
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(status)}>{formatStatus(status)}</Badge>
                      </TableCell>
                      <TableCell>
                        {total === null ? "-" : formatCurrency(total, currency)}
                      </TableCell>
                      <TableCell>{formatDate(createdAtInvoice)}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
