import { useCallback, useEffect, useMemo, useState } from "react"
import {
  IconCopy,
  IconEye,
  IconPlus,
  IconRefresh,
  IconTrash,
  IconChevronRight
} from "@tabler/icons-react"
import { useParams } from "react-router-dom"
import { cn } from "@/lib/utils"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { getErrorMessage, isForbiddenError } from "@/lib/api-errors"
import { canManageBilling, isOrgOwner } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    hour: "numeric",
    minute: "2-digit",
  }).format(date)
}

type ApiKey = {
  key_id: string
  name: string
  scopes?: string[]
  is_active: boolean
  created_at: string
  last_used_at?: string | null
  expires_at?: string | null
  rotated_from_key_id?: string | null
}

type ApiKeySecretResponse = {
  key_id: string
  api_key: string
}

const maskKeyID = (keyId: string) => {
  if (!keyId) return "-"
  const suffix = keyId.slice(-6)
  return `****${suffix}`
}

const statusLabel = (key: ApiKey) => {
  if (!key.is_active) return "Revoked"
  if (key.expires_at && new Date(key.expires_at).getTime() <= Date.now()) {
    return "Expired"
  }
  return "Active"
}

const statusVariant = (key: ApiKey) => {
  const status = statusLabel(key)
  if (status === "Revoked") return "destructive"
  if (status === "Expired") return "secondary"
  return "default"
}

const formatScopes = (key: ApiKey) => {
  if (Array.isArray(key.scopes) && key.scopes.length > 0) {
    if (key.scopes.includes("*") || key.scopes.some(s => s.endsWith(":*"))) {
      // Basic heuristic for summary
      const wildcards = key.scopes.filter(s => s.endsWith(":*"))
      if (wildcards.length > 0) return `${wildcards.length} full access roles, ${key.scopes.length - wildcards.length} specific`
      return "Full Access"
    }
    return key.scopes.join(", ")
  }
  return "-"
}

// ----------------------------------------------------------------------
// ScopeSelector Component
// ----------------------------------------------------------------------

interface ScopeSelectorProps {
  availableScopes: string[]
  value: string[]
  onChange: (scopes: string[]) => void
}

function ScopeSelector({ availableScopes, value, onChange }: ScopeSelectorProps) {
  const [open, setOpen] = useState(false)
  const [activeCategory, setActiveCategory] = useState<string | null>(null)

  // Group scopes by prefix
  const groups = useMemo(() => {
    const g: Record<string, string[]> = {}
    availableScopes.forEach((s) => {
      const prefix = s.split(":")[0]
      if (!g[prefix]) g[prefix] = []
      g[prefix].push(s)
    })
    return g
  }, [availableScopes])

  const categories = useMemo(() => Object.keys(groups).sort(), [groups])

  // Select the first category by default if none selected
  useEffect(() => {
    if (open && !activeCategory && categories.length > 0) {
      setActiveCategory(categories[0])
    }
  }, [open, activeCategory, categories])

  // Helpers to check selection state
  const isWildcardSelected = (category: string) => value.includes(`${category}:*`)

  const isCategoryFullySelected = (category: string) => {
    if (isWildcardSelected(category)) return true
    const categoryScopes = groups[category] || []
    return categoryScopes.every((s) => value.includes(s))
  }

  const isCategoryPartiallySelected = (category: string) => {
    if (isCategoryFullySelected(category)) return false
    const categoryScopes = groups[category] || []
    return categoryScopes.some((s) => value.includes(s))
  }

  const toggleWildcard = (category: string, checked: boolean) => {
    if (checked) {
      // Select wildcard, remove specific scopes of this group (clean up)
      const otherScopes = value.filter(s => !s.startsWith(`${category}:`))
      onChange([...otherScopes, `${category}:*`])
    } else {
      // Unselect wildcard
      onChange(value.filter(s => s !== `${category}:*`))
    }
  }

  const toggleScope = (scope: string, checked: boolean) => {
    if (checked) {
      onChange([...value, scope])
    } else {
      onChange(value.filter(s => s !== scope))
    }
  }

  // Derived display text
  const selectedCount = value.length
  const triggerLabel = selectedCount > 0
    ? `${selectedCount} permission${selectedCount === 1 ? '' : 's'} selected`
    : "Select permissions"

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between"
        >
          {triggerLabel}
          <IconPlus className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[600px] p-0" align="start" side="bottom" collisionPadding={10}>
        <div className="flex h-[350px]">
          {/* Left Pane: Categories */}
          <div className="w-[200px] border-r flex flex-col">
            <Command className="h-full">
              <CommandInput placeholder="Search roles..." />
              <CommandList className="max-h-full overflow-y-auto">
                <CommandEmpty>No roles found.</CommandEmpty>
                <CommandGroup heading="Fixed roles">
                  {categories.map((category) => {
                    const fully = isCategoryFullySelected(category)
                    const partial = isCategoryPartiallySelected(category)

                    return (
                      <CommandItem
                        key={category}
                        onSelect={() => setActiveCategory(category)}
                        className={cn(
                          "cursor-pointer flex items-center justify-between aria-selected:bg-accent aria-selected:text-accent-foreground",
                          activeCategory === category && "bg-accent text-accent-foreground"
                        )}
                      >
                        <div className="flex items-center gap-2 overflow-hidden">
                          <Checkbox
                            checked={fully || (partial ? "indeterminate" : false)}
                            className="shrink-0"
                            onClick={(e) => {
                              e.stopPropagation()
                              if (fully) {
                                // Deselect all in category
                                // Handle wildcard removal
                                const newVal = value.filter(s => !s.startsWith(category))
                                onChange(newVal)
                              } else {
                                // Select Wildcard (Full Access)
                                toggleWildcard(category, true)
                              }
                            }}
                          />
                          <span className="truncate capitalize">{category.replace(/_/g, ' ')}</span>
                        </div>
                        {activeCategory === category && <IconChevronRight className="h-4 w-4 shrink-0 opacity-50" />}
                      </CommandItem>
                    )
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </div>

          {/* Right Pane: Permissions */}
          <div className="flex-1 flex flex-col bg-background">
            {activeCategory ? (
              <>
                <div className="p-3 border-b bg-muted/20">
                  <h4 className="font-semibold capitalize text-sm">{activeCategory.replace(/_/g, ' ')} Permissions</h4>
                </div>
                <ScrollArea className="flex-1 p-3">
                  <div className="space-y-3">
                    {/* Wildcard Option */}
                    <div className="flex items-start gap-2 p-2 hover:bg-muted/50 rounded-md">
                      <Checkbox
                        id={`${activeCategory}-wildcard`}
                        checked={isWildcardSelected(activeCategory)}
                        onCheckedChange={(checked) => toggleWildcard(activeCategory, checked as boolean)}
                      />
                      <div className="grid gap-1 leading-none">
                        <label
                          htmlFor={`${activeCategory}-wildcard`}
                          className="text-sm font-medium leading-none cursor-pointer"
                        >
                          Full {activeCategory.replace(/_/g, ' ')} Access
                        </label>
                        <p className="text-xs text-muted-foreground">
                          Grants all current and future permissions for this resource.
                        </p>
                      </div>
                    </div>

                    {/* Individual Scopes */}
                    <div className="space-y-2 pl-6 border-l ml-2">
                      {groups[activeCategory]?.map(scope => {
                        const disabled = isWildcardSelected(activeCategory)
                        const checked = disabled || value.includes(scope)

                        return (
                          <div key={scope} className="flex items-center gap-2">
                            <Checkbox
                              id={scope}
                              checked={checked}
                              disabled={disabled}
                              onCheckedChange={(checked) => toggleScope(scope, checked as boolean)}
                            />
                            <label htmlFor={scope} className="text-sm font-mono text-muted-foreground cursor-pointer">
                              {scope}
                            </label>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                </ScrollArea>
              </>
            ) : (
              <div className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
                Select a role to view permissions
              </div>
            )}
          </div>
        </div>

        <Separator />
        <div className="p-3 flex items-center justify-between bg-muted/10">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onChange([])}
            className="text-muted-foreground hover:text-destructive"
          >
            Clear all
          </Button>
          <Button
            size="sm"
            onClick={() => setOpen(false)}
          >
            Apply
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

// ----------------------------------------------------------------------
// Main Page Component
// ----------------------------------------------------------------------

export default function OrgApiKeysPage() {
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)
  const isOwner = isOrgOwner(role)
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [listError, setListError] = useState<string | null>(null)
  const [availableScopes, setAvailableScopes] = useState<string[]>([])
  const [isForbidden, setIsForbidden] = useState(false)

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [createName, setCreateName] = useState("")
  const [createError, setCreateError] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [createdSecret, setCreatedSecret] = useState<ApiKeySecretResponse | null>(null)
  const [createScopes, setCreateScopes] = useState<string[]>([])

  const [revealKey, setRevealKey] = useState<ApiKey | null>(null)
  const [revealPassword, setRevealPassword] = useState("")
  const [revealError, setRevealError] = useState<string | null>(null)
  const [isRevealing, setIsRevealing] = useState(false)
  const [revealedSecret, setRevealedSecret] = useState<ApiKeySecretResponse | null>(null)

  const [revokeKey, setRevokeKey] = useState<ApiKey | null>(null)
  const [revokeError, setRevokeError] = useState<string | null>(null)
  const [isRevoking, setIsRevoking] = useState(false)

  const loadKeys = useCallback(async () => {
    if (!orgId) {
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setListError(null)
    setIsForbidden(false)
    try {
      const res = await admin.get<ApiKey[]>("/api-keys")
      setApiKeys(Array.isArray(res.data) ? res.data : [])
    } catch (err: any) {
      if (isForbiddenError(err)) {
        setIsForbidden(true)
      } else {
        setListError(getErrorMessage(err, "Unable to load API keys."))
      }
      setApiKeys([])
    } finally {
      setIsLoading(false)
    }
  }, [orgId])

  useEffect(() => {
    void loadKeys()
  }, [loadKeys])

  useEffect(() => {
    const loadScopes = async () => {
      if (!orgId) return
      try {
        const res = await admin.get<string[]>("/api-keys/scopes")
        setAvailableScopes(Array.isArray(res.data) ? res.data : [])
      } catch {
        setAvailableScopes([])
      }
    }
    void loadScopes()
  }, [orgId])

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!createName.trim()) {
      setCreateError("Name is required.")
      return
    }

    setIsCreating(true)
    setCreateError(null)
    try {
      const res = await admin.post<ApiKeySecretResponse>("/api-keys", {
        name: createName.trim(),
        scopes: createScopes,
      })
      setCreatedSecret(res.data)
      setCreateName("")
      setCreateScopes([])
      await loadKeys()
    } catch (err: any) {
      setCreateError(getErrorMessage(err, "Unable to create API key."))
    } finally {
      setIsCreating(false)
    }
  }

  const handleReveal = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!revealKey) return
    if (!revealPassword.trim()) {
      setRevealError("Password is required.")
      return
    }

    setIsRevealing(true)
    setRevealError(null)
    try {
      const res = await admin.post<ApiKeySecretResponse>(
        `/api-keys/${revealKey.key_id}/reveal`,
        { password: revealPassword }
      )
      setRevealedSecret(res.data)
      setRevealPassword("")
      await loadKeys()
    } catch (err: any) {
      setRevealError(getErrorMessage(err, "Unable to reveal API key."))
    } finally {
      setIsRevealing(false)
    }
  }

  const handleRevoke = async (event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault()
    if (!revokeKey) return

    setIsRevoking(true)
    setRevokeError(null)
    try {
      await admin.post(`/api-keys/${revokeKey.key_id}/revoke`)
      setRevokeKey(null)
      await loadKeys()
    } catch (err: any) {
      setRevokeError(getErrorMessage(err, "Unable to revoke API key."))
    } finally {
      setIsRevoking(false)
    }
  }

  const revealTitle = useMemo(() => {
    if (!revealKey) return "Reveal API key"
    return `Reveal ${revealKey.name}`
  }, [revealKey])

  if (!canManage) {
    return <ForbiddenState description="You do not have access to API keys." />
  }

  if (isForbidden) {
    return <ForbiddenState description="You do not have access to API keys." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">API keys</h1>
          <p className="text-text-muted text-sm">
            Manage credentials for server-to-server usage reporting.
          </p>
        </div>
        <Dialog
          open={isCreateOpen}
          onOpenChange={(open) => {
            setIsCreateOpen(open)
            if (!open) {
              setCreateError(null)
              setCreateName("")
              setCreateScopes([])
              setCreatedSecret(null)
            }
          }}
        >
          <DialogTrigger asChild>
            <Button size="sm">
              <IconPlus />
              Create API key
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Create API key</DialogTitle>
              <DialogDescription>
                Generate a new secret for this organization. Copy it now — you won&apos;t see it again.
              </DialogDescription>
            </DialogHeader>
            {createError && <Alert variant="destructive">{createError}</Alert>}
            {createdSecret ? (
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>New API key</Label>
                  <div className="flex flex-wrap items-center gap-2">
                    <Input readOnly value={createdSecret.api_key} className="font-mono" />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => navigator.clipboard.writeText(createdSecret.api_key)}
                    >
                      <IconCopy />
                      Copy
                    </Button>
                  </div>
                </div>
                <Alert>
                  <AlertDescription>
                    This key will not be shown again. Store it securely.
                  </AlertDescription>
                </Alert>
              </div>
            ) : (
              <form className="space-y-4" onSubmit={handleCreate}>
                <div className="space-y-2">
                  <Label htmlFor="api-key-name">Name</Label>
                  <Input
                    id="api-key-name"
                    value={createName}
                    onChange={(event) => setCreateName(event.target.value)}
                    placeholder="backend-prod"
                    autoComplete="off"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Scopes</Label>
                  <ScopeSelector
                    availableScopes={availableScopes}
                    value={createScopes}
                    onChange={setCreateScopes}
                  />
                </div>
                <DialogFooter>
                  <DialogClose asChild>
                    <Button type="button" variant="ghost">
                      Cancel
                    </Button>
                  </DialogClose>
                  <Button type="submit" disabled={isCreating}>
                    {isCreating ? "Creating..." : "Create key"}
                  </Button>
                </DialogFooter>
              </form>
            )}
            {createdSecret && (
              <DialogFooter>
                <DialogClose asChild>
                  <Button type="button">Done</Button>
                </DialogClose>
              </DialogFooter>
            )}
          </DialogContent>
        </Dialog>
      </div>

      {listError && <Alert variant="destructive">{listError}</Alert>}

      <div className="rounded-lg border border-border-subtle bg-bg-primary">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Key ID</TableHead>
              <TableHead>Scopes</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Last used</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="text-text-muted">
                  Loading API keys...
                </TableCell>
              </TableRow>
            ) : apiKeys.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-text-muted">
                  No API keys yet.
                </TableCell>
              </TableRow>
            ) : (
              apiKeys.map((key) => (
                <TableRow key={key.key_id}>
                  <TableCell className="font-medium">{key.name}</TableCell>
                  <TableCell className="font-mono text-xs text-text-muted">
                    {maskKeyID(key.key_id)}
                  </TableCell>
                  <TableCell className="text-text-muted text-xs max-w-[200px] truncate" title={key.scopes?.join(", ")}>
                    {formatScopes(key)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(key)}>{statusLabel(key)}</Badge>
                  </TableCell>
                  <TableCell className="text-text-muted text-xs">
                    {formatDateTime(key.last_used_at)}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        onClick={() => {
                          setRevealKey(key)
                          setRevealPassword("")
                          setRevealError(null)
                          setRevealedSecret(null)
                        }}
                      >
                        <IconEye />
                        Reveal
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        onClick={() => {
                          setRevealKey(key)
                          setRevealPassword("")
                          setRevealError(null)
                          setRevealedSecret(null)
                        }}
                      >
                        <IconRefresh />
                        Rotate
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        onClick={() => {
                          setRevokeKey(key)
                          setRevokeError(null)
                        }}
                        disabled={!isOwner}
                      >
                        <IconTrash />
                        Revoke
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog
        open={Boolean(revealKey)}
        onOpenChange={(open) => {
          if (!open) {
            setRevealKey(null)
            setRevealPassword("")
            setRevealError(null)
            setRevealedSecret(null)
          }
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{revealTitle}</DialogTitle>
            <DialogDescription>
              To reveal this API key, confirm your password. For security reasons, this will rotate the key.
            </DialogDescription>
          </DialogHeader>
          {revealError && <Alert variant="destructive">{revealError}</Alert>}
          {revealedSecret ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>New API key</Label>
                <div className="flex flex-wrap items-center gap-2">
                  <Input readOnly value={revealedSecret.api_key} className="font-mono" />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => navigator.clipboard.writeText(revealedSecret.api_key)}
                  >
                    <IconCopy />
                    Copy
                  </Button>
                </div>
              </div>
              <Alert>
                <AlertDescription>
                  This key will not be shown again. Store it securely.
                </AlertDescription>
              </Alert>
              <DialogFooter>
                <DialogClose asChild>
                  <Button type="button">Done</Button>
                </DialogClose>
              </DialogFooter>
            </div>
          ) : (
            <form className="space-y-4" onSubmit={handleReveal}>
              <div className="space-y-2">
                <Label htmlFor="reveal-password">Password</Label>
                <Input
                  id="reveal-password"
                  type="password"
                  value={revealPassword}
                  onChange={(event) => setRevealPassword(event.target.value)}
                />
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button type="button" variant="ghost">
                    Cancel
                  </Button>
                </DialogClose>
                <Button type="submit" disabled={isRevealing}>
                  {isRevealing ? "Revealing..." : "Confirm & rotate"}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(revokeKey)}
        onOpenChange={(open) => {
          if (!open) {
            setRevokeKey(null)
            setRevokeError(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke API key</AlertDialogTitle>
            <AlertDialogDescription>
              This will immediately disable the key. Existing integrations will stop working.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {revokeError && <Alert variant="destructive">{revokeError}</Alert>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isRevoking}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleRevoke} disabled={isRevoking}>
              {isRevoking ? "Revoking..." : "Revoke"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
