import { useMutation, useQuery, useInfiniteQuery, useQueryClient } from "@tanstack/react-query"
import { admin } from "@/api/client"
import type {
  InboxResponse,
  MyWorkResponse,
  RecentlyResolvedResponse,
  TeamViewResponse,
  ClaimAssignmentRequest,
  ClaimAssignmentResponse,
  ResolveAssignmentRequest,
  ResolveAssignmentResponse,
  ReleaseAssignmentRequest,
  PerformanceResponse,
  PerformanceData,
  ExposureAnalysisResponse,
} from "../types/ia-types"

export function useInbox(limit = 25) {
  return useInfiniteQuery({
    queryKey: ["billing-operations", "inbox", limit],
    queryFn: async ({ pageParam = undefined as string | undefined }) => {
      const res = await admin.get<InboxResponse>("/billing-operations/inbox", {
        params: { limit, cursor: pageParam },
      })
      return res.data
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor || undefined,
  })
}

export function useMyWork(limit = 50) {
  return useQuery({
    queryKey: ["billing-operations", "my-work", limit],
    queryFn: async () => {
      const res = await admin.get<MyWorkResponse>("/billing-operations/my-work", {
        params: { limit },
      })
      return res.data
    },
  })
}

export function useRecentlyResolved(limit = 50) {
  return useQuery({
    queryKey: ["billing-operations", "recently-resolved", limit],
    queryFn: async () => {
      const res = await admin.get<RecentlyResolvedResponse>("/billing-operations/recently-resolved", {
        params: { limit },
      })
      return res.data
    },
  })
}

export function useTeamView() {
  return useQuery({
    queryKey: ["billing-operations", "team-view"],
    queryFn: async () => {
      const res = await admin.get<TeamViewResponse>("/billing-operations/team")
      return res.data
    },
  })
}


export function useFinOpsPerformance(periodType = "daily") {
  return useQuery({
    queryKey: ["finops", "performance", "me", periodType],
    queryFn: async () => {
      const res = await admin.get<PerformanceResponse>("/finops/performance/me", {
        params: { period_type: periodType },
      })
      const apiData = res.data
      if (apiData?.snapshots && apiData.snapshots.length > 0) {
        return {
          current: apiData.snapshots[0],
          history: apiData.snapshots,
        } as PerformanceData
      }
      return null
    },
  })
}

export function useExposureAnalysis() {
  return useQuery({
    queryKey: ["finops", "exposure-analysis"],
    queryFn: async () => {
      const res = await admin.get<ExposureAnalysisResponse>("/finops/exposure-analysis")
      return res.data
    },
  })
}

// ===== Mutation Hooks =====

export function useClaimAssignment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (request: ClaimAssignmentRequest) => {
      const res = await admin.post<ClaimAssignmentResponse>("/billing-operations/claim", request)
      return res.data
    },
    onSuccess: () => {
      // Invalidate both inbox and my-work to refresh the UI
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "inbox"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "my-work"] })
    },
  })
}

export function useResolveAssignment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (request: ResolveAssignmentRequest) => {
      const res = await admin.post<ResolveAssignmentResponse>("/billing-operations/resolve", request)
      return res.data
    },
    onSuccess: () => {
      // Invalidate all relevant queries
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "my-work"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "recently-resolved"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "team-view"] })
    },
  })
}

export function useReleaseAssignment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (request: ReleaseAssignmentRequest) => {
      await admin.post("/billing-operations/release", request)
    },
    onSuccess: () => {
      // Invalidate all relevant queries
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "inbox"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "my-work"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "recently-resolved"] })
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "team-view"] })
    },
  })
}

export function useRecordFollowUp() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (data: { assignment_id: string; email_provider: string }) => {
      const res = await admin.post("/billing-operations/record-follow-up", data)
      return res.data
    },
    onSuccess: () => {
      // Invalidate my-work to refresh follow-up counts
      queryClient.invalidateQueries({ queryKey: ["billing-operations", "my-work"] })
    },
  })
}
