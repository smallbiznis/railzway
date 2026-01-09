import { useParams, useNavigate } from "react-router-dom";

export default function Topbar() {
  const { orgId } = useParams();
  const navigate = useNavigate();

  function logout() {
    fetch("/api/logout", { method: "POST" }).finally(() => {
      navigate("/login");
    });
  }

  return (
    <header className="h-14 border-b border-border-subtle flex items-center justify-between px-6 bg-bg-surface">
      {/* Left */}
      <div className="flex items-center gap-4">
        <span className="font-semibold text-lg">Valora</span>
        <span className="text-sm text-text-muted">Org: {orgId}</span>
      </div>

      {/* Right */}
      <div className="flex items-center gap-4">
        <button
          className="text-sm text-text-secondary hover:text-text-primary"
          onClick={() => navigate("/orgs")}
        >
          Switch Org
        </button>

        <div className="relative">
          <button className="text-sm font-medium">Account ▾</button>
          {/* simple dropdown placeholder */}
        </div>

        <button
          onClick={logout}
          className="text-sm text-status-error hover:text-status-error/80"
        >
          Logout
        </button>
      </div>
    </header>
  );
}
