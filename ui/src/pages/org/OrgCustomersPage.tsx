import { useCallback, useEffect, useMemo, useState } from "react"
import { useParams, useSearchParams } from "react-router-dom"
import { IconPlus } from "@tabler/icons-react"

import { api } from "@/api/client"
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
import { Checkbox } from "@/components/ui/checkbox"
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type Customer = {
  id: string | number
  name: string
  email: string
  created_at?: string
}

type CustomerListResponse = {
  customers: Customer[]
  next_page_token?: string
  previous_page_token?: string
  has_more?: boolean
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
  const [searchParams, setSearchParams] = useSearchParams()
  const [customers, setCustomers] = useState<Customer[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [listError, setListError] = useState<string | null>(null)
  const [createError, setCreateError] = useState<string | null>(null)
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [language, setLanguage] = useState("en-US")
  const [isCreating, setIsCreating] = useState(false)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [nextPageToken, setNextPageToken] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const nameFilter = searchParams.get("name") ?? ""
  const emailFilter = searchParams.get("email") ?? ""
  const createdFrom = searchParams.get("created_from") ?? ""
  const createdTo = searchParams.get("created_to") ?? ""
  const hasFilters = Boolean(
    nameFilter || emailFilter || createdFrom || createdTo
  )

  const loadCustomers = useCallback(async (options?: { pageToken?: string; append?: boolean }) => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    const { pageToken, append } = options ?? {}
    if (append) {
      setIsLoadingMore(true)
    } else {
      setIsLoading(true)
    }
    setListError(null)

    try {
      const res = await api.get("/customers", {
        params: {
          name: nameFilter || undefined,
          email: emailFilter || undefined,
          created_from: createdFrom || undefined,
          created_to: createdTo || undefined,
          page_token: pageToken,
          page_size: PAGE_SIZE,
        },
      })
      const payload: CustomerListResponse = res.data?.data ?? { customers: [] }
      const list = Array.isArray(payload.customers) ? payload.customers : []
      if (append) {
        setCustomers((prev) => [...prev, ...list])
      } else {
        setCustomers(list)
      }
      setNextPageToken(payload.next_page_token ?? null)
      setHasMore(Boolean(payload.has_more))
    } catch (err: any) {
      setListError(err?.message ?? "Unable to load customers.")
      if (!append) {
        setCustomers([])
      }
      setNextPageToken(null)
      setHasMore(false)
    } finally {
      if (append) {
        setIsLoadingMore(false)
      } else {
        setIsLoading(false)
      }
    }
  }, [createdFrom, createdTo, emailFilter, nameFilter, orgId])

  useEffect(() => {
    void loadCustomers()
  }, [loadCustomers])

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!orgId) return
    setIsCreating(true)
    setCreateError(null)
    try {
      await api.post("/customers", {
        organization_id: orgId,
        name,
        email,
      })
      await loadCustomers()
      setName("")
      setEmail("")
      setLanguage("en-US")
      setIsDialogOpen(false)
    } catch (err: any) {
      setCreateError(err?.message ?? "Unable to create customer.")
    } finally {
      setIsCreating(false)
    }
  }

  const countLabel = useMemo(() => {
    const total = customers.length
    return `${total} item${total === 1 ? "" : "s"}`
  }, [customers.length])

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Customers</h1>
          <p className="text-text-muted text-sm">
            Manage customers, payment methods, and billing contact data.
          </p>
        </div>
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
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="px-0 text-accent-primary"
              >
                More options
              </Button>
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

      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button variant="outline" size="sm">
          Copy
        </Button>
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

      {listError && <Alert variant="destructive">{listError}</Alert>}

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox aria-label="Select all customers" />
              </TableHead>
              <TableHead>Customer</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Primary payment method</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={6} className="text-text-muted">
                  Loading customers...
                </TableCell>
              </TableRow>
            )}
            {!isLoading && customers.length === 0 && !listError && (
              <TableRow>
                <TableCell colSpan={6} className="text-text-muted">
                  No customers yet.
                </TableCell>
              </TableRow>
            )}
            {!isLoading &&
              customers.length > 0 &&
              customers.map((customer) => (
                <TableRow key={customer.id}>
                  <TableCell>
                    <Checkbox aria-label={`Select ${customer.name}`} />
                  </TableCell>
                  <TableCell className="font-medium">{customer.name}</TableCell>
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
          </TableBody>
        </Table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span className="text-text-muted">{countLabel}</span>
        <Button
          variant="outline"
          size="sm"
          disabled={!hasMore || isLoadingMore}
          onClick={() => {
            if (nextPageToken) {
              void loadCustomers({ pageToken: nextPageToken, append: true })
            }
          }}
        >
          {isLoadingMore ? "Loading..." : "Load more"}
        </Button>
      </div>
    </div>
  )
}
