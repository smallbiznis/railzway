import { Loader2, Inbox } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Card, CardContent, CardDescription, CardTitle } from "@/components/ui/card"
import { useInbox, useClaimAssignment } from "../hooks/useIA"
import { formatCurrency } from "../utils/formatting"
import { cn } from "@/lib/utils"

export function InboxTab() {
  const { data, isLoading, error } = useInbox()
  const claimMutation = useClaimAssignment()

  const handleClaim = (entityType: string, entityId: string) => {
    claimMutation.mutate({
      entity_type: entityType as "invoice" | "customer",
      entity_id: entityId,
      assignment_ttl_minutes: 120, // 2 hours
    })
  }

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-64 items-center justify-center text-destructive">
        Failed to load inbox
      </div>
    )
  }

  const items = data?.items || []
  const invoices = items.filter(item => item.category === "overdue_invoice" || item.category === "high_exposure")
  const paymentIssues = items.filter(item => item.category === "failed_payment")

  const renderSection = (title: string, sectionItems: typeof items) => {
    if (sectionItems.length === 0) return null

    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2 px-1">
          <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">{title}</h3>
          <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
            {sectionItems.length}
          </Badge>
        </div>
        <div className="rounded-xl border border-border/60 bg-card shadow-sm overflow-hidden">
          <Table>
            <TableHeader className="bg-muted/30">
              <TableRow className="hover:bg-transparent">
                <TableHead className="font-medium">Issue Type</TableHead>
                <TableHead className="font-medium">Entity</TableHead>
                <TableHead className="font-medium">Customer</TableHead>
                <TableHead className="font-medium">Amount / Exposure</TableHead>
                <TableHead className="font-medium">Days Overdue</TableHead>
                <TableHead className="text-right font-medium">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sectionItems.map((item) => {
                const isClaiming = claimMutation.isPending
                const isFailedPayment = item.category === "failed_payment"

                return (
                  <TableRow key={`${item.entity_type}-${item.entity_id}`} className="group">
                    <TableCell>
                      <Badge
                        variant="secondary"
                        className={cn(
                          "border",
                          isFailedPayment
                            ? "bg-red-500/10 text-red-600 border-red-500/20"
                            : "bg-amber-500/10 text-amber-600 border-amber-500/20"
                        )}
                      >
                        {item.category.replace(/_/g, " ")}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {item.entity_name}
                    </TableCell>
                    <TableCell className="font-medium">
                      {item.customer_name || "N/A"}
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">
                      {item.amount_due !== undefined
                        ? formatCurrency(item.amount_due, item.currency || data?.currency)
                        : "N/A"}
                    </TableCell>
                    <TableCell>
                      {item.days_overdue !== undefined ? (
                        <span
                          className={cn(
                            "font-medium",
                            item.days_overdue > 30
                              ? "text-red-600"
                              : item.days_overdue > 14
                                ? "text-amber-600"
                                : "text-muted-foreground"
                          )}
                        >
                          {item.days_overdue} days
                        </span>
                      ) : (
                        "N/A"
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="default"
                        className="h-7 text-xs opacity-0 transition-opacity group-hover:opacity-100"
                        disabled={isClaiming}
                        onClick={() => handleClaim(item.entity_type, item.entity_id)}
                      >
                        {isClaiming ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                          "Claim"
                        )}
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {items.length === 0 ? (
        <Card className="border-dashed">
          <CardContent className="py-12 flex flex-col items-center justify-center text-center">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-muted mb-4">
              <Inbox className="h-6 w-6 text-muted-foreground" />
            </div>
            <CardTitle className="text-lg font-medium">Clear Skies!</CardTitle>
            <CardDescription className="max-w-sm mx-auto mt-1">
              No pending invoices or payment issues. Everything is currently on track.
            </CardDescription>
          </CardContent>
        </Card>
      ) : (
        <>
          {renderSection("Invoices", invoices)}
          {renderSection("Payment Issues", paymentIssues)}
        </>
      )}
    </div>
  )
}
