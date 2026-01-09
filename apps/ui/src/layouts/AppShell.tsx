import { useEffect, useMemo, useState } from "react"
import { NavLink, Outlet, useNavigate, useParams } from "react-router-dom"

import { auth } from "@/api/client"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuthStore } from "@/stores/authStore"
import { useOrgStore } from "@/stores/orgStore"
import { cn } from "@/lib/utils"

const navItems = [
  { label: "Dashboard", path: "dashboard" },
  { label: "Products", path: "products" },
  { label: "Meters", path: "meter" },
  { label: "Customers", path: "customers" },
  { label: "Subscriptions", path: "subscriptions" },
  { label: "Invoices", path: "invoices" },
  { label: "Settings", path: "settings" },
]

export function AppShell() {
  const { orgId } = useParams<{ orgId: string }>()
  const navigate = useNavigate()
  const { currentOrg, setCurrentOrg, setOrgs } = useOrgStore()
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    let isMounted = true
    if (!orgId) return
    setIsLoading(true)
    auth
      .post(`/user/using/${orgId}`)
      .then(() => auth.get("/user/orgs"))
      .then((res) => {
        if (!isMounted) return
        const list = res.data?.orgs ?? []
        setOrgs(list)
        const match = list.find(
          (item: any) => String(item?.id ?? item?.ID ?? item?.org_id ?? "") === orgId
        )
        setCurrentOrg(match ?? { id: orgId, name: `Org ${orgId}`, role: "MEMBER" })
        setIsLoading(false)
      })
      .catch(() => {
        if (!isMounted) return
        setIsLoading(false)
        setCurrentOrg(null)
        navigate("/orgs", { replace: true })
      })

    return () => {
      isMounted = false
    }
  }, [navigate, orgId, setCurrentOrg, setOrgs])

  const sidebar = useMemo(() => {
    if (!orgId) return null
    return navItems.map((item) => ({
      ...item,
      to: `/orgs/${orgId}/${item.path}`,
    }))
  }, [orgId])

  if (isLoading || !currentOrg) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg-subtle/40">
        <div className="space-y-3">
          <Skeleton className="h-10 w-72" />
          <Skeleton className="h-4 w-52" />
        </div>
      </div>
    )
  }

  return (
    <div className="grid min-h-screen grid-cols-[260px_1fr] bg-bg-subtle/20">
      <aside className="border-r border-border-subtle bg-bg-surface">
        <div className="px-4 py-5">
          <div className="text-lg font-semibold">Valora</div>
          <div className="text-sm text-text-muted">{currentOrg.name}</div>
        </div>
        <Separator />
        <nav className="flex flex-col gap-1 p-3">
          {sidebar?.map((item) => (
            <NavLink
              key={item.path}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-bg-subtle",
                  isActive && "bg-bg-subtle text-accent-primary"
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="flex flex-col">
        <Topbar />
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function Topbar() {
  const { orgs, currentOrg, setCurrentOrg } = useOrgStore()
  const navigate = useNavigate()
  const logout = useAuthStore((s) => s.logout)

  const initial = currentOrg?.name?.[0]?.toUpperCase() ?? "O"
  const handleOrgSwitch = async (org: { id: string; name: string; role?: string }) => {
    try {
      await auth.post(`/user/using/${org.id}`)
      setCurrentOrg(org)
      navigate(`/orgs/${org.id}/dashboard`)
    } catch (err) {
      console.error("Failed to switch org", err)
    }
  }

  return (
    <header className="flex items-center justify-between border-b border-border-subtle bg-bg-surface px-6 py-4">
      <div className="flex items-center gap-3">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" className="gap-2">
              <span className="font-medium">{currentOrg?.name}</span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuLabel>Workspaces</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {orgs.map((org) => (
              <DropdownMenuItem
                key={org.id}
                onSelect={() => {
                  void handleOrgSwitch(org)
                }}
              >
                {org.name}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => navigate("/onboarding")}>
              + New workspace
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Avatar className="cursor-pointer">
            <AvatarFallback>{initial}</AvatarFallback>
          </Avatar>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>Account</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => navigate("/orgs")}>
            Account settings
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={async () => {
              await logout()
              navigate("/login", { replace: true })
            }}
          >
            Logout
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  )
}
