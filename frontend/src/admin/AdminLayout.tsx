import { useEffect, useState } from "react";
import { Link, Navigate, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";

export default function AdminLayout() {
  const [me, setMe] = useState<{ username: string } | null>(null);
  const [checked, setChecked] = useState(false);
  const nav = useNavigate();
  const location = useLocation();

  useEffect(() => {
    adminApi
      .me()
      .then((m) => setMe(m))
      .catch((e) => {
        if (!(e instanceof ApiError && e.status === 401)) {
          console.error(e);
        }
        setMe(null);
      })
      .finally(() => setChecked(true));
  }, [location.pathname]);

  if (!checked) return null;

  if (!me && location.pathname !== "/admin/login") {
    return <Navigate to="/admin/login" replace />;
  }
  if (me && location.pathname === "/admin/login") {
    return <Navigate to="/admin" replace />;
  }

  if (!me) {
    return <Outlet />;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <Link to="/" className="text-sm text-neutral-500 hover:text-neutral-900">
            ← На сайт
          </Link>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Админка</h1>
          <p className="text-sm text-neutral-500">Вы вошли как {me.username}</p>
        </div>
        <button
          className="btn-secondary"
          onClick={async () => {
            try {
              await adminApi.logout();
            } catch {
              /* noop */
            }
            setMe(null);
            nav("/admin/login", { replace: true });
          }}
        >
          Выйти
        </button>
      </div>

      <nav className="flex gap-4 border-b border-neutral-200">
        <AdminTab to="/admin">Предметы</AdminTab>
        <AdminTab to="/admin/variants">Варианты</AdminTab>
        <AdminTab to="/admin/questions">Вопросы</AdminTab>
      </nav>

      <Outlet />
    </div>
  );
}

function AdminTab({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        `-mb-px border-b-2 px-1 py-2 text-sm transition-colors ${
          isActive
            ? "border-neutral-900 text-neutral-900"
            : "border-transparent text-neutral-500 hover:text-neutral-900"
        }`
      }
    >
      {children}
    </NavLink>
  );
}
