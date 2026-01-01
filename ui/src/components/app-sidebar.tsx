import * as React from "react"
import {
  IconBox,
  IconDashboard,
  IconFileDescription,
  IconInnerShadowTop,
  IconKey,
  IconListDetails,
  IconMeterCube,
  IconPlugConnected,
  IconShieldCheck,
  IconSettings,
  IconUsers,
} from "@tabler/icons-react"
import { NavLink, useParams } from "react-router-dom"

import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
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
      title: "Meters",
      url: `${orgBasePath}/meter`,
      icon: IconMeterCube,
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
      title: "Invoices",
      url: `${orgBasePath}/invoices`,
      icon: IconFileDescription,
    },
    {
      title: "Invoice templates",
      url: `${orgBasePath}/invoice-templates`,
      icon: IconFileDescription,
    },
  ]

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
  ]

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
        <NavSecondary items={navSecondary} className="mt-auto" />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={data.user} />
      </SidebarFooter>
    </Sidebar>
  )
}
