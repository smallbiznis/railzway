import { useState } from "react"
import { useSearchParams } from "react-router-dom"
import { Zap, BarChart3 } from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { useOrgStore } from "@/stores/orgStore"
import { canManageBilling } from "@/lib/roles"

import { InboxTab } from "../components/InboxTab"
import { MyWorkTab } from "../components/MyWorkTab"
import { RecentlyResolvedTab } from "../components/RecentlyResolvedTab"
import { TeamViewTab } from "../components/TeamViewTab"
import { PerformanceDashboard } from "../components/PerformanceDashboard"
import { BillingAnalyticsHeader } from "../components/BillingAnalyticsHeader"
import { ExposureAnalysisTab } from "../components/ExposureAnalysisTab"

export default function OrgBillingOperationsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get("tab") || "inbox"
  const [showPerformance, setShowPerformance] = useState(false)

  const role = useOrgStore((state) => state.currentOrg?.role)
  const isManager = canManageBilling(role)

  const handleTabChange = (value: string) => {
    setSearchParams({ tab: value }, { replace: true })
  }

  return (
    <div className="mx-auto w-full max-w-6xl space-y-8 py-8">
      {/* Page Header */}
      <header className="flex items-center justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="bg-primary/10 p-2 rounded-lg">
              <Zap className="h-5 w-5 text-primary" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight">Billing Operations</h1>
          </div>
          <p className="text-muted-foreground">
            Task-centric workspace for managing finance and billing exceptions.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            className="gap-2"
            onClick={() => setShowPerformance(true)}
          >
            <BarChart3 className="h-4 w-4" />
            Performance Dashboard
          </Button>
        </div>
      </header>

      <BillingAnalyticsHeader />

      {/* Main IA Tabs */}
      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-6">
        <div className="border-b">
          <TabsList className="bg-transparent h-auto p-0 gap-6">
            <TabsTrigger
              value="inbox"
              className="data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-0 py-2 border-b-2 border-transparent"
            >
              Inbox
            </TabsTrigger>
            <TabsTrigger
              value="my-work"
              className="data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-0 py-2 border-b-2 border-transparent"
            >
              My Work
            </TabsTrigger>
            <TabsTrigger
              value="recently-resolved"
              className="data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-0 py-2 border-b-2 border-transparent"
            >
              Recently Resolved
            </TabsTrigger>
            {isManager && (
              <TabsTrigger
                value="team"
                className="data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-0 py-2 border-b-2 border-transparent"
              >
                Team View
              </TabsTrigger>
            )}
            {isManager && (
              <TabsTrigger
                value="exposure"
                className="data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-0 py-2 border-b-2 border-transparent"
              >
                Exposure Analysis
              </TabsTrigger>
            )}
          </TabsList>
        </div>

        <TabsContent value="inbox" className="mt-0">
          <InboxTab />
        </TabsContent>
        <TabsContent value="my-work" className="mt-0">
          <MyWorkTab />
        </TabsContent>
        <TabsContent value="recently-resolved" className="mt-0">
          <RecentlyResolvedTab />
        </TabsContent>
        {isManager && (
          <TabsContent value="team" className="mt-0">
            <TeamViewTab />
          </TabsContent>
        )}
        {isManager && (
          <TabsContent value="exposure" className="mt-0">
            <ExposureAnalysisTab />
          </TabsContent>
        )}
      </Tabs>

      <PerformanceDashboard open={showPerformance} onOpenChange={setShowPerformance} />
    </div >
  )
}
