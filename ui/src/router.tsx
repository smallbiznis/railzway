import { useEffect, useState } from "react"
import { Navigate, Outlet, createBrowserRouter, useLocation } from "react-router-dom"

import DashboardLayout from "@/layouts/DashboardLayout"
import LoginPage from "@/pages/login"
import OnboardingPage from "@/pages/onboarding"
import OrgDashboard from "@/pages/org/OrgDashboard"
import OrgCustomersPage from "@/pages/org/OrgCustomersPage"
import OrgApiKeysPage from "@/pages/org/OrgApiKeysPage"
import OrgAuditLogsPage from "@/pages/org/OrgAuditLogsPage"
import OrgInvoiceDetailPage from "@/pages/org/OrgInvoiceDetailPage"
import OrgInvoicesPage from "@/pages/org/OrgInvoicesPage"
import OrgInvoiceTemplateFormPage from "@/pages/org/OrgInvoiceTemplateFormPage"
import OrgInvoiceTemplatesPage from "@/pages/org/OrgInvoiceTemplatesPage"
import OrgMeterPage from "@/pages/org/OrgMeterPage"
import OrgMeterDetailPage from "@/pages/org/OrgMeterDetailPage"
import OrgMeterCreatePage from "@/pages/org/OrgMeterCreatePage"
import OrgProductDetailPage from "@/pages/org/OrgProductDetailPage"
import OrgProductsPage from "@/pages/org/OrgProductsPage"
import OrgSettings from "@/pages/org/OrgSettings"
import OrgSubscriptionCreatePage from "@/pages/org/OrgSubscriptionCreatePage"
import OrgSubscriptionsPage from "@/pages/org/OrgSubscriptionsPage"
import OrgResolverPage from "@/pages/orgs"
import CreatePrice from "@/pages/products/CreatePrice"
import CreateProduct from "@/pages/products/CreateProduct"
import ChangePasswordPage from "@/pages/change-password"
import SignupPage from "@/pages/signup"
import { useAuthStore } from "@/stores/authStore"

// eslint-disable-next-line react-refresh/only-export-components
function RequireAuth() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const mustChangePassword = useAuthStore((s) => s.mustChangePassword)
  const location = useLocation()
  const [hasHydrated, setHasHydrated] = useState(
    useAuthStore.persist.hasHydrated()
  )
  useEffect(() => {
    const unsubscribe = useAuthStore.persist.onFinishHydration(() => {
      setHasHydrated(true)
    })
    if (useAuthStore.persist.hasHydrated()) {
      setHasHydrated(true)
    }
    return unsubscribe
  }, [])
  if (!hasHydrated) {
    return <div className="flex min-h-screen items-center justify-center text-text-muted text-sm">Loading session...</div>
  }
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  if (mustChangePassword && location.pathname !== "/change-password") {
    return <Navigate to="/change-password" replace />
  }
  return <Outlet />
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Navigate to="/orgs" replace />,
  },
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/signup",
    element: <SignupPage />,
  },
  {
    element: <RequireAuth />,
    children: [
      { path: "/change-password", element: <ChangePasswordPage /> },
      { path: "/orgs", element: <OrgResolverPage /> },
      { path: "/onboarding", element: <OnboardingPage /> },
      {
        path: "/orgs/:orgId",
        element: <DashboardLayout />,
        children: [
          { index: true, element: <Navigate to="dashboard" replace /> },
          { path: "dashboard", element: <OrgDashboard /> },
          {
            path: "products",
            element: <OrgProductsPage />,
          },
          {
            path: "products/create",
            element: <CreateProduct />,
          },
          {
            path: "products/:productId/prices/create",
            element: <CreatePrice />,
          },
          {
            path: "products/:productId",
            element: <OrgProductDetailPage />,
          },
          {
            path: "prices",
            element: <Navigate to="../products" replace />,
          },
          {
            path: "pricings",
            element: <Navigate to="../products" replace />,
          },
          {
            path: "price-amounts",
            element: <Navigate to="../products" replace />,
          },
          {
            path: "price-tiers",
            element: <Navigate to="../products" replace />,
          },
          {
            path: "meter",
            element: <OrgMeterPage />,
          },
          {
            path: "meter/create",
            element: <OrgMeterCreatePage />,
          },
          {
            path: "meter/:meterId",
            element: <OrgMeterDetailPage />,
          },
          {
            path: "api-keys",
            element: <OrgApiKeysPage />,
          },
          {
            path: "audit-logs",
            element: <OrgAuditLogsPage />,
          },
          {
            path: "customers",
            element: <OrgCustomersPage />,
          },
          {
            path: "subscriptions",
            element: <OrgSubscriptionsPage />,
          },
          {
            path: "subscriptions/create",
            element: <OrgSubscriptionCreatePage />,
          },
          {
            path: "invoices",
            element: <OrgInvoicesPage />,
          },
          {
            path: "invoices/:invoiceId",
            element: <OrgInvoiceDetailPage />,
          },
          {
            path: "invoice-templates",
            element: <OrgInvoiceTemplatesPage />,
          },
          {
            path: "invoice-templates/create",
            element: <OrgInvoiceTemplateFormPage />,
          },
          {
            path: "invoice-templates/:templateId",
            element: <OrgInvoiceTemplateFormPage />,
          },
          { path: "settings", element: <OrgSettings /> },
        ],
      },
    ],
  },
  { path: "*", element: <Navigate to="/orgs" replace /> },
])
