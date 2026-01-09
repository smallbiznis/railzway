import * as React from "react"
import {
  IconBox,
  IconDashboard,
  IconChartLine,
  IconActivity,
  IconFileDescription,
  IconInnerShadowTop,
  IconKey,
  IconListDetails,
  IconMeterCube,
  IconPlugConnected,
  IconTag,
  IconShieldCheck,
  IconSettings,
  IconUsers,
} from "@tabler/icons-react"
import { NavLink, useParams } from "react-router-dom"

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
} from "@/components/ui/sidebar"

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
      title: "Dashboard",
      url: `${orgBasePath}/dashboard`,
      icon: IconDashboard,
    },
    // Pricing stays within each product so navigation reflects user intent, not backend tables.
    {
      title: "Products",
      url: `${orgBasePath}/products`,
      icon: IconBox,
    },
    {
      title: "Pricing",
      url: `${orgBasePath}/prices`,
      icon: IconTag,
    },
    {
      title: "Meters",
      url: `${orgBasePath}/meter`,
      icon: IconMeterCube,
    },
  ].filter(() => canAccessAdmin)

  const billingNav = [
    {
      title: "Overview",
      url: `${orgBasePath}/billing/overview`,
      icon: IconChartLine,
    },
    {
      title: "Operations",
      url: `${orgBasePath}/billing/operations`,
      icon: IconActivity,
    },
    {
      title: "Invoices",
      url: `${orgBasePath}/invoices`,
      icon: IconFileDescription,
    },
    {
      title: "Customers",
      url: `${orgBasePath}/customers`,
      icon: IconUsers,
    },
    {
      title: "Subscriptions",
      url: `${orgBasePath}/subscriptions`,
      icon: IconListDetails,
    },
    {
      title: "Invoice templates",
      url: `${orgBasePath}/invoice-templates`,
      icon: IconFileDescription,
    },
  ].filter(() => canAccessAdmin)

  const navSecondary = [
    {
      title: "API Keys",
      url: `${orgBasePath}/api-keys`,
      icon: IconKey,
    },
    {
      title: "Payment providers",
      url: `${orgBasePath}/payment-providers`,
      icon: IconPlugConnected,
    },
    {
      title: "Audit Logs",
      url: `${orgBasePath}/audit-logs`,
      icon: IconShieldCheck,
    },
    {
      title: "Settings",
      url: `${orgBasePath}/settings`,
      icon: IconSettings,
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
              <NavLink to={`${orgBasePath}/dashboard`}>
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
                {billingNav.map((item) => (
                  <SidebarMenuItem key={item.title}>
                    <SidebarMenuButton asChild tooltip={item.title}>
                      <NavLink to={item.url}>
                        <item.icon />
                        <span>{item.title}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
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
