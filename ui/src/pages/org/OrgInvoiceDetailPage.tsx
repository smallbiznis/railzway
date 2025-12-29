import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { IconDotsVertical } from "@tabler/icons-react"

import { api } from "@/api/client"
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
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type Invoice = {
  id?: string | number
  ID?: string | number
  invoice_number?: number | string
  InvoiceNumber?: number | string
  customer_id?: string | number
  CustomerID?: string | number
  subscription_id?: string | number
  SubscriptionID?: string | number
  status?: string
  Status?: string
  subtotal_amount?: number | string
  SubtotalAmount?: number | string
  total_amount?: number | string
  TotalAmount?: number | string
  currency?: string
  Currency?: string
  issued_at?: string
  IssuedAt?: string
  due_at?: string
  DueAt?: string
  finalized_at?: string
  FinalizedAt?: string
  voided_at?: string
  VoidedAt?: string
  created_at?: string
  CreatedAt?: string
  metadata?: Record<string, unknown>
  Metadata?: Record<string, unknown>
}

type Customer = {
  id?: string | number
  ID?: string | number
  name?: string
  Name?: string
  email?: string
  Email?: string
}

type InvoiceLineItem = {
  description: string
  quantity: number
  unitAmount: number
  amount: number
}

export default function OrgInvoiceDetailPage() {
  const { orgId, invoiceId } = useParams()
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [customerError, setCustomerError] = useState<string | null>(null)

  useEffect(() => {
    if (!invoiceId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)

    api
      .get(`/invoices/${invoiceId}`)
      .then((response) => {
        if (!isMounted) return
        setInvoice(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!isMounted) return
        setError(err?.message ?? "Unable to load invoice.")
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [invoiceId, orgId])

  useEffect(() => {
    if (!orgId || !invoice) return
    const customerId = readField(invoice, ["customer_id", "CustomerID"])
    if (!customerId) return

    let active = true
    setCustomerError(null)

    api
      .get(`/customers/${customerId}`)
      .then((response) => {
        if (!active) return
        setCustomer(response.data?.data ?? null)
      })
      .catch((err) => {
        if (!active) return
        setCustomerError(err?.message ?? "Unable to load customer.")
      })

    return () => {
      active = false
    }
  }, [invoice, orgId])

  const invoiceNumber = useMemo(() => {
    if (!invoice) return "Invoice"
    return getInvoiceNumber(invoice)
  }, [invoice])

  const status = useMemo(() => {
    if (!invoice) return "DRAFT"
    return deriveStatus(invoice)
  }, [invoice])

  const amounts = useMemo(() => {
    if (!invoice) {
      return {
        total: null,
        tax: null,
        subtotal: null,
        paid: null,
        remaining: null,
        currency: "USD",
      }
    }

    const total = readNumber(invoice, [
      "subtotal_amount",
      "SubtotalAmount",
      "total_amount",
      "TotalAmount",
    ])
    const currency = readField(invoice, ["currency", "Currency"]) ?? "USD"
    const metadata = invoice.metadata ?? invoice.Metadata
    const tax = readMetadataNumber(metadata, [
      "tax",
      "tax_amount",
      "tax_amount_cents",
    ])
    const subtotal = total !== null && tax !== null ? total - tax : total
    const paid = readMetadataNumber(metadata, ["amount_paid", "paid_amount"])
    const remaining = total !== null ? total - (paid ?? 0) : null

    return { total, tax, subtotal, paid, remaining, currency }
  }, [invoice, status])

  const lineItems = useMemo(() => {
    if (!invoice) return []
    const metadata = invoice.metadata ?? invoice.Metadata
    if (!metadata) return []
    const raw = metadata.items ?? metadata.line_items ?? []
    if (!Array.isArray(raw)) return []
    return raw
      .map((item) => normalizeLineItem(item))
      .filter((item): item is InvoiceLineItem => item !== null)
  }, [invoice])

  const customerId = readField(invoice ?? undefined, ["customer_id", "CustomerID"])
  const customerName =
    readField(customer, ["name", "Name"]) ||
    (customerId ? `Customer ${customerId}` : "Customer")
  const customerEmail = readField(customer, ["email", "Email"]) ?? "-"
  const customerIdDisplay = customerId ?? "-"
  const createdAt = readField(invoice ?? undefined, ["created_at", "CreatedAt"])
  const dueAt = readField(invoice ?? undefined, ["due_at", "DueAt"])
  const issuedAt = readField(invoice ?? undefined, ["issued_at", "IssuedAt"])
  const invoiceIdDisplay = readField(invoice ?? undefined, ["id", "ID"]) ?? "-"

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading invoice...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (!invoice) {
    return <div className="text-text-muted text-sm">Invoice not found.</div>
  }

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to={`/orgs/${orgId}/invoices`}>Invoices</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{invoiceNumber}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-semibold">{invoiceNumber}</h1>
            <Badge variant={statusVariant(status)}>{formatStatus(status)}</Badge>
          </div>
          <p className="text-text-muted text-sm">
            Billed to{" "}
            {orgId ? (
              <Link className="text-accent-primary hover:underline" to={`/orgs/${orgId}/customers`}>
                {customerName}
              </Link>
            ) : (
              customerName
            )}{" "}
            - {formatCurrency(amounts.total, amounts.currency)}
          </p>
          {customerError && (
            <div className="text-status-error text-xs">{customerError}</div>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm">
            Edit draft
          </Button>
          <Button size="sm">Send invoice</Button>
          <Button variant="outline" size="sm" disabled>
            Charge customer
          </Button>
          <Button variant="outline" size="icon-sm" aria-label="Invoice actions">
            <IconDotsVertical />
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Recent activity</CardTitle>
              <Button variant="outline" size="sm">
                Add note
              </Button>
            </CardHeader>
            <CardContent>
              <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                No recent activity
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Summary</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-6 md:grid-cols-2 text-sm">
              <div className="space-y-4">
                <div>
                  <div className="text-text-muted">Billed to</div>
                  <div className="font-medium">{customerName}</div>
                  <div className="text-text-muted">{customerEmail}</div>
                </div>
                <div>
                  <div className="text-text-muted">Billing details</div>
                  <div className="font-medium">-</div>
                </div>
              </div>
              <div className="space-y-4">
                <div>
                  <div className="text-text-muted">Invoice number</div>
                  <div className="font-medium">{invoiceNumber}</div>
                </div>
                <div>
                  <div className="text-text-muted">Currency</div>
                  <div className="font-medium">{amounts.currency.toUpperCase()}</div>
                </div>
                <div>
                  <div className="text-text-muted">Billing method</div>
                  <div className="font-medium">Send invoice</div>
                </div>
                <div>
                  <div className="text-text-muted">Memo</div>
                  <div className="font-medium">-</div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Invoice items</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {lineItems.length === 0 ? (
                <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                  No line items on this invoice.
                </div>
              ) : (
                <div className="rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Description</TableHead>
                        <TableHead>Qty</TableHead>
                        <TableHead>Unit price</TableHead>
                        <TableHead>Amount</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {lineItems.map((item, index) => (
                        <TableRow key={`${item.description}-${index}`}>
                          <TableCell className="font-medium">{item.description}</TableCell>
                          <TableCell>{item.quantity}</TableCell>
                          <TableCell>
                            {formatCurrency(item.unitAmount, amounts.currency)}
                          </TableCell>
                          <TableCell>
                            {formatCurrency(item.amount, amounts.currency)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
              <div className="flex justify-end">
                <div className="grid w-full max-w-xs gap-2 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-text-muted">Subtotal</span>
                    <span className="font-medium">
                      {formatCurrency(amounts.subtotal, amounts.currency)}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-text-muted">Tax</span>
                    <span className="font-medium">
                      {amounts.tax === null ? "-" : formatCurrency(amounts.tax, amounts.currency)}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-text-muted">Total</span>
                    <span className="font-semibold">
                      {formatCurrency(amounts.total, amounts.currency)}
                    </span>
                  </div>
                  <Separator />
                  <div className="flex items-center justify-between">
                    <span className="text-text-muted">Amount paid</span>
                    <span className="font-medium">
                      {formatCurrency(amounts.paid ?? 0, amounts.currency)}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-text-muted">Amount remaining</span>
                    <span className="font-medium">
                      {formatCurrency(amounts.remaining, amounts.currency)}
                    </span>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Tax calculation</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                No tax rate applied.
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Accounting</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center justify-between gap-3 text-sm">
              <div className="text-text-muted">
                Use Revenue Recognition to automate accrual accounting for invoices.
              </div>
              <Button variant="link" size="sm">
                Go to Revenue Recognition
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Payments</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                No payments yet. Payments received on this invoice will appear here.
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Credit notes</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                No credit notes. Credits issued for this invoice will appear here.
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <div>
                <div className="text-text-muted">ID</div>
                <div className="font-medium">{invoiceIdDisplay}</div>
              </div>
              <div>
                <div className="text-text-muted">Customer ID</div>
                <div className="font-medium">{customerIdDisplay}</div>
              </div>
              <div>
                <div className="text-text-muted">Created</div>
                <div className="font-medium">{formatDateTime(createdAt)}</div>
              </div>
              <div>
                <div className="text-text-muted">Issued</div>
                <div className="font-medium">{formatDateTime(issuedAt)}</div>
              </div>
              <div>
                <div className="text-text-muted">Due</div>
                <div className="font-medium">{formatDateTime(dueAt)}</div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Metadata</CardTitle>
              <Button variant="outline" size="icon-sm" aria-label="Edit metadata">
                ...
              </Button>
            </CardHeader>
            <CardContent>
              {hasMetadata(invoice) ? (
                <div className="space-y-2 text-sm">
                  {Object.entries(invoice.metadata ?? invoice.Metadata ?? {}).map(
                    ([key, value]) => (
                      <div key={key} className="flex items-start justify-between gap-3">
                        <span className="text-text-muted">{key}</span>
                        <span className="font-medium">{String(value)}</span>
                      </div>
                    )
                  )}
                </div>
              ) : (
                <div className="rounded-lg border border-dashed p-6 text-center text-text-muted text-sm">
                  No metadata
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

const readField = <T extends Record<string, unknown>>(
  record: T | null | undefined,
  keys: (keyof T)[]
): string | null => {
  if (!record) return null
  for (const key of keys) {
    const value = record[key]
    if (value === undefined || value === null) continue
    if (typeof value === "string" && value.trim()) return value
    if (typeof value === "number") return String(value)
  }
  return null
}

const readNumber = <T extends Record<string, unknown>>(
  record: T,
  keys: (keyof T)[]
): number | null => {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === "number" && Number.isFinite(value)) return value
    if (typeof value === "string" && value.trim()) {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) return parsed
    }
  }
  return null
}

const readMetadataNumber = (
  metadata: Record<string, unknown> | undefined,
  keys: string[]
) => {
  if (!metadata) return null
  for (const key of keys) {
    const value = metadata[key]
    if (typeof value === "number" && Number.isFinite(value)) return value
    if (typeof value === "string" && value.trim()) {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) return parsed
    }
  }
  return null
}

const readMetadataValue = (
  metadata: Record<string, unknown> | undefined,
  keys: string[]
) => {
  if (!metadata) return null
  for (const key of keys) {
    const value = metadata[key]
    if (typeof value === "string" && value.trim()) return value
  }
  return null
}

const getInvoiceNumber = (invoice: Invoice) => {
  const invoiceNumber = readField(invoice, [
    "invoice_number",
    "InvoiceNumber",
  ])
  if (invoiceNumber) return String(invoiceNumber)
  const metadata = invoice.metadata ?? invoice.Metadata
  const fromMetadata = readMetadataValue(metadata, ["invoice_number", "number"])
  if (fromMetadata) return fromMetadata
  return readField(invoice, ["id", "ID"]) ?? "Invoice"
}

const deriveStatus = (invoice: Invoice) => {
  return (
    readField(invoice, ["status", "Status"])?.toUpperCase() ?? "UNKNOWN"
  )
}

const formatStatus = (status?: string) => {
  switch (status) {
    case "DRAFT":
      return "Draft"
    case "FINALIZED":
      return "Finalized"
    case "VOID":
      return "Void"
    default:
      return status ?? "-"
  }
}

const statusVariant = (status?: string) => {
  switch (status) {
    case "FINALIZED":
      return "secondary"
    case "DRAFT":
    case "VOID":
      return "outline"
    default:
      return "secondary"
  }
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

const formatCurrency = (amount: number | null, currency: string) => {
  if (amount === null) return "-"
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

const normalizeLineItem = (item: unknown): InvoiceLineItem | null => {
  if (!item || typeof item !== "object") return null
  const record = item as Record<string, unknown>
  const description =
    readField(record, ["description", "name", "title"]) ?? "Line item"
  const quantity = readNumber(record, ["quantity", "qty"]) ?? 0
  const unitAmount =
    readNumber(record, ["unit_price", "unit_amount", "unitAmount"]) ?? 0
  const amount = readNumber(record, ["amount", "total"]) ?? 0
  return { description, quantity, unitAmount, amount }
}

const hasMetadata = (invoice: Invoice) => {
  const metadata = invoice.metadata ?? invoice.Metadata
  return metadata && Object.keys(metadata).length > 0
}
