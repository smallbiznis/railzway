import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"
import {
  Calendar as CalendarIcon,
  ChevronRight,
  Info,
  MoreHorizontal,
  Plus,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
} from "@/components/ui/input-group"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { admin } from "@/api/client"

type Customer = {
  id: string | number
  name?: string
  email?: string
}

type CustomerListResponse = {
  customers: Customer[]
}

type Product = {
  id: string | number
  name?: string
  code?: string
  active?: boolean
}

const formatCustomerLabel = (customer: Customer) => {
  const name = customer.name?.trim()
  const email = customer.email?.trim()
  if (name && email) return `${name} - ${email}`
  if (name) return name
  if (email) return email
  return `Customer ${customer.id}`
}

const formatProductLabel = (product: Product) => {
  const name = product.name?.trim()
  const code = product.code?.trim()
  if (name && code) return `${name} - ${code}`
  if (name) return name
  if (code) return code
  return `Product ${product.id}`
}

const formatDate = (value?: Date) => {
  if (!value) return "Select date"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  }).format(value)
}

const addMonths = (date: Date, months: number) => {
  const result = new Date(date)
  const day = result.getDate()
  result.setDate(1)
  result.setMonth(result.getMonth() + months)
  const daysInMonth = new Date(
    result.getFullYear(),
    result.getMonth() + 1,
    0
  ).getDate()
  result.setDate(Math.min(day, daysInMonth))
  return result
}

export default function OrgSubscriptionCreatePage() {
  const { orgId } = useParams()
  const subscriptionsPath = orgId ? `/orgs/${orgId}/subscriptions` : "/orgs"
  const [customers, setCustomers] = useState<Customer[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [customersLoading, setCustomersLoading] = useState(false)
  const [productsLoading, setProductsLoading] = useState(false)
  const [customersError, setCustomersError] = useState<string | null>(null)
  const [productsError, setProductsError] = useState<string | null>(null)
  const [selectedCustomer, setSelectedCustomer] = useState("")
  const [selectedProduct, setSelectedProduct] = useState("")
  const [subscriptionStart, setSubscriptionStart] = useState<Date | undefined>(
    new Date()
  )
  const [billStart, setBillStart] = useState<Date | undefined>(new Date())
  const [durationMode, setDurationMode] = useState<"forever" | "cycles">(
    "forever"
  )
  const [durationCycles, setDurationCycles] = useState(1)
  const [customCycles, setCustomCycles] = useState("1")

  const baseDurationDate = subscriptionStart ?? new Date()
  const durationEnd =
    durationMode === "cycles"
      ? addMonths(baseDurationDate, durationCycles)
      : undefined
  const durationEndLabel =
    durationMode === "forever" ? "Forever" : formatDate(durationEnd)
  const shortcutCycles = [1, 2, 3, 6, 12]

  const applyCustomCycles = () => {
    const parsed = Number(customCycles)
    if (!Number.isFinite(parsed) || parsed <= 0) return
    const rounded = Math.floor(parsed)
    setDurationMode("cycles")
    setDurationCycles(rounded)
  }

  useEffect(() => {
    if (!orgId) {
      setCustomers([])
      return
    }
    let isMounted = true
    setCustomersLoading(true)
    setCustomersError(null)

    admin
      .get("/customers", {
        params: {
          page_size: 200,
        },
      })
      .then((response) => {
        if (!isMounted) return
        const payload: CustomerListResponse = response.data?.data ?? {
          customers: [],
        }
        const list = Array.isArray(payload.customers) ? payload.customers : []
        setCustomers(list)
      })
      .catch((err) => {
        if (!isMounted) return
        setCustomersError(err?.message ?? "Unable to load customers.")
        setCustomers([])
      })
      .finally(() => {
        if (!isMounted) return
        setCustomersLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  useEffect(() => {
    if (!orgId) {
      setProducts([])
      return
    }
    let isMounted = true
    setProductsLoading(true)
    setProductsError(null)

    admin
      .get("/products")
      .then((response) => {
        if (!isMounted) return
        const list = Array.isArray(response.data?.data)
          ? response.data?.data
          : []
        setProducts(list)
      })
      .catch((err) => {
        if (!isMounted) return
        setProductsError(err?.message ?? "Unable to load products.")
        setProducts([])
      })
      .finally(() => {
        if (!isMounted) return
        setProductsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [orgId])

  return (
    <div className="space-y-8">
      <div className="space-y-2">
        <div className="text-sm text-text-muted">
          <Link className="text-accent-primary hover:underline" to={subscriptionsPath}>
            Subscriptions
          </Link>{" "}
          / Create subscription
        </div>
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Create a subscription</h1>
          <p className="text-text-muted text-sm">
            Configure billing phases, pricing, and payment behavior for this
            customer.
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Customer</CardTitle>
          <CardDescription>
            Choose who will be billed for this subscription.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label htmlFor="subscription-customer">Customer</Label>
            <Select
              value={selectedCustomer}
              onValueChange={setSelectedCustomer}
              disabled={customersLoading}
            >
              <SelectTrigger id="subscription-customer" className="w-full">
                <SelectValue
                  placeholder={
                    customersLoading
                      ? "Loading customers..."
                      : "Select a customer"
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {customersLoading && (
                  <SelectItem value="loading" disabled>
                    Loading customers...
                  </SelectItem>
                )}
                {!customersLoading && customers.length === 0 && (
                  <SelectItem value="empty" disabled>
                    No customers found
                  </SelectItem>
                )}
                {customers.map((customer) => (
                  <SelectItem key={customer.id} value={String(customer.id)}>
                    {formatCustomerLabel(customer)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {customersError && (
              <p className="text-status-error text-sm">{customersError}</p>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Subscription details</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label>Duration</Label>
            <div className="flex flex-wrap items-center gap-2 rounded-md border bg-bg-primary px-3 py-2 text-sm shadow-xs">
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="ghost" size="sm" className="gap-2 px-1">
                    <CalendarIcon className="size-4 text-text-muted" />
                    <span>{formatDate(subscriptionStart)}</span>
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start">
                  <Calendar
                    mode="single"
                    selected={subscriptionStart}
                    onSelect={setSubscriptionStart}
                    initialFocus
                  />
                </PopoverContent>
              </Popover>
              <ChevronRight className="size-4 text-text-muted" />
              <Popover>
                <PopoverTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="gap-2 px-1 text-text-muted"
                  >
                    {durationEndLabel}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-72 p-4" align="start">
                  <div className="space-y-4">
                    <div className="text-sm font-semibold">Shortcuts</div>
                    <div className="space-y-2">
                      <Button
                        variant="link"
                        size="sm"
                        className={`h-auto justify-start px-0 text-base ${
                          durationMode === "forever"
                            ? "text-accent-primary"
                            : "text-accent-primary/80"
                        }`}
                        onClick={() => setDurationMode("forever")}
                      >
                        Forever
                      </Button>
                      {shortcutCycles.map((cycles) => {
                        const dateLabel = formatDate(
                          addMonths(baseDurationDate, cycles)
                        )
                        const isActive =
                          durationMode === "cycles" && durationCycles === cycles
                        return (
                          <Button
                            key={cycles}
                            variant="link"
                            size="sm"
                            className={`h-auto justify-start px-0 text-base ${
                              isActive ? "text-accent-primary" : "text-accent-primary/80"
                            }`}
                            onClick={() => {
                              setDurationMode("cycles")
                              setDurationCycles(cycles)
                            }}
                          >
                            <span>{cycles} cycle{cycles === 1 ? "" : "s"}</span>
                            <span className="text-text-muted">
                              {" "}
                              ({dateLabel})
                            </span>
                          </Button>
                        )
                      })}
                    </div>
                    <div className="flex items-center gap-2">
                      <InputGroup className="flex-1">
                        <InputGroupInput
                          type="number"
                          min={1}
                          value={customCycles}
                          onChange={(event) =>
                            setCustomCycles(event.target.value)
                          }
                        />
                        <InputGroupAddon align="inline-end">
                          <InputGroupText>cycles</InputGroupText>
                        </InputGroupAddon>
                      </InputGroup>
                      <Button size="sm" onClick={applyCustomCycles}>
                        Apply
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          </div>

          <div className="space-y-3">
            <Label>Pricing</Label>
            <div className="overflow-hidden rounded-lg border">
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Product</TableHead>
                      <TableHead className="w-[120px]">Qty</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                      <TableHead className="w-[60px]" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="min-w-[240px]">
                        <Select
                          value={selectedProduct}
                          onValueChange={setSelectedProduct}
                          disabled={productsLoading}
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue
                              placeholder={
                                productsLoading
                                  ? "Loading products..."
                                  : "Select a product"
                              }
                            />
                          </SelectTrigger>
                          <SelectContent>
                            {productsLoading && (
                              <SelectItem value="loading" disabled>
                                Loading products...
                              </SelectItem>
                            )}
                            {!productsLoading && products.length === 0 && (
                              <SelectItem value="empty" disabled>
                                No products found
                              </SelectItem>
                            )}
                            {products.map((product) => (
                              <SelectItem
                                key={product.id}
                                value={String(product.id)}
                              >
                                {formatProductLabel(product)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell>
                        <Input
                          type="number"
                          min={1}
                          defaultValue={1}
                          className="w-20"
                        />
                      </TableCell>
                      <TableCell className="text-right text-text-muted">
                        -
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="Row actions"
                        >
                          <MoreHorizontal className="size-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
              {productsError && (
                <div className="border-t px-4 py-2 text-status-error text-sm">
                  {productsError}
                </div>
              )}
              <div className="flex flex-wrap items-center gap-4 border-t px-4 py-3">
                <Button variant="link" size="sm" className="px-0">
                  <Plus className="size-4" />
                  Add product
                </Button>
                <Button variant="link" size="sm" className="px-0">
                  Add coupon
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Phases</CardTitle>
          <CardDescription>
            Control when billing starts, trials, and metadata per phase.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-bg-subtle/10 p-4">
            <div className="flex items-center gap-3">
              <Switch id="collect-tax" />
              <Label htmlFor="collect-tax" className="text-sm font-medium">
                Collect tax automatically
              </Label>
            </div>
            <Button variant="link" size="sm" className="px-0">
              Add tax manually
            </Button>
          </div>

          <div className="space-y-6 rounded-lg border bg-bg-primary p-4 shadow-xs">
            <div className="space-y-2">
              <Label>Bill starting</Label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    className="justify-start gap-2"
                  >
                    <CalendarIcon className="size-4" />
                    {formatDate(billStart)}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start">
                  <Calendar
                    mode="single"
                    selected={billStart}
                    onSelect={setBillStart}
                    initialFocus
                  />
                </PopoverContent>
              </Popover>
              <p className="text-text-muted text-sm">
                This is when the first invoice will be generated.
              </p>
            </div>
            <div className="space-y-2">
              <Label>Free trial days</Label>
              <Button variant="outline" size="sm" className="justify-start gap-2">
                <Plus className="size-4" />
                Add trial days
              </Button>
            </div>
            <div className="space-y-2">
              <Label>Metadata</Label>
              <Button variant="link" size="sm" className="px-0">
                <Plus className="size-4" />
                Add metadata
              </Button>
            </div>
          </div>

          <Button variant="outline" className="w-full justify-center gap-2">
            <Plus className="size-4" />
            Add phase
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Subscription settings</CardTitle>
          <CardDescription>
            These settings apply to the entire subscription and span across all
            phases.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-3">
            <div className="text-sm font-semibold">Billing</div>
            <div className="flex flex-wrap items-center gap-3">
              <Switch id="bill-upfront" />
              <Label htmlFor="bill-upfront" className="text-sm font-medium">
                Bill upfront
              </Label>
              <Button variant="outline" size="sm">
                Preview
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="Billing preview info"
              >
                <Info className="size-4" />
              </Button>
            </div>
          </div>

          <Separator />

          <div className="space-y-3">
            <div className="text-sm font-semibold">Payment</div>
            <RadioGroup defaultValue="invoice" className="gap-3">
              <div className="flex cursor-not-allowed items-start gap-3 rounded-lg border p-3 opacity-60">
                <RadioGroupItem
                  value="auto"
                  id="payment-auto"
                  className="mt-1"
                  disabled
                />
                <Label htmlFor="payment-auto" className="flex flex-col gap-1">
                  <span className="text-sm font-medium">
                    Automatically charge a payment method on file
                  </span>
                  <span className="text-text-muted text-sm">
                    Automatic charge is disabled for this subscription.
                  </span>
                </Label>
              </div>
              <div className="flex items-start gap-3 rounded-lg border p-3">
                <RadioGroupItem
                  value="invoice"
                  id="payment-invoice"
                  className="mt-1"
                />
                <Label htmlFor="payment-invoice" className="flex flex-col gap-1">
                  <span className="text-sm font-medium">
                    Email invoice to the customer to pay manually
                  </span>
                  <span className="text-text-muted text-sm">
                    Issue invoices with a due date for manual payment.
                  </span>
                </Label>
              </div>
            </RadioGroup>
          </div>

          <Separator />

          <div className="space-y-3">
            <div className="text-sm font-semibold">Advanced settings</div>
            <div className="space-y-2">
              <Label htmlFor="invoice-template">Invoice template</Label>
              <Select>
                <SelectTrigger id="invoice-template">
                  <SelectValue placeholder="Select a template..." />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default invoice template</SelectItem>
                  <SelectItem value="modern">Modern invoice template</SelectItem>
                  <SelectItem value="minimal">Minimal invoice template</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center gap-3">
        <Button>Create subscription</Button>
        <Button variant="outline" asChild>
          <Link to={subscriptionsPath}>Cancel</Link>
        </Button>
      </div>
    </div>
  )
}
