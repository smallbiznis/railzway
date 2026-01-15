import * as React from "react"
import { NavLink, useLocation, useParams } from "react-router-dom"

import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { BarChart3, Copy, CreditCard, Gauge, History, Home, Key, Package, Receipt, RefreshCcw, Settings, Tag, Users, Zap } from "lucide-react"
import { cn } from "@/lib/utils"

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation()
  const { orgId } = useParams()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canAccessAdmin = canManageBilling(role)
  const orgBasePath = orgId ? `/orgs/${orgId}` : "/orgs"

  const navMain = [
    {
      title: "Home",
      url: `${orgBasePath}/home`,
      icon: Home,
    },
    // Pricing stays within each product so navigation reflects user intent, not backend tables.
    {
      title: "Products",
      url: `${orgBasePath}/products`,
      icon: Package,
    },
    {
      title: "Pricing",
      url: `${orgBasePath}/prices`,
      icon: Tag,
    },
    {
      title: "Meters",
      url: `${orgBasePath}/meter`,
      icon: Gauge,
    },
  ].filter(() => canAccessAdmin)

  const billingNav = [
    {
      title: "Overview",
      url: `${orgBasePath}/billing/overview`,
      icon: BarChart3,
    },
    {
      title: "Operations",
      url: `${orgBasePath}/billing/operations`,
      icon: Zap,
    },
    {
      title: "Invoices",
      url: `${orgBasePath}/invoices`,
      icon: Receipt,
    },
    {
      title: "Customers",
      url: `${orgBasePath}/customers`,
      icon: Users,
    },
    {
      title: "Subscriptions",
      url: `${orgBasePath}/subscriptions`,
      icon: RefreshCcw,
    },
    {
      title: "Invoice templates",
      url: `${orgBasePath}/invoice-templates`,
      icon: Copy,
    },
  ].filter(() => canAccessAdmin)

  const navSecondary = [
    {
      title: "API Keys",
      url: `${orgBasePath}/api-keys`,
      icon: Key,
    },
    {
      title: "Payment providers",
      url: `${orgBasePath}/payment-providers`,
      icon: CreditCard,
    },
    {
      title: "Audit Logs",
      url: `${orgBasePath}/audit-logs`,
      icon: History,
    },
    {
      title: "Settings",
      url: `${orgBasePath}/settings`,
      icon: Settings,
    },
  ].filter(() => canAccessAdmin)


  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              className="data-[slot=sidebar-menu-button]:!p-1.5"
            >
              <NavLink to={`${orgBasePath}/home`}>
                <img src="/primary.svg" className="size-6" alt="Logo" />
                <span className="text-base font-semibold">Railzway</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navMain} />
        {billingNav.length > 0 && (
          <SidebarGroup>
            <SidebarGroupLabel>Billing</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {billingNav.map((item) => {
                  const isActive = location.pathname.replace(/\/$/, "") === item.url.replace(/\/$/, "")
                  return (
                    <SidebarMenuItem key={item.title}>
                      <SidebarMenuButton asChild tooltip={item.title}>
                        <NavLink
                          to={item.url}
                          className={cn(
                            "flex w-full items-center gap-2",
                            isActive && "bg-bg-subtle text-accent-primary font-medium"
                          )}
                        >
                          <item.icon />
                          <span>{item.title}</span>
                        </NavLink>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  )
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
        <NavSecondary items={navSecondary} className="mt-auto" />
      </SidebarContent>
    </Sidebar>
  )
}
