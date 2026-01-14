import {
  LineChart,
  Line,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine
} from "recharts"
import {
  ShieldCheck,
  Zap,
} from "lucide-react"

import { Card, CardContent } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { useFinOpsPerformance } from "../hooks/useIA"
import { Skeleton } from "@/components/ui/skeleton"

export function BillingAnalyticsHeader() {
  const { data, isLoading } = useFinOpsPerformance("daily")

  if (isLoading) {
    return <AnalyticsSkeleton />
  }

  if (!data) return null

  const score = data.current?.scores.total || 0
  const historyData = data.history.map(h => ({
    date: new Date(h.period_start).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
    score: h.scores.total
  })).reverse()

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-8">
      {/* Total Score Card */}
      <Card className="border-border/60 shadow-sm relative overflow-hidden">
        <CardContent className="p-4 flex flex-row items-center justify-between h-full">
          <div className="space-y-1">
            <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              FinOps Score
            </p>
            <div className="flex items-baseline gap-2">
              <span className={cn(
                "text-2xl font-bold tracking-tight",
                score >= 80 ? "text-emerald-500" : score >= 60 ? "text-amber-500" : "text-red-500"
              )}>
                {score}
              </span>
              <span className="text-xs text-muted-foreground">/ 100</span>
            </div>
          </div>
          <div className="h-12 w-12 flex items-center justify-center relative">
            <svg className="h-full w-full transform -rotate-90">
              <circle
                className="text-muted/20"
                strokeWidth="4"
                stroke="currentColor"
                fill="transparent"
                r="18"
                cx="24"
                cy="24"
              />
              <circle
                className={cn(
                  score >= 80 ? "text-emerald-500" : score >= 60 ? "text-amber-500" : "text-red-500",
                  "transition-all duration-1000 ease-out"
                )}
                strokeWidth="4"
                strokeDasharray={113}
                strokeDashoffset={113 - (113 * score / 100)}
                strokeLinecap="round"
                stroke="currentColor"
                fill="transparent"
                r="18"
                cx="24"
                cy="24"
              />
            </svg>
          </div>
        </CardContent>
      </Card>

      {/* Responsiveness */}
      <MetricCard
        title="Responsiveness"
        value={`${Math.round(data.current.metrics.avg_response_minutes)}m avg`}
        icon={Zap}
        color="text-blue-500"
        subtext="Target: < 4h"
      />

      {/* Risk Control */}
      <MetricCard
        title="Risk Control"
        value={`${Math.round(data.current.metrics.escalation_ratio * 100)}% active`}
        icon={ShieldCheck}
        color="text-purple-500"
        subtext="Escalations"
      />

      {/* Mini Trend Chart */}
      <Card className="border-border/60 shadow-sm">
        <CardContent className="p-0 h-24">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={historyData}>
              <Tooltip
                contentStyle={{ borderRadius: "8px", border: "1px solid #e5e7eb", fontSize: "12px" }}
                itemStyle={{ padding: 0 }}
                labelStyle={{ display: "none" }}
              />
              <ReferenceLine y={80} stroke="#10b981" strokeDasharray="3 3" strokeOpacity={0.5} />
              <Line
                type="monotone"
                dataKey="score"
                stroke="hsl(var(--primary))"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>
    </div>
  )
}

function MetricCard({ title, value, icon: Icon, color, subtext }: any) {
  return (
    <Card className="border-border/60 shadow-sm">
      <CardContent className="p-4 flex flex-col justify-between h-full">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{title}</span>
          <Icon className={cn("h-4 w-4", color)} />
        </div>
        <div>
          <div className="text-xl font-bold tracking-tight">{value}</div>
          <div className="text-[10px] text-muted-foreground mt-1">{subtext}</div>
        </div>
      </CardContent>
    </Card>
  )
}

function AnalyticsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-8">
      {[...Array(4)].map((_, i) => (
        <Skeleton key={i} className="h-24 w-full rounded-xl" />
      ))}
    </div>
  )
}
