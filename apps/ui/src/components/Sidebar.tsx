import { NavLink } from "react-router-dom";

export default function Sidebar({ orgId }: { orgId: string }) {
  const base = `/orgs/${orgId}`;

  return (
    <aside className="w-64 bg-bg-surface text-text-primary">
      <div className="p-4 font-bold text-xl">Valora</div>
      <nav className="flex flex-col gap-1 p-2">
        <NavLink to={`${base}/dashboard`}>Dashboard</NavLink>
        <NavLink to={`${base}/products`}>Products</NavLink>
        <NavLink to={`${base}/meter`}>Meters</NavLink>
        <NavLink to={`${base}/customers`}>Customers</NavLink>
        <NavLink to={`${base}/subscriptions`}>Subscriptions</NavLink>
        <NavLink to={`${base}/invoices`}>Invoices</NavLink>
        <NavLink to={`${base}/settings`}>Settings</NavLink>
      </nav>
    </aside>
  );
}
