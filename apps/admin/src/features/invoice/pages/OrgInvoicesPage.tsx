import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { admin } from "@/api/client"
import { TableSkeleton } from "@/components/loading-skeletons"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useCursorPagination } from "@/hooks/useCursorPagination"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

const PAGE_SIZE = 20

export default function OrgInvoicesPage() {
  const { orgId } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [customers, setCustomers] = useState<Customer[]>([])
  const [isCustomerForbidden, setIsCustomerForbidden] = useState(false)
  const statusParam = searchParams.get("status") ?? "ALL"
  const statusFilter = statusTabs.some((tab) => tab.value === statusParam)
    ? statusParam
    : "ALL"
  const invoiceNumberFilter = searchParams.get("invoice_number") ?? ""
  const customerIdFilter = searchParams.get("customer_id") ?? ""
  const createdFrom = searchParams.get("created_from") ?? ""
  const createdTo = searchParams.get("created_to") ?? ""
  const dueFrom = searchParams.get("due_from") ?? ""
  const dueTo = searchParams.get("due_to") ?? ""
  const finalizedFrom = searchParams.get("finalized_from") ?? ""
  const finalizedTo = searchParams.get("finalized_to") ?? ""
  const totalMin = searchParams.get("total_min") ?? ""
  const totalMax = searchParams.get("total_max") ?? ""
  const hasFilters = Boolean(
    statusFilter !== "ALL" ||
    invoiceNumberFilter ||
    customerIdFilter ||
    createdFrom ||
    createdTo ||
    dueFrom ||
    dueTo ||
    finalizedFrom ||
    finalizedTo ||
    totalMin ||
    totalMax
  )
  const hasAdvancedDateFilters = Boolean(dueFrom || dueTo || finalizedFrom || finalizedTo)

  const fetchInvoices = useCallback(
    async (cursor: string | null) => {
      const response = await admin.get("/invoices", {
        params: {
          status: statusFilter === "ALL" ? undefined : statusFilter,
          invoice_number: invoiceNumberFilter || undefined,
          customer_id: customerIdFilter || undefined,
          created_from: createdFrom || undefined,
          created_to: createdTo || undefined,
          due_from: dueFrom || undefined,
          due_to: dueTo || undefined,
          finalized_from: finalizedFrom || undefined,
          finalized_to: finalizedTo || undefined,
          total_min: totalMin || undefined,
          total_max: totalMax || undefined,
          page_token: cursor || undefined,
          page_size: PAGE_SIZE,
        },
      })

      const payload = response.data?.data
      const items = Array.isArray(payload?.items)
        ? payload.items
        : Array.isArray(payload)
          ? payload
          : []
      const pageInfo = payload?.page_info ?? response.data?.page_info ?? null

      return { items, page_info: pageInfo }
    },
    [
      createdFrom,
      createdTo,
      customerIdFilter,
      dueFrom,
      dueTo,
      finalizedFrom,
      finalizedTo,
      invoiceNumberFilter,
      statusFilter,
      totalMax,
      totalMin,
    ]
  )

  const {
    items: invoices,
    error: invoiceError,
    isLoading,
    isLoadingMore,
    hasPrev,
    hasNext,
    loadNext,
    loadPrev,
  } = useCursorPagination<Invoice>(fetchInvoices, {
    enabled: Boolean(orgId),
    mode: "replace",
    dependencies: [
      orgId,
      createdFrom,
      createdTo,
      customerIdFilter,
      dueFrom,
      dueTo,
      finalizedFrom,
      finalizedTo,
      invoiceNumberFilter,
      statusFilter,
      totalMax,
      totalMin,
    ],
  })

  useEffect(() => {
    if (!orgId) {
      return
    }

    let isMounted = true
    setIsCustomerForbidden(false)

    admin
      .get("/customers", {
        params: { page_size: 50 },
      })
      .then((response) => {
        if (!isMounted) return
        const payload = response.data?.data ?? {}
        const list = Array.isArray(payload.customers)
          ? payload.customers
          : []
        setCustomers(list)
      })
      .catch((err) => {
        if (!isMounted) return
        if (isForbiddenError(err)) {
          setIsCustomerForbidden(true)
        }
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

  const countLabel = useMemo(() => {
    const total = invoices.length
    return `Showing ${total} invoice${total === 1 ? "" : "s"}`
  }, [invoices.length])

  const isInvoiceForbidden = invoiceError ? isForbiddenError(invoiceError) : false
  const invoiceErrorMessage =
    invoiceError && !isInvoiceForbidden
      ? getErrorMessage(invoiceError, "Unable to load invoices.")
      : null

  if (isInvoiceForbidden || isCustomerForbidden) {
    return <ForbiddenState description="You do not have access to invoices." />
  }

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
        </div>
      </div>

      <Tabs
        value={statusFilter}
        onValueChange={(value) => {
          const next = new URLSearchParams(searchParams)
          if (value === "ALL") {
            next.delete("status")
          } else {
            next.set("status", value)
          }
          setSearchParams(next, { replace: true })
        }}
      >
        <TabsList className="flex w-full flex-wrap justify-start">
          {statusTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <Card>
        <CardHeader>
          <div>
            <CardTitle>Filters</CardTitle>
            <CardDescription>Filter invoices by customer, totals, and dates.</CardDescription>
          </div>
          <CardAction>
            <Button
              variant="ghost"
              size="sm"
              disabled={!hasFilters}
              onClick={() => {
                const next = new URLSearchParams(searchParams)
                next.delete("status")
                next.delete("invoice_number")
                next.delete("customer_id")
                next.delete("created_from")
                next.delete("created_to")
                next.delete("due_from")
                next.delete("due_to")
                next.delete("finalized_from")
                next.delete("finalized_to")
                next.delete("total_min")
                next.delete("total_max")
                setSearchParams(next, { replace: true })
              }}
            >
              Clear filters
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-customer">Customer ID</Label>
              <Input
                id="invoice-filter-customer"
                placeholder="e.g. 1234567890"
                value={customerIdFilter}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value.trim()
                  if (value) {
                    next.set("customer_id", value)
                  } else {
                    next.delete("customer_id")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-number">Invoice number</Label>
              <Input
                id="invoice-filter-number"
                placeholder="e.g. INV-1001"
                value={invoiceNumberFilter}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value.trim()
                  if (value) {
                    next.set("invoice_number", value)
                  } else {
                    next.delete("invoice_number")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-total-min">Total min</Label>
              <Input
                id="invoice-filter-total-min"
                type="number"
                placeholder="0"
                value={totalMin}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value
                  if (value) {
                    next.set("total_min", value)
                  } else {
                    next.delete("total_min")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-total-max">Total max</Label>
              <Input
                id="invoice-filter-total-max"
                type="number"
                placeholder="0"
                value={totalMax}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value
                  if (value) {
                    next.set("total_max", value)
                  } else {
                    next.delete("total_max")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-created-from">Created from</Label>
              <Input
                id="invoice-filter-created-from"
                type="date"
                value={createdFrom}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value
                  if (value) {
                    next.set("created_from", value)
                  } else {
                    next.delete("created_from")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="invoice-filter-created-to">Created to</Label>
              <Input
                id="invoice-filter-created-to"
                type="date"
                value={createdTo}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value
                  if (value) {
                    next.set("created_to", value)
                  } else {
                    next.delete("created_to")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
          </div>

          <Collapsible defaultOpen={hasAdvancedDateFilters}>
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="w-fit px-0">
                Due and finalized dates
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="invoice-filter-due-from">Due from</Label>
                  <Input
                    id="invoice-filter-due-from"
                    type="date"
                    value={dueFrom}
                    onChange={(event) => {
                      const next = new URLSearchParams(searchParams)
                      const value = event.target.value
                      if (value) {
                        next.set("due_from", value)
                      } else {
                        next.delete("due_from")
                      }
                      setSearchParams(next, { replace: true })
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="invoice-filter-due-to">Due to</Label>
                  <Input
                    id="invoice-filter-due-to"
                    type="date"
                    value={dueTo}
                    onChange={(event) => {
                      const next = new URLSearchParams(searchParams)
                      const value = event.target.value
                      if (value) {
                        next.set("due_to", value)
                      } else {
                        next.delete("due_to")
                      }
                      setSearchParams(next, { replace: true })
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="invoice-filter-finalized-from">Finalized from</Label>
                  <Input
                    id="invoice-filter-finalized-from"
                    type="date"
                    value={finalizedFrom}
                    onChange={(event) => {
                      const next = new URLSearchParams(searchParams)
                      const value = event.target.value
                      if (value) {
                        next.set("finalized_from", value)
                      } else {
                        next.delete("finalized_from")
                      }
                      setSearchParams(next, { replace: true })
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="invoice-filter-finalized-to">Finalized to</Label>
                  <Input
                    id="invoice-filter-finalized-to"
                    type="date"
                    value={finalizedTo}
                    onChange={(event) => {
                      const next = new URLSearchParams(searchParams)
                      const value = event.target.value
                      if (value) {
                        next.set("finalized_to", value)
                      } else {
                        next.delete("finalized_to")
                      }
                      setSearchParams(next, { replace: true })
                    }}
                  />
                </div>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </CardContent>
      </Card>

      {isLoading && invoices.length === 0 && (
        <TableSkeleton
          rows={7}
          columnTemplate="grid-cols-[1.3fr_1.2fr_1.4fr_1.5fr_1fr_1fr_auto]"
          headerWidths={["w-20", "w-24", "w-24", "w-28", "w-16", "w-20", "w-6"]}
          cellWidths={["w-[70%]", "w-[60%]", "w-[75%]", "w-[70%]", "w-[60%]", "w-[60%]", "w-3"]}
        />
      )}
      {invoiceErrorMessage && (
        <div className="text-status-error text-sm">{invoiceErrorMessage}</div>
      )}
      {!isLoading && !invoiceErrorMessage && invoices.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No invoices yet</EmptyTitle>
            <EmptyDescription>
              Generate invoices from subscriptions or create an invoice to preview billing.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent />
        </Empty>
      )}
      {invoices.length > 0 && (
        <>
          <div className="rounded-lg border">
            <Table className="min-w-[920px]">
              <TableHeader className="[&_th]:sticky [&_th]:top-0 [&_th]:z-10 [&_th]:bg-bg-surface">
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
                {invoices.map((invoice) => {
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
                {isLoadingMore && (
                  <TableRow>
                    <TableCell colSpan={7}>
                      <div className="text-text-muted flex items-center gap-2 text-sm">
                        <Spinner className="size-4" />
                        Loading invoices...
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="text-text-muted text-sm">{countLabel}</div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => void loadPrev()}
                disabled={!hasPrev || isLoadingMore}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void loadNext()}
                disabled={!hasNext || isLoadingMore}
              >
                Next
              </Button>
            </div>
          </div>
        </>
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
