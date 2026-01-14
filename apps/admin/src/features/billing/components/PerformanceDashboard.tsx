import { useEffect, useState } from "react"
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine
} from "recharts"
import {
  TrendingUp,
  AlertTriangle,
  ShieldCheck,
  Zap,
  CheckCircle2,
  Trophy,
  Loader2
} from "lucide-react"

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { admin } from "@/api/client"
import { cn } from "@/lib/utils"

type PerformanceMetrics = {
  avg_response_minutes: number
  completion_ratio: number
  escalation_ratio: number
  exposure_handled: number
  total_assigned: number
  total_resolved: number
  total_escalated: number
}

type PerformanceScores = {
  responsiveness: number
  completion: number
  effectiveness: number
  risk: number
  total: number
}

type FinOpsScoreSnapshot = {
  period_start: string
  period_end: string
  metrics: PerformanceMetrics
  scores: PerformanceScores
}

type PerformanceData = {
  current: FinOpsScoreSnapshot
  history: FinOpsScoreSnapshot[]
}

export function PerformanceDashboard({
  open,
  onOpenChange
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [data, setData] = useState<PerformanceData | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      setLoading(true)
      admin.get("/finops/performance/me", { params: { period_type: "daily" } })
        .then(res => {
          const apiData = res.data
          if (apiData?.snapshots && apiData.snapshots.length > 0) {
            setData({
              current: apiData.snapshots[0],
              history: apiData.snapshots
            })
          }
        })
        .catch(err => console.error("Failed to load performance", err))
        .finally(() => setLoading(false))
    }
  }, [open])

  if (!open) return null

  const score = data?.current?.scores?.total || 0
  const historyData = data?.history?.map(h => ({
    date: new Date(h.period_start).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
    score: h.scores.total
  })).reverse() || []

  // Coaching logic
  const getCoachingParam = () => {
    if (!data?.current) return null
    const s = data.current.scores
    const m = data.current.metrics

    if (s.responsiveness < 15) return {
      title: "Improve Responsiveness",
      desc: "Your average response time is high. Try to acknowledge assignments within 4 hours.",
      icon: Zap
    }
    if (s.completion < 15) return {
      title: "Close Implementation",
      desc: "Completion rate is lagging. Focus on resolving assigned tickets before picking new ones.",
      icon: CheckCircle2
    }
    if (s.risk < 15) return {
      title: "Reduce Escalations",
      desc: "Escalation rate is impactful. Ensure SLAs don't breach by acting before the deadline.",
      icon: AlertTriangle
    }
    if (s.effectiveness < 10 && m.total_resolved > 5) return {
      title: "Target High Value",
      desc: "You are resolving many items, but exposure handled is low. Prioritize critical invoices.",
      icon: Trophy
    }
    return {
      title: "Doing Great!",
      desc: "Your metrics are solid across the board. Keep maintaining this standard.",
      icon: ShieldCheck
    }
  }

  const coaching = getCoachingParam()

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[400px] sm:w-[540px] overflow-y-auto">
        <SheetHeader className="mb-6">
          <SheetTitle className="flex items-center gap-2">
            <Trophy className="h-5 w-5 text-amber-500" />
            Performance Dashboard
          </SheetTitle>
          <SheetDescription>
            Your FinOps impact and quality metrics for the current period.
          </SheetDescription>
        </SheetHeader>

        {loading ? (
          <div className="flex h-64 items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : !data ? (
          <div className="flex flex-col items-center justify-center h-96 text-center space-y-4">
            <div className="p-6 rounded-full bg-muted/50">
              <TrendingUp className="h-12 w-12 text-muted-foreground/30" />
            </div>
            <div className="space-y-2">
              <h3 className="text-xl font-semibold">No Activity Recorded</h3>
              <p className="text-muted-foreground max-w-xs mx-auto">
                Your performance metrics will appear here once you've resolved your first assignments.
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-8">
            {/* ... Total Score Section ... */}
            <div className="flex flex-col items-center justify-center p-6 bg-card border rounded-xl shadow-sm">
              <div className="relative flex items-center justify-center">
                {/* Ring using simple SVG or CSS */}
                <svg className="h-40 w-40 transform -rotate-90">
                  <circle
                    className="text-muted/20"
                    strokeWidth="12"
                    stroke="currentColor"
                    fill="transparent"
                    r="70"
                    cx="80"
                    cy="80"
                  />
                  <circle
                    className={cn(
                      score >= 80 ? "text-emerald-500" : score >= 60 ? "text-amber-500" : "text-red-500",
                      "transition-all duration-1000 ease-out"
                    )}
                    strokeWidth="12"
                    strokeDasharray={440}
                    strokeDashoffset={440 - (440 * score / 100)}
                    strokeLinecap="round"
                    stroke="currentColor"
                    fill="transparent"
                    r="70"
                    cx="80"
                    cy="80"
                  />
                </svg>
                <div className="absolute flex flex-col items-center">
                  <span className="text-4xl font-bold tracking-tighter">{score}</span>
                  <span className="text-xs uppercase text-muted-foreground font-medium">FinOps Score</span>
                </div>
              </div>
            </div>

            {/* Metrics Grid */}
            <div className="grid grid-cols-2 gap-4">
              <MetricCard
                title="Responsiveness"
                score={data?.current?.scores.responsiveness || 0}
                max={25}
                value={`${Math.round(data?.current?.metrics.avg_response_minutes || 0)}m avg`}
                icon={Zap}
                color="text-blue-500"
              />
              <MetricCard
                title="Completion"
                score={data?.current?.scores.completion || 0}
                max={25}
                value={`${Math.round((data?.current?.metrics.completion_ratio || 0) * 100)}%`}
                icon={CheckCircle2}
                color="text-emerald-500"
              />
              <MetricCard
                title="Risk Control"
                score={data?.current?.scores.risk || 0}
                max={25}
                value={`${Math.round((data?.current?.metrics.escalation_ratio || 0) * 100)}% esc`}
                icon={ShieldCheck}
                color="text-purple-500"
              />
              <MetricCard
                title="Effectiveness"
                score={data?.current?.scores.effectiveness || 0}
                max={25}
                value={`$${((data?.current?.metrics.exposure_handled || 0) / 100).toLocaleString()}`}
                icon={TrendingUp}
                color="text-amber-500"
              />
            </div>

            {/* Trend Chart */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">30-Day Trend</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="h-[200px] w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={historyData}>
                      <XAxis
                        dataKey="date"
                        stroke="#888888"
                        fontSize={12}
                        tickLine={false}
                        axisLine={false}
                      />
                      <YAxis
                        stroke="#888888"
                        fontSize={12}
                        tickLine={false}
                        axisLine={false}
                        domain={[0, 100]}
                      />
                      <Tooltip
                        contentStyle={{ borderRadius: "8px", border: "1px solid #e5e7eb" }}
                      />
                      <ReferenceLine y={80} stroke="#10b981" strokeDasharray="3 3" />
                      <Line
                        type="monotone"
                        dataKey="score"
                        stroke="hsl(var(--primary))"
                        strokeWidth={2}
                        dot={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>

            {/* Coaching */}
            {coaching && (
              <div className="rounded-lg border bg-muted/40 p-4">
                <div className="flex items-start gap-4">
                  <div className="p-2 rounded-full bg-background border shadow-sm">
                    <coaching.icon className="h-5 w-5 text-primary" />
                  </div>
                  <div>
                    <h4 className="font-medium text-sm">{coaching.title}</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      {coaching.desc}
                    </p>
                  </div>
                </div>
              </div>
            )}

            <div className="text-xs text-center text-muted-foreground pb-4">
              Scores are calculated daily based on resolved assignments and SLA compliance.
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

function MetricCard({ title, score, max, value, icon: Icon, color }: any) {
  return (
    <Card className="shadow-sm">
      <CardContent className="p-4 flex flex-col justify-between h-full space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">{title}</span>
          <Icon className={cn("h-4 w-4", color)} />
        </div>
        <div>
          <div className="text-2xl font-bold tracking-tight">{score}</div>
          <div className="text-xs text-muted-foreground">/ {max} pts</div>
        </div>
        <div className="pt-2 border-t mt-auto">
          <div className="text-xs font-mono font-medium">{value}</div>
        </div>
      </CardContent>
    </Card>
  )
}
