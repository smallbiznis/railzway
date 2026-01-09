import { useCallback, useMemo, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"
import { IconPlus } from "@tabler/icons-react"

import { admin } from "@/api/client"
import { TableSkeleton } from "@/components/loading-skeletons"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert } from "@/components/ui/alert"
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
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useCursorPagination } from "@/hooks/useCursorPagination"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

type Customer = {
  id: string | number
  name: string
  email: string
  created_at?: string
}

const PAGE_SIZE = 25

const formatDateTime = (value?: string) => {
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

export default function OrgCustomersPage() {
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const [searchParams, setSearchParams] = useSearchParams()
  const [createError, setCreateError] = useState<string | null>(null)
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [language, setLanguage] = useState("en-US")
  const [isCreating, setIsCreating] = useState(false)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const nameFilter = searchParams.get("name") ?? ""
  const emailFilter = searchParams.get("email") ?? ""
  const createdFrom = searchParams.get("created_from") ?? ""
  const createdTo = searchParams.get("created_to") ?? ""
  const hasFilters = Boolean(
    nameFilter || emailFilter || createdFrom || createdTo
  )

  const fetchCustomers = useCallback(
    async (cursor: string | null) => {
      const res = await admin.get("/customers", {
        params: {
          name: nameFilter || undefined,
          email: emailFilter || undefined,
          created_from: createdFrom || undefined,
          created_to: createdTo || undefined,
          cursor: cursor || undefined,
          page_token: cursor || undefined,
          page_size: PAGE_SIZE,
        },
      })
      const payload = res.data?.data ?? {}
      const items = Array.isArray(payload.items)
        ? payload.items
        : Array.isArray(payload.customers)
          ? payload.customers
          : []
      const pageInfo = payload.page_info ?? payload

      return { items, page_info: pageInfo }
    },
    [createdFrom, createdTo, emailFilter, nameFilter]
  )

  const {
    items: customers,
    error: listError,
    isLoading,
    isLoadingMore,
    hasPrev,
    hasNext,
    loadNext,
    loadPrev,
    reload,
  } = useCursorPagination<Customer>(fetchCustomers, {
    enabled: Boolean(orgId),
    mode: "replace",
    dependencies: [orgId, nameFilter, emailFilter, createdFrom, createdTo],
  })

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!orgId) return
    setIsCreating(true)
    setCreateError(null)
    try {
      await admin.post("/customers", {
        organization_id: orgId,
        name,
        email,
      })
      await reload()
      setName("")
      setEmail("")
      setLanguage("en-US")
      setIsDialogOpen(false)
    } catch (err: any) {
      setCreateError(getErrorMessage(err, "Unable to create customer."))
    } finally {
      setIsCreating(false)
    }
  }

  const countLabel = useMemo(() => {
    const total = customers.length
    return `Showing ${total} customer${total === 1 ? "" : "s"}`
  }, [customers.length])

  const isForbidden = listError ? isForbiddenError(listError) : false
  const listErrorMessage =
    listError && !isForbidden
      ? getErrorMessage(listError, "Unable to load customers.")
      : null

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to customers." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Customers</h1>
          <p className="text-text-muted text-sm">
            Manage customers, payment methods, and billing contact data.
          </p>
        </div>
        {canManage ? (
          <Dialog
            open={isDialogOpen}
            onOpenChange={(open) => {
              setIsDialogOpen(open)
              if (!open) {
                setCreateError(null)
                setName("")
                setEmail("")
                setLanguage("en-US")
              }
            }}
          >
            <DialogTrigger asChild>
              <Button size="sm">
                <IconPlus />
                Add customer
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-xl">
              <DialogHeader>
                <DialogTitle>Create customer</DialogTitle>
                <DialogDescription>
                  Add a customer profile for billing and invoicing.
                </DialogDescription>
              </DialogHeader>
              <form className="space-y-4" onSubmit={handleCreate}>
                {createError && <Alert variant="destructive">{createError}</Alert>}
                <div className="space-y-2">
                  <Label htmlFor="customer-name">Name</Label>
                  <Input
                    id="customer-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Customer display name"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="customer-email">Email</Label>
                  <Input
                    id="customer-email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="billing@company.com"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="customer-language">Language</Label>
                  <Select value={language} onValueChange={setLanguage}>
                    <SelectTrigger id="customer-language">
                      <SelectValue placeholder="Select a language" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="en-US">English (United States)</SelectItem>
                      <SelectItem value="en-GB">English (United Kingdom)</SelectItem>
                      <SelectItem value="id-ID">Bahasa Indonesia</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <Separator />
                <DialogFooter>
                  <DialogClose asChild>
                    <Button type="button" variant="outline">
                      Cancel
                    </Button>
                  </DialogClose>
                  <Button type="submit" disabled={isCreating || !orgId}>
                    {isCreating ? "Saving..." : "Add customer"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        ) : (
          <Button size="sm" disabled>
            <IconPlus />
            Add customer
          </Button>
        )}
      </div>

      <Card>
        <CardHeader>
          <div>
            <CardTitle>Filters</CardTitle>
            <CardDescription>Filter customers by name, email, or created date.</CardDescription>
          </div>
          <CardAction>
            <Button
              variant="ghost"
              size="sm"
              disabled={!hasFilters}
              onClick={() => {
                const next = new URLSearchParams(searchParams)
                next.delete("name")
                next.delete("email")
                next.delete("created_from")
                next.delete("created_to")
                setSearchParams(next, { replace: true })
              }}
            >
              Clear filters
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="customer-filter-name">Customer name</Label>
              <Input
                id="customer-filter-name"
                placeholder="e.g. Acme Corp"
                value={nameFilter}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value.trim()
                  if (value) {
                    next.set("name", value)
                  } else {
                    next.delete("name")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="customer-filter-email">Email</Label>
              <Input
                id="customer-filter-email"
                placeholder="billing@company.com"
                value={emailFilter}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  const value = event.target.value.trim()
                  if (value) {
                    next.set("email", value)
                  } else {
                    next.delete("email")
                  }
                  setSearchParams(next, { replace: true })
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="customer-filter-created-from">Created from</Label>
              <Input
                id="customer-filter-created-from"
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
              <Label htmlFor="customer-filter-created-to">Created to</Label>
              <Input
                id="customer-filter-created-to"
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
        </CardContent>
      </Card>

      {listErrorMessage && <Alert variant="destructive">{listErrorMessage}</Alert>}

      {isLoading && customers.length === 0 && (
        <TableSkeleton
          rows={6}
          columnTemplate="grid-cols-[2fr_2fr_1fr_1fr_auto]"
          headerWidths={["w-24", "w-24", "w-20", "w-16", "w-6"]}
          cellWidths={["w-[70%]", "w-[75%]", "w-[60%]", "w-[60%]", "w-3"]}
        />
      )}

      {!isLoading && !listErrorMessage && customers.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No customers yet</EmptyTitle>
            <EmptyDescription>
              Add a customer to start billing subscriptions and invoices.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            {canManage && (
              <Button size="sm" onClick={() => setIsDialogOpen(true)}>
                Add customer
              </Button>
            )}
          </EmptyContent>
        </Empty>
      )}

      {customers.length > 0 && (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader className="[&_th]:sticky [&_th]:top-0 [&_th]:z-10 [&_th]:bg-bg-surface">
                <TableRow>
                  <TableHead>Customer</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Primary payment method</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {customers.map((customer) => (
                  <TableRow key={customer.id}>
                    <TableCell className="font-medium">
                      {orgId ? (
                        <Link
                          className="text-accent-primary hover:underline"
                          to={`/orgs/${orgId}/customers/${customer.id}`}
                        >
                          {customer.name}
                        </Link>
                      ) : (
                        customer.name
                      )}
                    </TableCell>
                    <TableCell className="text-text-muted">
                      {customer.email}
                    </TableCell>
                    <TableCell className="text-text-muted">-</TableCell>
                    <TableCell className="text-text-muted">
                      {formatDateTime(customer.created_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon-sm" aria-label="Open actions">
                        ...
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {isLoadingMore && (
                  <TableRow>
                    <TableCell colSpan={5}>
                      <div className="text-text-muted flex items-center gap-2 text-sm">
                        <Spinner className="size-4" />
                        Loading customers...
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
            <span className="text-text-muted">{countLabel}</span>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!hasPrev || isLoadingMore}
                onClick={() => void loadPrev()}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasNext || isLoadingMore}
                onClick={() => void loadNext()}
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
