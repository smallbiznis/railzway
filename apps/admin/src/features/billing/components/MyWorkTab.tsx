import { useState } from "react"
import { Link } from "react-router-dom"
import { useOrgStore } from "@/stores/orgStore"
import { Loader2, CheckCircle2, ChevronRight, XCircle, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useMyWork, useResolveAssignment, useReleaseAssignment } from "../hooks/useIA"
import { SLABadge } from "./SLABadge"
import { ResolveDialog } from "./ResolveDialog"
import { formatCurrency, formatAssignmentAge, formatTimeRemaining } from "../utils/formatting"
import { cn } from "@/lib/utils"

export function MyWorkTab() {
  const org = useOrgStore((s) => s.currentOrg)
  const { data, isLoading, error } = useMyWork()
  const resolveMutation = useResolveAssignment()
  const releaseMutation = useReleaseAssignment()

  const [resolveTarget, setResolveTarget] = useState<{
    type: "invoice" | "customer"
    id: string
    name: string
  } | null>(null)

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
        Failed to load your work queue
      </div>
    )
  }

  const items = data?.items || []

  const handleResolveSubmit = (resolution: string) => {
    if (!resolveTarget) return
    resolveMutation.mutate({
      entity_type: resolveTarget.type,
      entity_id: resolveTarget.id,
      resolution: resolution,
    })
  }

  const handleRelease = (type: "invoice" | "customer", id: string) => {
    releaseMutation.mutate({
      entity_type: type,
      entity_id: id,
      reason: "manual_release",
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          You have {items.length} {items.length === 1 ? "active task" : "active tasks"}
        </p>
      </div>

      <div className="rounded-xl border border-border/60 bg-card shadow-sm overflow-hidden">
        <Table>
          <TableHeader className="bg-muted/30">
            <TableRow className="hover:bg-transparent">
              <TableHead className="font-medium">Entity</TableHead>
              <TableHead className="font-medium">Snapshot Amount</TableHead>
              <TableHead className="font-medium">Due (at Claim)</TableHead>
              <TableHead className="font-medium">Age</TableHead>
              <TableHead className="font-medium">SLA Status</TableHead>
              <TableHead className="text-right font-medium">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="h-32 text-center text-muted-foreground">
                  Your work queue is empty. Claim something from the Inbox! 🚀
                </TableCell>
              </TableRow>
            ) : (
              items.map((item) => {
                const timeRemaining = formatTimeRemaining(item.assignment_expires_at)

                return (
                  <TableRow key={item.assignment_id} className="group">
                    <TableCell>
                      <div className="flex flex-col">
                        <Link
                          to={`/orgs/${org?.id}/${item.entity_type}s/${item.entity_id}`}
                          className="font-medium hover:underline hover:text-primary transition-colors"
                        >
                          {item.entity_name}
                        </Link>
                        <span className="text-[10px] text-muted-foreground uppercase font-mono tracking-wider">
                          {item.entity_type}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">
                      {formatCurrency(item.amount_due_at_claim, item.currency)}
                    </TableCell>
                    <TableCell>
                      <span className={cn(
                        "font-medium",
                        item.days_overdue_at_claim > 30 ? "text-red-600" : "text-muted-foreground"
                      )}>
                        {item.days_overdue_at_claim} days
                      </span>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatAssignmentAge(item.claimed_at)}
                    </TableCell>
                    <TableCell>
                      <SLABadge
                        status={item.sla_status}
                        minutesRemaining={timeRemaining.minutes}
                      />
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          size="sm"
                          variant="secondary"
                          className="h-7 text-xs bg-emerald-500/10 text-emerald-600 border-emerald-500/20 hover:bg-emerald-500/20"
                          onClick={() => setResolveTarget({
                            type: item.entity_type,
                            id: item.entity_id,
                            name: item.entity_name
                          })}
                        >
                          <CheckCircle2 className="h-3 w-3 mr-1" />
                          Resolve
                        </Button>

                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                              <ChevronRight className="h-4 w-4 rotate-90" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              className="text-amber-600 focus:text-amber-600"
                              onClick={() => handleRelease(item.entity_type, item.entity_id)}
                            >
                              <XCircle className="h-4 w-4 mr-2" />
                              Release to Inbox
                            </DropdownMenuItem>
                            <DropdownMenuItem className="text-red-600 focus:text-red-600">
                              <AlertCircle className="h-4 w-4 mr-2" />
                              Escalate to Manager
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      <ResolveDialog
        open={!!resolveTarget}
        onOpenChange={(open) => !open && setResolveTarget(null)}
        onSubmit={handleResolveSubmit}
        entityType={resolveTarget?.type || "invoice"}
        entityName={resolveTarget?.name || ""}
      />
    </div>
  )
}
