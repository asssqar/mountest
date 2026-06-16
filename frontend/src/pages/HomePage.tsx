import { useEffect, useState, FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Guest, Subject } from "../api/types";
import { useGuest } from "../hooks/useGuest";

export default function HomePage() {
  const { guest, setGuest } = useGuest();
  const nav = useNavigate();
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [loading, setLoading] = useState(true);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<Subject[]>("/subjects")
      .then(setSubjects)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Не удалось загрузить предметы"))
      .finally(() => setLoading(false));
  }, []);

  async function startGuest(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setSubmitting(true);
    try {
      const g = await api.post<Guest>("/guest-sessions", {
        firstName: firstName.trim(),
        lastName: lastName.trim(),
      });
      setGuest(g);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось создать сессию");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-8">
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">
          MounTest — тренажёр ЕНТ
        </h1>
        <p className="text-neutral-600">
          Решайте варианты по предметам, проверяйте свои ошибки и тренируйтесь без регистрации.
        </p>
      </section>

      {!guest ? (
        <section className="card max-w-md">
          <h2 className="mb-3 text-lg font-medium">Кто проходит тест?</h2>
          <form onSubmit={startGuest} className="space-y-3">
            <div>
              <label className="label">Имя</label>
              <input
                className="input"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                required
                placeholder="Ваше имя"
              />
            </div>
            <div>
              <label className="label">Фамилия</label>
              <input
                className="input"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                required
                placeholder="Ваша фамилия"
              />
            </div>
            {err ? <p className="text-sm text-red-600">{err}</p> : null}
            <button className="btn-primary" disabled={submitting}>
              {submitting ? "Подождите..." : "Начать"}
            </button>
          </form>
        </section>
      ) : (
        <section className="card flex items-center justify-between">
          <div>
            <p className="text-sm text-neutral-500">Вы вошли как</p>
            <p className="text-base font-medium">
              {guest.firstName} {guest.lastName}
            </p>
          </div>
          <button
            className="btn-secondary"
            onClick={() => {
              setGuest(null);
              nav("/");
            }}
          >
            Сменить пользователя
          </button>
        </section>
      )}

      <section>
        <h2 className="mb-3 text-lg font-medium">Предметы</h2>
        {loading ? (
          <p className="text-neutral-500">Загрузка...</p>
        ) : subjects.length === 0 ? (
          <p className="text-neutral-500">
            Пока нет доступных предметов — как только появятся варианты, они отобразятся здесь.
          </p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2">
            {subjects.map((s) => (
              <li key={s.id}>
                <Link
                  to={`/subjects/${s.id}`}
                  className="card block transition-shadow hover:shadow-md"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-base font-medium">{s.name}</span>
                    <span className="text-sm text-neutral-500">
                      {s.variantsCount}{" "}
                      {pluralVariants(s.variantsCount)}
                    </span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function pluralVariants(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return "вариант";
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return "варианта";
  return "вариантов";
}
