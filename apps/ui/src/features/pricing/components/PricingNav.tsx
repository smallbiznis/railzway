import { NavLink, useParams } from "react-router-dom"

import { cn } from "@/lib/utils"

const navItems = [
  { label: "Prices", path: "prices" },
  { label: "Pricing models", path: "pricings" },
  { label: "Price amounts", path: "price-amounts" },
  { label: "Price tiers", path: "price-tiers" },
]

export function PricingNav() {
  const { orgId } = useParams()
  const base = orgId ? `/orgs/${orgId}` : "/orgs"

  return (
    <div className="flex flex-wrap items-center gap-2">
      {navItems.map((item) => (
        <NavLink
          key={item.path}
          to={`${base}/${item.path}`}
          className={({ isActive }) =>
            cn(
              "rounded-full border px-3 py-1 text-xs font-medium transition-colors",
              isActive
                ? "border-accent-primary text-accent-primary"
                : "border-border-subtle text-text-muted hover:text-text-primary"
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </div>
  )
}
