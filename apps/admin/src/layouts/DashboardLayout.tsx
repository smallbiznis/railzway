import type { CSSProperties } from "react"
import { useEffect, useMemo, useState } from "react"
import { Outlet, useNavigate, useParams } from "react-router-dom"

import { auth } from "@/api/client"
import { AppSidebar } from "@/components/app-sidebar"
import { SiteHeader } from "@/components/site-header"
// import { WorkbenchDock } from "@/components/workbench-dock"
import { Skeleton } from "@/components/ui/skeleton"
import {
  SidebarInset,
  SidebarProvider,
} from "@/components/ui/sidebar"
import { Toaster } from "@/components/ui/sonner"
import { useOrgStore } from "@/stores/orgStore"

export default function DashboardLayout() {
  const { orgId } = useParams<{ orgId: string }>()
  const navigate = useNavigate()
  const { setCurrentOrg, setOrgs } = useOrgStore()
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!orgId) return
    let active = true
    setIsLoading(true)

    auth
      .post(`/user/using/${orgId}`)
      .then(() => auth.get("/user/orgs"))
      .then((res) => {
        if (!active) return
        const list = res.data?.orgs ?? []
        setOrgs(list)
        const match = list.find(
          (item: any) => String(item?.id ?? item?.ID ?? item?.org_id ?? "") === orgId
        )
        setCurrentOrg(match ?? { id: orgId, name: `Org ${orgId}`, role: "MEMBER" })
        setIsLoading(false)
      })
      .catch(() => {
        if (!active) return
        setIsLoading(false)
        setCurrentOrg(null)
        navigate("/orgs", { replace: true })
      })

    return () => {
      active = false
    }
  }, [navigate, orgId, setCurrentOrg, setOrgs])

  const content = useMemo(() => {
    if (isLoading) {
      return (
        <div className="space-y-3">
          <Skeleton className="h-10 w-72" />
          <Skeleton className="h-4 w-52" />
        </div>
      )
    }
    return <Outlet />
  }, [isLoading])

  return (
    <>
      <SidebarProvider
        style={
          {
            "--sidebar-width": "calc(var(--spacing) * 72)",
            "--header-height": "calc(var(--spacing) * 12)",
          } as CSSProperties
        }
      >
        <AppSidebar variant="inset" />
        <SidebarInset>
          <SiteHeader />
          <div className="flex flex-1 flex-col">
            <div className="@container/main flex flex-1 flex-col gap-2">
              <div className="flex flex-1 flex-col gap-4 py-4 md:gap-6 md:py-6">
                <div className="px-4 lg:px-6">
                  {content}
                </div>
              </div>
            </div>
          </div>
        </SidebarInset>
      </SidebarProvider>
      <Toaster />
      {/* <WorkbenchDock /> */}
    </>
  )
}
