import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useGuest } from "../hooks/useGuest";

export default function Layout() {
  const { guest, setGuest } = useGuest();
  const nav = useNavigate();

  const navClass = ({ isActive }: { isActive: boolean }) =>
    `text-sm transition-colors ${isActive ? "text-neutral-900 font-medium" : "text-neutral-500 hover:text-neutral-900"}`;

  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b border-neutral-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <Link to="/" className="text-base font-semibold tracking-tight">
            MounTest
          </Link>
          <nav className="flex items-center gap-5">
            <NavLink to="/" end className={navClass}>
              Предметы
            </NavLink>
            {guest ? (
              <div className="flex items-center gap-2 text-sm">
                <span className="text-neutral-500">
                  {guest.firstName} {guest.lastName}
                </span>
                <button
                  className="text-neutral-500 hover:text-neutral-900"
                  onClick={() => {
                    setGuest(null);
                    nav("/");
                  }}
                >
                  Выйти
                </button>
              </div>
            ) : null}
          </nav>
        </div>
      </header>
      <main className="flex-1">
        <div className="mx-auto max-w-5xl px-4 py-8">
          <Outlet />
        </div>
      </main>
      <footer className="border-t border-neutral-200 bg-white">
        <div className="mx-auto max-w-5xl px-4 py-4 text-xs text-neutral-500">
          MounTest — тренажёр ЕНТ. MVP.
        </div>
      </footer>
    </div>
  );
}
