import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import {
  IconDotsVertical,
  IconPlus,
} from "@tabler/icons-react"

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
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

export default function OrgInvoicesPage() {
  const { orgId } = useParams()
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState("ALL")
  const [searchQuery, setSearchQuery] = useState("")

  useEffect(() => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    let isMounted = true
    setIsLoading(true)
    setError(null)

    Promise.allSettled([
      api.get("/invoices"),
      api.get("/customers", {
        params: { page_size: 200 },
      }),
    ])
      .then(([invoiceResult, customerResult]) => {
        if (!isMounted) return
        if (invoiceResult.status === "fulfilled") {
          setInvoices(invoiceResult.value.data?.data ?? [])
        } else {
          setError(
            invoiceResult.reason?.message ?? "Unable to load invoices."
          )
        }
        if (customerResult.status === "fulfilled") {
          const payload = customerResult.value.data?.data ?? {}
          const list = Array.isArray(payload.customers)
            ? payload.customers
            : []
          setCustomers(list)
        }
      })
      .finally(() => {
        if (!isMounted) return
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  const customersById = useMemo(() => {
    const map = new Map<string, Customer>()
    customers.forEach((customer) => {
      const id = readField(customer, ["id", "ID"])
      if (id) {
        map.set(id, customer)
      }
    })
    return map
  }, [customers])

  const filteredInvoices = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    return invoices.filter((invoice) => {
      const status = deriveStatus(invoice)
      if (statusFilter !== "ALL" && status !== statusFilter) {
        return false
      }
      if (!query) return true
      const invoiceNumber = getInvoiceNumber(invoice).toLowerCase()
      const customerId = readField(invoice, ["customer_id", "CustomerID"]) ?? ""
      const customer = customersById.get(customerId)
      const customerName = readField(customer, ["name", "Name"]) ?? ""
      const customerEmail = readField(customer, ["email", "Email"]) ?? ""
      return (
        invoiceNumber.includes(query) ||
        customerId.toLowerCase().includes(query) ||
        customerName.toLowerCase().includes(query) ||
        customerEmail.toLowerCase().includes(query)
      )
    })
  }, [customersById, invoices, searchQuery, statusFilter])

  const countLabel = useMemo(() => {
    const total = filteredInvoices.length
    return `${total} item${total === 1 ? "" : "s"}`
  }, [filteredInvoices.length])

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Invoices</h1>
          <p className="text-text-muted text-sm">
            Review invoices, due dates, and payment status for this organization.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {orgId && (
            <Button asChild variant="outline" size="sm">
              <Link to={`/orgs/${orgId}/invoice-templates`}>
                Manage templates
              </Link>
            </Button>
          )}
          <Button size="sm">
            <IconPlus />
            Create invoice
          </Button>
          <Button variant="outline" size="icon-sm" aria-label="Invoice actions">
            <IconDotsVertical />
          </Button>
        </div>
      </div>

      <Tabs value={statusFilter} onValueChange={setStatusFilter}>
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
          {filters.map((filter) => (
            <Button key={filter} variant="outline" size="sm" className="gap-2">
              <IconPlus />
              {filter}
            </Button>
          ))}
          <Button variant="ghost" size="sm" onClick={() => setStatusFilter("ALL")}>
            Clear filters
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm">
            Export
          </Button>
          <Button variant="outline" size="sm">
            Analyze
          </Button>
          <Button variant="outline" size="sm">
            Edit columns
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Input
          className="w-full max-w-md"
          placeholder="Search by invoice number or customer"
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
        <div className="text-text-muted text-sm">{countLabel}</div>
      </div>

      {isLoading && (
        <div className="text-text-muted text-sm">Loading invoices...</div>
      )}
      {error && <div className="text-status-error text-sm">{error}</div>}
      {!isLoading && !error && filteredInvoices.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No invoices yet</EmptyTitle>
            <EmptyDescription>
              Generate invoices from subscriptions or create an invoice to preview billing.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button size="sm">
              <IconPlus />
              Create invoice
            </Button>
          </EmptyContent>
        </Empty>
      )}
      {!isLoading && !error && filteredInvoices.length > 0 && (
        <div className="rounded-lg border">
          <Table className="min-w-[920px]">
            <TableHeader>
              <TableRow>
                <TableHead>Total</TableHead>
                <TableHead>Invoice number</TableHead>
                <TableHead>Customer</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Due</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredInvoices.map((invoice) => {
                const invoiceId = readField(invoice, ["id", "ID"]) ?? "-"
                const invoiceNumber = getInvoiceNumber(invoice)
                const status = deriveStatus(invoice)
                const statusLabel = formatStatus(status)
                const customerId =
                  readField(invoice, ["customer_id", "CustomerID"]) ?? ""
                const customer = customersById.get(customerId)
                const customerName =
                  readField(customer, ["name", "Name"]) ??
                  (customerId ? `Customer ${customerId}` : "-")
                const customerEmail =
                  readField(customer, ["email", "Email"]) ?? "-"
                const amount = readNumber(invoice, [
                  "subtotal_amount",
                  "SubtotalAmount",
                  "total_amount",
                  "TotalAmount",
                ])
                const currency = readField(invoice, ["currency", "Currency"]) ?? "USD"
                const dueDate = readField(invoice, ["due_at", "DueAt"])
                const createdAt = readField(invoice, ["created_at", "CreatedAt"])

                return (
                  <TableRow key={invoiceId}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <div className="font-medium">
                          {formatCurrency(amount, currency)}
                        </div>
                        <Badge variant={statusVariant(status)}>{statusLabel}</Badge>
                      </div>
                    </TableCell>
                    <TableCell className="font-medium">
                      {orgId ? (
                        <Link
                          to={`/orgs/${orgId}/invoices/${invoiceId}`}
                          className="hover:text-accent-primary"
                        >
                          {invoiceNumber}
                        </Link>
                      ) : (
                        invoiceNumber
                      )}
                    </TableCell>
                    <TableCell>{customerName}</TableCell>
                    <TableCell className="text-text-muted">
                      {customerEmail}
                    </TableCell>
                    <TableCell>{formatDateTime(dueDate)}</TableCell>
                    <TableCell>{formatDateTime(createdAt)}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label="Open invoice actions"
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
    </div>
  )
}

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

const statusTabs = [
  { value: "ALL", label: "All invoices" },
  { value: "DRAFT", label: "Draft" },
  { value: "FINALIZED", label: "Finalized" },
  { value: "VOID", label: "Void" },
]

const filters = [
  "Status",
  "Created",
  "Due date",
  "Scheduled finalization date",
  "Total",
  "More filters",
]

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
