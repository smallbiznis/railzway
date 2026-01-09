import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { IconPlus } from "@tabler/icons-react"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

const formatDate = (value?: string) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  }).format(date)
}

type InvoiceTemplate = {
  id: string
  name: string
  is_default: boolean
  locale: string
  currency: string
  header?: Record<string, unknown>
  footer?: Record<string, unknown>
  style?: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

export default function OrgInvoiceTemplatesPage() {
  const { orgId } = useParams()
  const [searchQuery, setSearchQuery] = useState("")
  const [templates, setTemplates] = useState<InvoiceTemplate[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)
  const [pendingDefault, setPendingDefault] = useState<InvoiceTemplate | null>(null)
  const [isSettingDefault, setIsSettingDefault] = useState(false)
  const [setDefaultError, setSetDefaultError] = useState<string | null>(null)
  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"

  useEffect(() => {
    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get("/invoice-templates")
      .then((response) => {
        if (!active) return
        setTemplates(response.data?.data ?? [])
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load templates."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [])

  const filteredTemplates = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    if (!query) return templates
    return templates.filter((template) => {
      const footer = String(template.footer?.notes ?? "").toLowerCase()
      const header = String(template.header?.company_name ?? "").toLowerCase()
      return (
        template.name.toLowerCase().includes(query) ||
        header.includes(query) ||
        footer.includes(query)
      )
    })
  }, [searchQuery, templates])

  const countLabel = useMemo(() => {
    const total = filteredTemplates.length
    return `${total} template${total === 1 ? "" : "s"}`
  }, [filteredTemplates.length])

  const defaultTemplate = useMemo(
    () => templates.find((template) => template.is_default),
    [templates]
  )

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading templates...</div>
  }

  if (error) {
    return <div className="text-status-error text-sm">{error}</div>
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to invoice templates." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Invoice templates</h1>
          <p className="text-text-muted text-sm">
            Set reusable layouts for invoices, notes, and line item grouping.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {orgId && (
            <Button asChild size="sm">
              <Link to={`${orgBasePath}/invoice-templates/create`}>
                <IconPlus />
                Create template
              </Link>
            </Button>
          )}
          <Button asChild variant="outline" size="sm">
            <Link to={`${orgBasePath}/invoices`}>View invoices</Link>
          </Button>
        </div>
      </div>

      {defaultTemplate && (
        <div className="rounded-lg border bg-bg-subtle/30 p-4 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">Default template</Badge>
            <span className="font-medium">{defaultTemplate.name}</span>
          </div>
          <div className="text-text-muted mt-1">
            Used for new subscriptions and invoices unless another template is selected.
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Input
          className="w-full max-w-md"
          placeholder="Search templates"
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
        <div className="text-text-muted text-sm">{countLabel}</div>
      </div>

      {filteredTemplates.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No templates yet</EmptyTitle>
            <EmptyDescription>
              Create an invoice template to standardize branding and layout.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            {orgId && (
              <Button asChild>
                <Link to={`${orgBasePath}/invoice-templates/create`}>
                  Create template
                </Link>
              </Button>
            )}
          </EmptyContent>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow>
                <TableHead>Template</TableHead>
                <TableHead>Locale</TableHead>
                <TableHead>Currency</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredTemplates.map((template) => (
                <TableRow key={template.id}>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{template.name}</span>
                        {template.is_default && (
                          <Badge variant="secondary">Default</Badge>
                        )}
                      </div>
                      <span className="text-text-muted text-sm">
                        {template.header?.company_name ? String(template.header.company_name) : ""}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>{template.locale?.toUpperCase()}</TableCell>
                  <TableCell>{template.currency?.toUpperCase()}</TableCell>
                  <TableCell>{formatDate(template.updated_at)}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-2">
                      {orgId ? (
                        <Button asChild variant="ghost" size="sm">
                          <Link to={`${orgBasePath}/invoice-templates/${template.id}`}>
                            Edit
                          </Link>
                        </Button>
                      ) : (
                        <Button variant="ghost" size="sm" disabled>
                          Edit
                        </Button>
                      )}
                      {!template.is_default && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setPendingDefault(template)
                            setSetDefaultError(null)
                          }}
                        >
                          Set default
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <AlertDialog
        open={Boolean(pendingDefault)}
        onOpenChange={(open) => {
          if (!open) {
            setPendingDefault(null)
            setSetDefaultError(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Set default template</AlertDialogTitle>
            <AlertDialogDescription>
              This will make the selected template the default for future invoices.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {setDefaultError && <div className="text-status-error text-sm">{setDefaultError}</div>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isSettingDefault}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={isSettingDefault}
              onClick={async () => {
                if (!pendingDefault) return
                setIsSettingDefault(true)
                setSetDefaultError(null)
                try {
                  await admin.post(`/invoice-templates/${pendingDefault.id}/set-default`)
                  const response = await admin.get("/invoice-templates")
                  setTemplates(response.data?.data ?? [])
                  setPendingDefault(null)
                } catch (err) {
                  setSetDefaultError(getErrorMessage(err, "Unable to set default template."))
                } finally {
                  setIsSettingDefault(false)
                }
              }}
            >
              {isSettingDefault ? "Setting..." : "Confirm"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
