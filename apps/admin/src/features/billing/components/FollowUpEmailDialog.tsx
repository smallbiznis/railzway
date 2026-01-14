import { useState } from "react"
import { Mail } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"

interface FollowUpEmailDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  customerName: string
  customerEmail: string
  invoiceNumber: string
  amount: string
  daysOverdue: number
  orgName?: string
  followUpCount?: number
  onEmailOpened: (provider: string) => void
}

const DEFAULT_TEMPLATE = {
  subject: "Reminder: Invoice {{invoice_number}} Payment Due",
  body: `Dear {{customer_name}},

This is a friendly reminder that invoice {{invoice_number}} for {{amount}} is currently overdue by {{days_overdue}} days.

Please process payment at your earliest convenience.

Best regards,
{{org_name}}`
}

export function FollowUpEmailDialog({
  open,
  onOpenChange,
  customerName,
  customerEmail,
  invoiceNumber,
  amount,
  daysOverdue,
  orgName = "Our Team",
  followUpCount = 0,
  onEmailOpened,
}: FollowUpEmailDialogProps) {
  const [emailProvider, setEmailProvider] = useState<string>("default")
  const [subject, setSubject] = useState(DEFAULT_TEMPLATE.subject)
  const [body, setBody] = useState(DEFAULT_TEMPLATE.body)

  // Replace template variables
  const replaceVariables = (text: string) => {
    return text
      .replace(/\{\{customer_name\}\}/g, customerName)
      .replace(/\{\{invoice_number\}\}/g, invoiceNumber)
      .replace(/\{\{amount\}\}/g, amount)
      .replace(/\{\{days_overdue\}\}/g, String(daysOverdue))
      .replace(/\{\{org_name\}\}/g, orgName)
  }

  const handleOpenEmail = () => {
    const finalSubject = replaceVariables(subject)
    const finalBody = replaceVariables(body)

    // Encode for URL
    const encodedSubject = encodeURIComponent(finalSubject)
    const encodedBody = encodeURIComponent(finalBody)
    const encodedTo = encodeURIComponent(customerEmail)

    let url = ""

    if (emailProvider === "gmail") {
      // Gmail web compose
      url = `https://mail.google.com/mail/?view=cm&to=${encodedTo}&su=${encodedSubject}&body=${encodedBody}`
      window.open(url, "_blank")
    } else if (emailProvider === "outlook") {
      // Outlook web compose
      url = `https://outlook.office.com/mail/deeplink/compose?to=${encodedTo}&subject=${encodedSubject}&body=${encodedBody}`
      window.open(url, "_blank")
    } else {
      // Default mailto
      url = `mailto:${customerEmail}?subject=${encodedSubject}&body=${encodedBody}`
      window.location.href = url
    }

    // Notify parent that email was opened
    onEmailOpened(emailProvider)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Mail className="h-5 w-5" />
            Send Follow-Up Email
            {followUpCount > 0 && (
              <Badge variant="outline" className="ml-2">
                {followUpCount} sent
              </Badge>
            )}
          </DialogTitle>
          <DialogDescription>
            Compose a follow-up email for {customerName}. The email will open in your chosen email client.
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto pr-2">
          <div className="space-y-4">
            {/* Email Provider Selection */}
            <div className="space-y-2">
              <Label htmlFor="email-provider">Email Provider</Label>
              <Select value={emailProvider} onValueChange={setEmailProvider}>
                <SelectTrigger id="email-provider">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default Email Client</SelectItem>
                  <SelectItem value="gmail">Gmail (Web)</SelectItem>
                  <SelectItem value="outlook">Outlook (Web)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Customer Info */}
            <div className="rounded-lg border p-3 text-sm">
              <div className="font-medium">To: {customerEmail}</div>
              <div className="text-text-muted">
                Invoice: {invoiceNumber} • Amount: {amount} • {daysOverdue} days overdue
              </div>
            </div>

            {/* Subject */}
            <div className="space-y-2">
              <Label htmlFor="subject">Subject</Label>
              <Input
                id="subject"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                placeholder="Email subject"
              />
            </div>

            {/* Body */}
            <div className="space-y-2">
              <Label htmlFor="body">Message</Label>
              <Textarea
                id="body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={10}
                placeholder="Email message"
              />
              <p className="text-xs text-text-muted">
                Available variables: {"{"}{"{"} customer_name {"}"}{"}"},  {"{"}{"{"} invoice_number {"}"}{"}"},  {"{"}{"{"} amount {"}"}{"}"},  {"{"}{"{"} days_overdue {"}"}{"}"},  {"{"}{"{"} org_name {"}"}{"}"}
              </p>
            </div>

            {/* Preview */}
            <div className="space-y-2">
              <Label>Preview</Label>
              <div className="rounded-lg border bg-bg-subtle p-3 text-sm">
                <div className="font-medium mb-2">Subject: {replaceVariables(subject)}</div>
                <div className="whitespace-pre-wrap text-text-muted">
                  {replaceVariables(body)}
                </div>
              </div>
            </div>

            {followUpCount >= 3 && (
              <div className="rounded-lg border border-status-warning bg-status-warning/10 p-3 text-sm text-status-warning">
                ⚠️ You've sent {followUpCount} follow-up emails. Consider escalating this task.
              </div>
            )}
          </div>
        </div>

        <DialogFooter className="mt-2">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleOpenEmail}>
            <Mail className="h-4 w-4 mr-2" />
            Open Email Client
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
