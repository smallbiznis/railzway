import * as React from "react"
import {
  IconInnerShadowTop,
} from "@tabler/icons-react"
import { NavLink, useLocation, useParams } from "react-router-dom"

import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubItem,
  SidebarMenuSubButton,
} from "@/components/ui/sidebar"
import { BarChart3, ChevronRight, Copy, CreditCard, Gauge, History, Home, Key, Package, Receipt, RefreshCcw, Settings, Tag, Users, Zap } from "lucide-react"

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"

const data = {
  user: {
    name: "shadcn",
    email: "m@example.com",
    avatar: "/avatars/shadcn.jpg",
  },
}

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
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
      items: [
        { title: "Inbox", url: `${orgBasePath}/billing/operations?tab=inbox` },
        { title: "My Work", url: `${orgBasePath}/billing/operations?tab=my-work` },
        { title: "Recently Resolved", url: `${orgBasePath}/billing/operations?tab=recently-resolved` },
        { title: "Team View", url: `${orgBasePath}/billing/operations?tab=team`, hide: !canManageBilling(role) },
      ].filter(i => !i.hide)
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

  const location = useLocation()

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
                <IconInnerShadowTop className="!size-5" />
                <span className="text-base font-semibold">Valora</span>
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
                  const isAnySubActive = item.items?.some(sub =>
                    location.pathname + location.search === sub.url
                  )

                  return (
                    <SidebarMenuItem key={item.title}>
                      {item.items && item.items.length > 0 ? (
                        <Collapsible asChild defaultOpen={isAnySubActive} className="group/collapsible">
                          <div>
                            <CollapsibleTrigger asChild>
                              <SidebarMenuButton tooltip={item.title} isActive={isAnySubActive}>
                                <item.icon />
                                <span>{item.title}</span>
                                <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                              </SidebarMenuButton>
                            </CollapsibleTrigger>
                            <CollapsibleContent>
                              <SidebarMenuSub>
                                {item.items.map((subItem) => (
                                  <SidebarMenuSubItem key={subItem.title}>
                                    <SidebarMenuSubButton
                                      asChild
                                      isActive={location.pathname + location.search === subItem.url}
                                    >
                                      <NavLink to={subItem.url} end={false}>
                                        <span>{subItem.title}</span>
                                      </NavLink>
                                    </SidebarMenuSubButton>
                                  </SidebarMenuSubItem>
                                ))}
                              </SidebarMenuSub>
                            </CollapsibleContent>
                          </div>
                        </Collapsible>
                      ) : (
                        <SidebarMenuButton asChild tooltip={item.title}>
                          <NavLink to={item.url}>
                            <item.icon />
                            <span>{item.title}</span>
                          </NavLink>
                        </SidebarMenuButton>
                      )}
                    </SidebarMenuItem>
                  )
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
        <NavSecondary items={navSecondary} className="mt-auto" />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={data.user} />
      </SidebarFooter>
    </Sidebar>
  )
}
