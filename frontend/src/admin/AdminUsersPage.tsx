import { FormEvent, useEffect, useState } from "react";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { AdminUser } from "../api/types";

type CredentialReveal = {
  email: string;
  password: string;
  mode: "created" | "reset";
};

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  const [newEmail, setNewEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const [reveal, setReveal] = useState<CredentialReveal | null>(null);
  const [copied, setCopied] = useState(false);

  async function reload() {
    setLoading(true);
    try {
      setUsers(await adminApi.listUsers());
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void reload();
  }, []);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setSubmitting(true);
    try {
      const created = await adminApi.createUser(newEmail.trim());
      setReveal({ email: created.username, password: created.password, mode: "created" });
      setCopied(false);
      setNewEmail("");
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось создать пользователя");
    } finally {
      setSubmitting(false);
    }
  }

  async function onReset(u: AdminUser) {
    if (!confirm(`Сбросить пароль для ${u.username}? Старый пароль перестанет работать.`)) return;
    try {
      const res = await adminApi.resetUserPassword(u.id);
      setReveal({ email: u.username, password: res.password, mode: "reset" });
      setCopied(false);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось сбросить пароль");
    }
  }

  async function onDelete(u: AdminUser) {
    if (
      !confirm(
        `Удалить аккаунт ${u.username}?\nВарианты, которые он создал, останутся, но станут без владельца — управлять ими сможет только суперадмин.`,
      )
    )
      return;
    try {
      await adminApi.deleteUser(u.id);
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось удалить");
    }
  }

  async function copyPassword() {
    if (!reveal) return;
    try {
      await navigator.clipboard.writeText(`${reveal.email} / ${reveal.password}`);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="space-y-6">
      <form onSubmit={onCreate} className="card flex items-end gap-3">
        <div className="flex-1">
          <label className="label">Email нового редактора</label>
          <input
            className="input"
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            placeholder="teacher@school.kz"
            required
          />
        </div>
        <button className="btn-primary" disabled={submitting}>
          {submitting ? "Создание..." : "Создать"}
        </button>
      </form>

      <p className="text-xs text-neutral-500">
        После создания пароль будет показан один раз — обязательно скопируйте и отправьте редактору.
        Восстановить пароль можно только через «Сбросить».
      </p>

      {err ? <p className="text-sm text-red-600">{err}</p> : null}

      {loading ? (
        <p className="text-neutral-500">Загрузка...</p>
      ) : users.length === 0 ? (
        <p className="text-neutral-500">Пока нет пользователей.</p>
      ) : (
        <ul className="space-y-2">
          {users.map((u) => (
            <li key={u.id} className="card flex items-center justify-between gap-3">
              <div>
                <p className="font-medium">{u.username}</p>
                <p className="text-xs text-neutral-500">
                  <span
                    className={`mr-2 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                      u.role === "superadmin"
                        ? "bg-amber-100 text-amber-800"
                        : "bg-neutral-100 text-neutral-700"
                    }`}
                  >
                    {u.role === "superadmin" ? "суперадмин" : "редактор"}
                  </span>
                  создан {new Date(u.createdAt).toLocaleString("ru-RU")}
                </p>
              </div>
              {u.role === "superadmin" ? (
                <span className="text-xs text-neutral-400">— системный, изменения через .env</span>
              ) : (
                <div className="flex gap-2">
                  <button className="btn-secondary" onClick={() => onReset(u)}>
                    Сбросить пароль
                  </button>
                  <button className="btn-danger" onClick={() => onDelete(u)}>
                    Удалить
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      {reveal ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="card w-full max-w-md space-y-4">
            <h3 className="text-lg font-medium">
              {reveal.mode === "created" ? "Пользователь создан" : "Пароль сброшен"}
            </h3>
            <p className="text-sm text-neutral-600">
              Скопируйте логин и пароль сейчас — после закрытия окна пароль больше не показывается.
            </p>
            <div className="rounded-md border border-neutral-200 bg-neutral-50 p-3 font-mono text-sm">
              <div>
                <span className="text-neutral-500">email:</span> {reveal.email}
              </div>
              <div>
                <span className="text-neutral-500">пароль:</span> {reveal.password}
              </div>
            </div>
            <div className="flex items-center justify-end gap-2">
              <button className="btn-secondary" onClick={copyPassword}>
                {copied ? "Скопировано ✓" : "Скопировать"}
              </button>
              <button
                className="btn-primary"
                onClick={() => {
                  setReveal(null);
                  setCopied(false);
                }}
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
