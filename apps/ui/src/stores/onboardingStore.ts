import { create } from "zustand"

type OnboardingState = {
  currentStep: number
  orgId: string | null
  nextStep: () => void
  prevStep: () => void
  setOrgId: (orgId: string) => void
  reset: () => void
}

export const useOnboardingStore = create<OnboardingState>((set) => ({
  currentStep: 1,
  orgId: null,
  nextStep: () =>
    set((state) => ({
      currentStep: Math.min(state.currentStep + 1, 4),
    })),
  prevStep: () =>
    set((state) => ({
      currentStep: Math.max(state.currentStep - 1, 1),
    })),
  setOrgId: (orgId) => set({ orgId }),
  reset: () => set({ currentStep: 1, orgId: null }),
}))
