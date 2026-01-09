import { useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Textarea } from "@/components/ui/textarea"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"

const localeOptions = ["en", "en-GB", "id-ID"]
const currencyOptions = ["USD", "EUR", "GBP", "IDR"]

type InvoiceTemplate = {
  id: string
  name: string
  is_default: boolean
  locale: string
  currency: string
  header?: Record<string, unknown>
  footer?: Record<string, unknown>
  style?: Record<string, unknown>
}

export default function OrgInvoiceTemplateFormPage() {
  const { orgId, templateId } = useParams()
  const navigate = useNavigate()
  const [template, setTemplate] = useState<InvoiceTemplate | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isForbidden, setIsForbidden] = useState(false)

  const [name, setName] = useState("")
  const [locale, setLocale] = useState("en")
  const [currency, setCurrency] = useState("USD")
  const [companyName, setCompanyName] = useState("")
  const [logoUrl, setLogoUrl] = useState("")
  const [footerNotes, setFooterNotes] = useState("")
  const [footerLegal, setFooterLegal] = useState("")
  const [primaryColor, setPrimaryColor] = useState("#111827")
  const [fontFamily, setFontFamily] = useState("Inter")
  const [makeDefault, setMakeDefault] = useState(false)

  const [isSaving, setIsSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [isSettingDefault, setIsSettingDefault] = useState(false)
  const [setDefaultError, setSetDefaultError] = useState<string | null>(null)
  const [isDefaultDialogOpen, setIsDefaultDialogOpen] = useState(false)

  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"
  const isEditing = Boolean(templateId)

  useEffect(() => {
    if (!templateId) {
      setIsLoading(false)
      return
    }
    let active = true
    setIsLoading(true)
    setError(null)
    setIsForbidden(false)

    admin
      .get(`/invoice-templates/${templateId}`)
      .then((response) => {
        if (!active) return
        const data = response.data?.data ?? null
        setTemplate(data)
        setName(data?.name ?? "")
        setLocale(data?.locale ?? "en")
        setCurrency(data?.currency ?? "USD")
        setCompanyName(String(data?.header?.company_name ?? ""))
        setLogoUrl(String(data?.header?.logo_url ?? ""))
        setFooterNotes(String(data?.footer?.notes ?? ""))
        setFooterLegal(String(data?.footer?.legal ?? ""))
        setPrimaryColor(String(data?.style?.primary_color ?? "#111827"))
        setFontFamily(String(data?.style?.font ?? "Inter"))
        setMakeDefault(Boolean(data?.is_default))
      })
      .catch((err) => {
        if (!active) return
        if (isForbiddenError(err)) {
          setIsForbidden(true)
          return
        }
        setError(getErrorMessage(err, "Unable to load template."))
      })
      .finally(() => {
        if (!active) return
        setIsLoading(false)
      })

    return () => {
      active = false
    }
  }, [templateId])

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault()
    setSaveError(null)
    setIsSaving(true)

    const payload = {
      name: name.trim(),
      locale,
      currency,
      header: {
        company_name: companyName.trim(),
        logo_url: logoUrl.trim(),
      },
      footer: {
        notes: footerNotes.trim(),
        legal: footerLegal.trim(),
      },
      style: {
        primary_color: primaryColor.trim(),
        font: fontFamily.trim(),
      },
    }

    try {
      if (isEditing && templateId) {
        await admin.patch(`/invoice-templates/${templateId}`, payload)
        navigate(`${orgBasePath}/invoice-templates/${templateId}`)
      } else {
        const res = await admin.post("/invoice-templates", {
          ...payload,
          is_default: makeDefault,
        })
        const newId = res.data?.data?.id
        if (newId) {
          navigate(`${orgBasePath}/invoice-templates/${newId}`)
        } else {
          navigate(`${orgBasePath}/invoice-templates`)
        }
      }
    } catch (err) {
      setSaveError(getErrorMessage(err, "Unable to save template."))
    } finally {
      setIsSaving(false)
    }
  }

  if (isLoading) {
    return <div className="text-text-muted text-sm">Loading template...</div>
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
          <h1 className="text-2xl font-semibold">
            {isEditing ? "Update template" : "Create template"}
          </h1>
          <p className="text-text-muted text-sm">
            Configure branding, layout, and localization for invoices.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="outline" size="sm">
            <Link to={`${orgBasePath}/invoice-templates`}>Back to templates</Link>
          </Button>
          {template?.is_default && <Badge variant="secondary">Default</Badge>}
          {isEditing && template && !template.is_default && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setIsDefaultDialogOpen(true)
                setSetDefaultError(null)
              }}
            >
              Set default
            </Button>
          )}
        </div>
      </div>

      <form className="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]" onSubmit={handleSave}>
        <Card>
          <CardContent className="space-y-8 pt-6">
            {saveError && <div className="text-status-error text-sm">{saveError}</div>}
            <section className="space-y-3">
              <div>
                <h2 className="text-base font-semibold">Template name</h2>
                <p className="text-text-muted text-sm">Visible to your team only.</p>
              </div>
              <Input
                placeholder="Enter name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </section>

            <Separator />

            <section className="space-y-4">
              <div>
                <h2 className="text-base font-semibold">Localization</h2>
                <p className="text-text-muted text-sm">
                  Locale and currency used when rendering invoices.
                </p>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="template-locale">Locale</Label>
                  <Select value={locale} onValueChange={setLocale}>
                    <SelectTrigger id="template-locale">
                      <SelectValue placeholder="Select locale" />
                    </SelectTrigger>
                    <SelectContent>
                      {localeOptions.map((option) => (
                        <SelectItem key={option} value={option}>
                          {option}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="template-currency">Currency</Label>
                  <Select value={currency} onValueChange={setCurrency}>
                    <SelectTrigger id="template-currency">
                      <SelectValue placeholder="Select currency" />
                    </SelectTrigger>
                    <SelectContent>
                      {currencyOptions.map((option) => (
                        <SelectItem key={option} value={option}>
                          {option}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </section>

            <Separator />

            <section className="space-y-4">
              <div>
                <h2 className="text-base font-semibold">Header</h2>
                <p className="text-text-muted text-sm">Branding displayed on invoices.</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="template-company">Company name</Label>
                <Input
                  id="template-company"
                  placeholder="Company name"
                  value={companyName}
                  onChange={(event) => setCompanyName(event.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="template-logo">Logo URL</Label>
                <Input
                  id="template-logo"
                  placeholder="https://..."
                  value={logoUrl}
                  onChange={(event) => setLogoUrl(event.target.value)}
                />
              </div>
            </section>

            <Separator />

            <section className="space-y-4">
              <div>
                <h2 className="text-base font-semibold">Footer</h2>
                <p className="text-text-muted text-sm">Notes and legal disclosures.</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="template-notes">Footer notes</Label>
                <Textarea
                  id="template-notes"
                  placeholder="Thanks for your business"
                  value={footerNotes}
                  onChange={(event) => setFooterNotes(event.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="template-legal">Legal footer</Label>
                <Textarea
                  id="template-legal"
                  placeholder="Legal terms, VAT info, etc."
                  value={footerLegal}
                  onChange={(event) => setFooterLegal(event.target.value)}
                />
              </div>
            </section>

            <Separator />

            <section className="space-y-4">
              <div>
                <h2 className="text-base font-semibold">Style</h2>
                <p className="text-text-muted text-sm">Visual styling for invoice renders.</p>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="template-color">Primary color</Label>
                  <Input
                    id="template-color"
                    placeholder="#111827"
                    value={primaryColor}
                    onChange={(event) => setPrimaryColor(event.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="template-font">Font family</Label>
                  <Input
                    id="template-font"
                    placeholder="Inter"
                    value={fontFamily}
                    onChange={(event) => setFontFamily(event.target.value)}
                  />
                </div>
              </div>
            </section>

            {!isEditing && (
              <section className="space-y-3">
                <Label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={makeDefault}
                    onChange={(event) => setMakeDefault(event.target.checked)}
                  />
                  Set as default template
                </Label>
                <p className="text-text-muted text-xs">
                  The default template is used for new invoices unless overridden.
                </p>
              </section>
            )}

            <div className="flex flex-wrap items-center gap-3">
              <Button type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : isEditing ? "Save changes" : "Create template"}
              </Button>
              <Button variant="outline" asChild>
                <Link to={`${orgBasePath}/invoice-templates`}>Cancel</Link>
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="space-y-4 pt-6 text-sm">
            <h2 className="text-base font-semibold">Preview</h2>
            <p className="text-text-muted">
              Preview renders on invoices are generated via the invoice render API.
            </p>
            <p className="text-text-muted">
              Save changes to make them available for the next invoice render.
            </p>
          </CardContent>
        </Card>
      </form>

      <AlertDialog
        open={isDefaultDialogOpen}
        onOpenChange={(open) => {
          setIsDefaultDialogOpen(open)
          if (!open) {
            setSetDefaultError(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Set default template</AlertDialogTitle>
            <AlertDialogDescription>
              This will make the current template the default for future invoices.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {setDefaultError && <div className="text-status-error text-sm">{setDefaultError}</div>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isSettingDefault}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={isSettingDefault}
              onClick={async () => {
                if (!templateId) return
                setIsSettingDefault(true)
                setSetDefaultError(null)
                try {
                  await admin.post(`/invoice-templates/${templateId}/set-default`)
                  setTemplate((prev) => (prev ? { ...prev, is_default: true } : prev))
                  setIsDefaultDialogOpen(false)
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
