import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"
import { admin } from "@/api/client"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"

type PaymentDetail = {
  payment_id: string
  amount: number
  currency: string
  occurred_at: string
  provider: string
  method: string
  card_brand?: string
  card_last4?: string
  status: string
}

export function InvoicePaymentsTable() {
  const { invoiceId } = useParams()
  const [payments, setPayments] = useState<PaymentDetail[]>([])
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!invoiceId) return

    setIsLoading(true)
    admin
      .get(`/billing-operations/invoices/${invoiceId}/payments`)
      .then((res) => {
        setPayments(res.data?.payments ?? [])
      })
      .catch((err) => {
        console.error("Failed to load payments", err)
      })
      .finally(() => {
        setIsLoading(false)
      })
  }, [invoiceId])

  if (isLoading) {
    return <div className="p-6 text-center text-sm text-muted-foreground">Loading payments...</div>
  }

  if (payments.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        No payments found for this invoice.
      </div>
    )
  }

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Date</TableHead>
            <TableHead>Amount</TableHead>
            <TableHead>Method</TableHead>
            <TableHead>Provider</TableHead>
            <TableHead>Status</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {payments.map((payment) => (
            <TableRow key={payment.payment_id}>
              <TableCell className="font-medium">
                {new Date(payment.occurred_at).toLocaleString()}
              </TableCell>
              <TableCell>
                {new Intl.NumberFormat("en-US", {
                  style: "currency",
                  currency: payment.currency.toUpperCase(),
                }).format(payment.amount / 100)}
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  {payment.card_brand && <span className="capitalize">{payment.card_brand}</span>}
                  {payment.card_last4 && <span className="font-mono text-xs">•••• {payment.card_last4}</span>}
                  {!payment.card_brand && !payment.card_last4 && <span className="capitalize">{payment.method || "-"}</span>}
                </div>
              </TableCell>
              <TableCell className="capitalize">{payment.provider}</TableCell>
              <TableCell>
                <Badge variant={payment.status === "succeeded" ? "default" : "destructive"}>
                  {payment.status}
                </Badge>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
