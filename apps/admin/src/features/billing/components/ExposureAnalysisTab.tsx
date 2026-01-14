import { useExposureAnalysis } from "../hooks/useIA"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from "recharts"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"

export function ExposureAnalysisTab() {
  const { data, isLoading } = useExposureAnalysis()

  if (isLoading) return <ExposureSkeleton />
  if (!data) return <div className="p-4 text-center text-muted-foreground">No exposure data available</div>

  const formatCurrency = (val: number) => {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: data.currency }).format(val / 100)
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-3">
        {/* Total Exposure Card */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Exposure</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatCurrency(data.total_exposure)}</div>
            <p className="text-xs text-muted-foreground">Total outstanding amount</p>
          </CardContent>
        </Card>

        {/* Overdue */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Overdue Amount</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-amber-600">
              {formatCurrency(data.by_risk_category.find(c => c.category === 'Overdue')?.amount || 0)}
            </div>
            <p className="text-xs text-muted-foreground">Past due date</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Exposure by Aging</CardTitle></CardHeader>
          <CardContent className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data.by_aging_bucket}>
                <XAxis dataKey="bucket" fontSize={12} stroke="#888888" />
                <YAxis fontSize={12} stroke="#888888" tickFormatter={(val) => `$${val / 100}`} />
                <Tooltip
                  formatter={(val: number) => formatCurrency(val)}
                  contentStyle={{ backgroundColor: "#1f2937", border: "none", color: "#fff" }}
                />
                <Bar dataKey="amount" fill="#3b82f6" radius={[4, 4, 0, 0]}>
                  {data.by_aging_bucket.map((_, index) => (
                    <Cell key={index} fill={index === 0 ? "#10b981" : index > 2 ? "#ef4444" : "#f59e0b"} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Top High Exposure Accounts</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Customer</TableHead>
                  <TableHead className="text-right">Outstanding</TableHead>
                  <TableHead className="text-right">Risk Score</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.top_high_exposure.length === 0 && (
                  <TableRow><TableCell colSpan={3} className="text-center">No high exposure accounts</TableCell></TableRow>
                )}
                {data.top_high_exposure.map(item => (
                  <TableRow key={item.entity_name}>
                    <TableCell className="font-medium">{item.entity_name}</TableCell>
                    <TableCell className="text-right">{formatCurrency(item.amount_due || 0)}</TableCell>
                    <TableCell className="text-right">
                      <Badge variant={item.risk_score && item.risk_score > 50 ? "destructive" : "secondary"}>
                        {item.risk_score}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function ExposureSkeleton() {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-3 gap-4">
        <Skeleton className="h-[100px]" />
        <Skeleton className="h-[100px]" />
        <Skeleton className="h-[100px]" />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Skeleton className="h-[300px]" />
        <Skeleton className="h-[300px]" />
      </div>
    </div>
  )
}
