import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";

export default function AdminLoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const nav = useNavigate();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setSubmitting(true);
    try {
      await adminApi.login(username.trim(), password);
      nav("/admin", { replace: true });
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ошибка входа");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="mb-4 text-2xl font-semibold tracking-tight">Вход в админку</h1>
      <form onSubmit={onSubmit} className="card space-y-3">
        <div>
          <label className="label">Логин</label>
          <input
            className="input"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label">Пароль</label>
          <input
            type="password"
            className="input"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        {err ? <p className="text-sm text-red-600">{err}</p> : null}
        <button className="btn-primary w-full" disabled={submitting}>
          {submitting ? "Вход..." : "Войти"}
        </button>
      </form>
    </div>
  );
}
