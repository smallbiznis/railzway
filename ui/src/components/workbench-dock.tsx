import { useMemo, useState } from "react"
import {
  IconChevronDown,
  IconChevronUp,
  IconCopy,
  IconRefresh,
} from "@tabler/icons-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"

const apiKeys = [
  {
    name: "Default",
    publishable: "pk_test_51N8r3uB4X6dC9X0t",
    secret: "sk_test_51N8r3uB4X6dC9X0t",
    lastRotated: "2024-08-04",
  },
  {
    name: "Live",
    publishable: "pk_live_51M1x6yT8K2wQ7Z1p",
    secret: "sk_live_51M1x6yT8K2wQ7Z1p",
    lastRotated: "2024-07-21",
  },
]

const webhookEndpoints = [
  {
    id: "wh_92af",
    url: "https://api.valora.dev/hooks/billing",
    signingSecret: "whsec_c81c9s2Hk12",
    status: "Delivered",
    lastDelivery: "2 minutes ago",
  },
  {
    id: "wh_33bd",
    url: "https://api.valora.dev/hooks/invoices",
    signingSecret: "whsec_4k1p2n9Qx6",
    status: "Retrying",
    lastDelivery: "12 minutes ago",
  },
  {
    id: "wh_118c",
    url: "https://api.valora.dev/hooks/usage",
    signingSecret: "whsec_8u2k9p1Tq3",
    status: "Failed",
    lastDelivery: "42 minutes ago",
  },
]

const billingEvents = [
  {
    id: "evt_1Qf3g2xx",
    type: "billing.subscription.created",
    status: "Delivered",
    createdAt: "2024-08-09 09:32:18 UTC",
    payload: {
      id: "evt_1Qf3g2xx",
      type: "billing.subscription.created",
      data: {
        subscription_id: "sub_8rT9K2",
        customer_id: "cus_E1H2",
        plan: "pro",
        status: "active",
        billing_period_start: "2024-08-09T09:32:18Z",
      },
    },
  },
  {
    id: "evt_1Qf3g2xy",
    type: "billing.invoice.finalized",
    status: "Delivered",
    createdAt: "2024-08-08 18:07:44 UTC",
    payload: {
      id: "evt_1Qf3g2xy",
      type: "billing.invoice.finalized",
      data: {
        invoice_id: "inv_5A7X9",
        amount_due: 12900,
        currency: "usd",
        due_date: "2024-08-15",
        status: "open",
      },
    },
  },
  {
    id: "evt_1Qf3g2xz",
    type: "billing.usage.reported",
    status: "Failed",
    createdAt: "2024-08-08 12:45:02 UTC",
    payload: {
      id: "evt_1Qf3g2xz",
      type: "billing.usage.reported",
      data: {
        meter_id: "meter_3f9k",
        usage_value: 932,
        aggregation: "sum",
        status: "rejected",
        error: "rate_limit",
      },
    },
  },
]

function maskSecret(value: string) {
  if (value.length <= 10) {
    return `${value.slice(0, 3)}******`
  }
  return `${value.slice(0, 7)}******${value.slice(-4)}`
}

function statusBadgeVariant(
  status: string
): "default" | "secondary" | "destructive" {
  if (status === "Failed") return "destructive"
  if (status === "Retrying") return "secondary"
  return "default"
}

export function WorkbenchDock() {
  const [isOpen, setIsOpen] = useState(false)
  const [selectedEventId, setSelectedEventId] = useState(
    billingEvents[0]?.id ?? ""
  )

  const selectedEvent = useMemo(() => {
    return billingEvents.find((event) => event.id === selectedEventId)
  }, [selectedEventId])

  return (
    <div className="fixed inset-x-0 bottom-0 z-40">
      <div className="w-full">
        {isOpen ? (
          <section
            id="workbench-panel"
            aria-label="Workbench"
            className="bg-bg-surface/95 border-border-subtle shadow-md flex h-[60vh] min-h-[360px] flex-col border-t backdrop-blur"
          >
            <div className="border-border-subtle flex items-center justify-between border-b px-6 py-4">
              <div className="flex items-center gap-3">
                <div className="text-sm font-semibold">Workbench</div>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setIsOpen(false)}
              >
                Hide
                <IconChevronDown />
              </Button>
            </div>
            <div className="flex min-h-0 flex-1 flex-col gap-4 px-6 py-5">
              <Tabs defaultValue="overview" className="flex min-h-0 flex-1 flex-col">
                <TabsList className="w-full justify-start">
                  <TabsTrigger value="overview">Overview</TabsTrigger>
                  <TabsTrigger value="api-keys">API Keys</TabsTrigger>
                  <TabsTrigger value="webhooks">Webhooks</TabsTrigger>
                  <TabsTrigger value="events">Events</TabsTrigger>
                </TabsList>
                <TabsContent value="overview" className="flex min-h-0 flex-1 flex-col">
                  <div className="grid gap-4 lg:grid-cols-[1.2fr_1fr]">
                    <Card className="py-5">
                      <CardHeader className="pb-4">
                        <CardTitle>API requests (last 7 days)</CardTitle>
                        <CardDescription>All environments</CardDescription>
                      </CardHeader>
                      <CardContent>
                        <div className="text-3xl font-semibold">42,189</div>
                        <div className="text-text-muted mt-2 text-sm">
                          Peak day: 8,312 requests
                        </div>
                      </CardContent>
                    </Card>
                    <Card className="py-5">
                      <CardHeader className="pb-4">
                        <CardTitle>Success vs error</CardTitle>
                        <CardDescription>Last 7 days</CardDescription>
                      </CardHeader>
                      <CardContent className="space-y-3">
                        <div className="flex items-center justify-between text-sm">
                          <span className="text-text-muted">Success</span>
                          <span className="font-medium">98.2%</span>
                        </div>
                        <div className="bg-bg-subtle h-2 w-full overflow-hidden rounded-full">
                          <div className="bg-status-success h-full w-[98%]" />
                        </div>
                        <div className="flex items-center justify-between text-sm">
                          <span className="text-text-muted">Errors</span>
                          <span className="font-medium">1.8%</span>
                        </div>
                      </CardContent>
                    </Card>
                  </div>
                </TabsContent>
                <TabsContent value="api-keys" className="flex min-h-0 flex-1 flex-col">
                  <div className="bg-bg-primary border-border-subtle overflow-hidden rounded-xl border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Name</TableHead>
                          <TableHead>Publishable key</TableHead>
                          <TableHead>Secret key</TableHead>
                          <TableHead>Last rotated</TableHead>
                          <TableHead className="text-right">Actions</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {apiKeys.map((key) => (
                          <TableRow key={key.name}>
                            <TableCell className="font-medium">{key.name}</TableCell>
                            <TableCell className="text-text-muted font-mono text-xs">
                              {key.publishable}
                            </TableCell>
                            <TableCell className="text-text-muted font-mono text-xs">
                              {maskSecret(key.secret)}
                            </TableCell>
                            <TableCell className="text-text-muted text-xs">
                              {key.lastRotated}
                            </TableCell>
                            <TableCell>
                              <div className="flex items-center justify-end gap-2">
                                <Button variant="ghost" size="sm" type="button">
                                  <IconCopy />
                                  Copy
                                </Button>
                                <Button variant="ghost" size="sm" type="button">
                                  <IconRefresh />
                                  Rotate
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </TabsContent>
                <TabsContent value="webhooks" className="flex min-h-0 flex-1 flex-col">
                  <div className="bg-bg-primary border-border-subtle overflow-hidden rounded-xl border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Endpoint</TableHead>
                          <TableHead>Signing secret</TableHead>
                          <TableHead>Recent delivery</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {webhookEndpoints.map((endpoint) => (
                          <TableRow key={endpoint.id}>
                            <TableCell className="text-text-muted font-mono text-xs">
                              {endpoint.url}
                            </TableCell>
                            <TableCell className="text-text-muted font-mono text-xs">
                              {maskSecret(endpoint.signingSecret)}
                            </TableCell>
                            <TableCell>
                              <div className="flex items-center gap-2 text-xs">
                                <Badge variant={statusBadgeVariant(endpoint.status)}>
                                  {endpoint.status}
                                </Badge>
                                <span className="text-text-muted">
                                  {endpoint.lastDelivery}
                                </span>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </TabsContent>
                <TabsContent value="events" className="flex min-h-0 flex-1 flex-col">
                  <div className="grid min-h-0 gap-4 lg:grid-cols-[280px_1fr]">
                    <div className="border-border-subtle bg-bg-primary flex min-h-0 flex-col gap-2 overflow-auto rounded-xl border p-3">
                      {billingEvents.map((event) => {
                        const isSelected = event.id === selectedEventId
                        return (
                          <button
                            key={event.id}
                            type="button"
                            onClick={() => setSelectedEventId(event.id)}
                            className={cn(
                              "border-border-subtle text-left flex flex-col gap-2 rounded-lg border px-3 py-2 text-sm transition-colors",
                              isSelected
                                ? "bg-bg-subtle text-text-primary"
                                : "hover:bg-bg-subtle/50 text-text-secondary"
                            )}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <span className="font-medium">{event.type}</span>
                              <Badge variant={statusBadgeVariant(event.status)}>
                                {event.status}
                              </Badge>
                            </div>
                            <span className="text-text-muted font-mono text-xs">
                              {event.id}
                            </span>
                            <span className="text-text-muted text-xs">
                              {event.createdAt}
                            </span>
                          </button>
                        )
                      })}
                    </div>
                    <div className="border-border-subtle bg-bg-primary flex min-h-0 flex-col rounded-xl border p-4">
                      <div className="text-text-muted text-xs">Payload (read-only)</div>
                      <pre className="text-text-secondary mt-3 flex-1 overflow-auto rounded-lg bg-bg-subtle/40 p-3 text-xs font-mono">
                        {JSON.stringify(selectedEvent?.payload ?? {}, null, 2)}
                      </pre>
                    </div>
                  </div>
                </TabsContent>
              </Tabs>
            </div>
          </section>
        ) : null}
        <div className="bg-bg-surface/95 border-border-subtle flex h-12 items-center border-t px-4 shadow-sm backdrop-blur">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            aria-expanded={isOpen}
            aria-controls="workbench-panel"
            onClick={() => setIsOpen((prev) => !prev)}
          >
            Workbench
            {isOpen ? <IconChevronDown /> : <IconChevronUp />}
          </Button>
        </div>
      </div>
    </div>
  )
}
