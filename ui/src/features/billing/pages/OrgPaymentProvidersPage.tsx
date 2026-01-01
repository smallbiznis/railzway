import { useCallback, useEffect, useMemo, useState } from "react"
import { useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { Alert, AlertDescription } from "@/components/ui/alert"
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

type CatalogProvider = {
  provider: string
  display_name: string
  description?: string | null
  supports_webhook: boolean
  supports_refund: boolean
}

type ProviderConfig = {
  provider: string
  is_active: boolean
  configured: boolean
}

type ProviderField = {
  key: string
  label: string
  placeholder?: string
  helper?: string
  type?: "text" | "password" | "textarea"
  optional?: boolean
}

const providerFields: Record<string, ProviderField[]> = {
  stripe: [
    {
      key: "api_key",
      label: "API key",
      placeholder: "sk_live_...",
      type: "password",
      helper: "Use a restricted key with billing-only permissions.",
    },
    {
      key: "webhook_secret",
      label: "Webhook secret",
      placeholder: "whsec_...",
      type: "password",
      helper: "Used to validate settlement webhooks.",
    },
  ],
  midtrans: [
    {
      key: "server_key",
      label: "Server key",
      placeholder: "Midtrans server key",
      type: "password",
    },
    {
      key: "client_key",
      label: "Client key",
      placeholder: "Midtrans client key",
      type: "password",
    },
  ],
  xendit: [
    {
      key: "secret_key",
      label: "Secret key",
      placeholder: "xnd_...",
      type: "password",
    },
    {
      key: "callback_token",
      label: "Callback token",
      placeholder: "Webhook callback token",
      type: "password",
    },
  ],
  manual: [
    {
      key: "display_label",
      label: "Display label",
      placeholder: "Manual transfer",
      type: "text",
      optional: true,
    },
    {
      key: "instructions",
      label: "Settlement instructions",
      placeholder: "Bank account or cash collection notes",
      type: "textarea",
      optional: true,
    },
  ],
}

const statusBadge = (config?: ProviderConfig) => {
  if (!config?.configured) {
    return <Badge variant="secondary">Not configured</Badge>
  }
  if (config.is_active) {
    return <Badge>Active</Badge>
  }
  return <Badge variant="outline">Inactive</Badge>
}

export default function OrgPaymentProvidersPage() {
  const { orgId } = useParams()
  const [catalog, setCatalog] = useState<CatalogProvider[]>([])
  const [configs, setConfigs] = useState<ProviderConfig[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  const [activeProvider, setActiveProvider] = useState<CatalogProvider | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [rawConfig, setRawConfig] = useState("")
  const [isSaving, setIsSaving] = useState(false)
  const [toggleProvider, setToggleProvider] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setLoadError(null)

    try {
      const [catalogRes, configRes] = await Promise.all([
        admin.get<{ providers: CatalogProvider[] }>("/payment-providers/catalog"),
        admin.get<{ configs: ProviderConfig[] }>("/payment-providers"),
      ])

      setCatalog(Array.isArray(catalogRes.data?.providers) ? catalogRes.data.providers : [])
      setConfigs(Array.isArray(configRes.data?.configs) ? configRes.data.configs : [])
    } catch (err: any) {
      setLoadError(err?.message ?? "Unable to load payment providers.")
      setCatalog([])
      setConfigs([])
    } finally {
      setIsLoading(false)
    }
  }, [orgId])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const configMap = useMemo(() => {
    const map = new Map<string, ProviderConfig>()
    configs.forEach((config) => {
      map.set(config.provider, config)
    })
    return map
  }, [configs])

  const rows = useMemo(() => {
    return catalog.map((provider) => ({
      provider,
      config: configMap.get(provider.provider),
    }))
  }, [catalog, configMap])

  const resetDialog = () => {
    setActiveProvider(null)
    setFormValues({})
    setFieldErrors({})
    setFormError(null)
    setRawConfig("")
    setIsSaving(false)
  }

  const openDialog = (provider: CatalogProvider) => {
    setActiveProvider(provider)
    setFormValues({})
    setFieldErrors({})
    setFormError(null)
    setRawConfig("")
    setIsDialogOpen(true)
  }

  const handleDialogChange = (open: boolean) => {
    setIsDialogOpen(open)
    if (!open) {
      resetDialog()
    }
  }

  const updateField = (key: string, value: string) => {
    setFormValues((prev) => ({ ...prev, [key]: value }))
    setFieldErrors((prev) => {
      if (!prev[key]) return prev
      const { [key]: _, ...rest } = prev
      return rest
    })
    if (formError) {
      setFormError(null)
    }
  }

  const buildConfigPayload = () => {
    if (!activeProvider) return null
    const fields = providerFields[activeProvider.provider]

    if (!fields || fields.length === 0) {
      const trimmed = rawConfig.trim()
      if (!trimmed) {
        setFormError("Configuration JSON is required.")
        return null
      }
      try {
        const parsed = JSON.parse(trimmed)
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
          setFormError("Configuration JSON must be an object.")
          return null
        }
        return parsed as Record<string, unknown>
      } catch {
        setFormError("Configuration JSON is invalid.")
        return null
      }
    }

    const nextErrors: Record<string, string> = {}
    const payload: Record<string, string> = {}

    fields.forEach((field) => {
      const raw = formValues[field.key] ?? ""
      const trimmed = raw.trim()
      if (!field.optional && !trimmed) {
        nextErrors[field.key] = "Required"
        return
      }
      if (trimmed) {
        payload[field.key] = trimmed
      }
    })

    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors)
      setFormError("Please fill in the required fields.")
      return null
    }

    if (Object.keys(payload).length === 0) {
      setFormError("At least one credential is required.")
      return null
    }

    return payload
  }

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!activeProvider) return

    const payload = buildConfigPayload()
    if (!payload) return

    setIsSaving(true)
    setFormError(null)

    try {
      await admin.post("/payment-providers", {
        provider: activeProvider.provider,
        config: payload,
      })
      setIsDialogOpen(false)
      resetDialog()
      await loadData()
    } catch (err: any) {
      setFormError(err?.message ?? "Unable to save provider configuration.")
    } finally {
      setIsSaving(false)
    }
  }

  const handleToggle = async (provider: string, nextValue: boolean) => {
    setToggleProvider(provider)
    try {
      await admin.patch(`/payment-providers/${provider}`, { is_active: nextValue })
      await loadData()
    } catch (err: any) {
      setLoadError(err?.message ?? "Unable to update provider status.")
    } finally {
      setToggleProvider(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Payment providers</h1>
        <p className="text-sm text-text-muted">
          Configure settlement credentials per provider. Valora never processes payments or stores
          raw secrets.
        </p>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription>{loadError}</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Provider catalog</CardTitle>
          <CardDescription>
            Enable the providers your organization will use for settlement webhooks.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {isLoading && <div className="text-sm text-text-muted">Loading providers...</div>}
          {!isLoading && rows.length === 0 && (
            <div className="text-sm text-text-muted">No providers available.</div>
          )}
          {!isLoading && rows.length > 0 && (
            <div className="overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Provider</TableHead>
                    <TableHead>Capabilities</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map(({ provider, config }) => (
                    <TableRow key={provider.provider}>
                      <TableCell className="space-y-1">
                        <div className="font-medium">{provider.display_name}</div>
                        <div className="text-sm text-text-muted">
                          {provider.description ?? "No description provided."}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          <Badge variant={provider.supports_webhook ? "default" : "secondary"}>
                            Webhooks {provider.supports_webhook ? "on" : "off"}
                          </Badge>
                          <Badge variant={provider.supports_refund ? "default" : "secondary"}>
                            Refunds {provider.supports_refund ? "on" : "off"}
                          </Badge>
                        </div>
                      </TableCell>
                      <TableCell>{statusBadge(config)}</TableCell>
                      <TableCell className="text-right">
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => openDialog(provider)}
                          >
                            {config?.configured ? "Update" : "Configure"}
                          </Button>
                          <div className="flex items-center gap-2">
                            <Switch
                              checked={config?.configured ? config.is_active : false}
                              disabled={!config?.configured || toggleProvider === provider.provider}
                              onCheckedChange={(value) =>
                                handleToggle(provider.provider, Boolean(value))
                              }
                            />
                            <span className="text-xs text-text-muted">Active</span>
                          </div>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {activeProvider ? `Configure ${activeProvider.display_name}` : "Configure provider"}
            </DialogTitle>
            <DialogDescription>
              Credentials are encrypted and never displayed again after saving.
            </DialogDescription>
          </DialogHeader>
          <form className="space-y-4" onSubmit={handleSave}>
            {formError && (
              <Alert variant="destructive">
                <AlertDescription>{formError}</AlertDescription>
              </Alert>
            )}

            {activeProvider && providerFields[activeProvider.provider]?.length ? (
              <div className="space-y-4">
                {providerFields[activeProvider.provider].map((field) => {
                  const error = fieldErrors[field.key]
                  const value = formValues[field.key] ?? ""
                  const inputId = `provider-${field.key}`

                  return (
                    <div key={field.key} className="space-y-2">
                      <Label htmlFor={inputId}>
                        {field.label}
                        {field.optional ? " (optional)" : ""}
                      </Label>
                      {field.type === "textarea" ? (
                        <Textarea
                          id={inputId}
                          value={value}
                          placeholder={field.placeholder}
                          onChange={(event) => updateField(field.key, event.target.value)}
                        />
                      ) : (
                        <Input
                          id={inputId}
                          type={field.type ?? "text"}
                          value={value}
                          placeholder={field.placeholder}
                          onChange={(event) => updateField(field.key, event.target.value)}
                        />
                      )}
                      {error && <p className="text-xs text-status-error">{error}</p>}
                      {field.helper && !error && (
                        <p className="text-xs text-text-muted">{field.helper}</p>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="raw-config">Configuration JSON</Label>
                <Textarea
                  id="raw-config"
                  placeholder='{"api_key":"..."}'
                  value={rawConfig}
                  onChange={(event) => {
                    setRawConfig(event.target.value)
                    if (formError) setFormError(null)
                  }}
                />
                <p className="text-xs text-text-muted">
                  Provide a JSON object with provider credentials.
                </p>
              </div>
            )}

            <DialogFooter className="gap-2 sm:justify-end">
              <Button type="button" variant="ghost" onClick={() => setIsDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save configuration"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
