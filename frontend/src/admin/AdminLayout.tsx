import { useEffect, useState } from "react";
import { Link, Navigate, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { AdminMe } from "../api/types";

export default function AdminLayout() {
  const [me, setMe] = useState<AdminMe | null>(null);
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

  const isSuper = me.role === "superadmin";

  // editor не имеет доступа к управлению предметами — кидаем на варианты.
  if (!isSuper && (location.pathname === "/admin" || location.pathname === "/admin/")) {
    return <Navigate to="/admin/variants" replace />;
  }
  if (!isSuper && location.pathname.startsWith("/admin/users")) {
    return <Navigate to="/admin/variants" replace />;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <Link to="/" className="text-sm text-neutral-500 hover:text-neutral-900">
            ← На сайт
          </Link>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Админка</h1>
          <p className="text-sm text-neutral-500">
            {me.username}
            <span className="ml-2 inline-flex items-center rounded-full bg-neutral-100 px-2 py-0.5 text-xs font-medium text-neutral-700">
              {isSuper ? "суперадмин" : "редактор"}
            </span>
          </p>
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
        {isSuper ? <AdminTab to="/admin">Предметы</AdminTab> : null}
        <AdminTab to="/admin/variants">Варианты</AdminTab>
        <AdminTab to="/admin/questions">Вопросы</AdminTab>
        <AdminTab to="/admin/attempts">История</AdminTab>
        {isSuper ? <AdminTab to="/admin/users">Пользователи</AdminTab> : null}
      </nav>

      <Outlet context={{ me }} />
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
