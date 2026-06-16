import { useEffect, useState } from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Attempt, Subject, Variant } from "../api/types";
import { useGuest } from "../hooks/useGuest";
import { saveAttemptToken } from "../hooks/attemptTokens";

export default function SubjectPage() {
  const { id } = useParams<{ id: string }>();
  const { guest } = useGuest();
  const nav = useNavigate();
  const [variants, setVariants] = useState<Variant[]>([]);
  const [subject, setSubject] = useState<Subject | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [startingId, setStartingId] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.get<Subject[]>("/subjects"),
      api.get<Variant[]>(`/subjects/${id}/variants`),
    ])
      .then(([subjects, vs]) => {
        setSubject(subjects.find((s) => s.id === id) ?? null);
        setVariants(vs);
      })
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Ошибка загрузки"))
      .finally(() => setLoading(false));
  }, [id]);

  async function startVariant(variantId: string) {
    if (!guest) {
      nav("/");
      return;
    }
    setStartingId(variantId);
    try {
      const att = await api.post<Attempt>("/attempts", {
        variantId,
        guestSessionId: guest.id,
      });
      // Без токена дальше /attempts/{id} вернёт 401 — обязательно сохраняем сразу.
      if (att.attemptToken) {
        saveAttemptToken(att.id, att.attemptToken);
      }
      nav(`/attempts/${att.id}`);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось начать тест");
      setStartingId(null);
    }
  }

  if (loading) return <p className="text-neutral-500">Загрузка...</p>;
  if (err) return <p className="text-red-600">{err}</p>;
  // Предмет без вариантов не попадает в публичный список — убираем прямую ссылку на пустую страницу.
  if (variants.length === 0) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="space-y-6">
      <div>
        <Link to="/" className="text-sm text-neutral-500 hover:text-neutral-900">
          ← Все предметы
        </Link>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">
          {subject ? subject.name : "Предмет"}
        </h1>
      </div>

      {!guest ? (
        <div className="card">
          <p className="text-sm text-neutral-700">
            Чтобы начать вариант, представьтесь на{" "}
            <Link to="/" className="underline">главной странице</Link>.
          </p>
        </div>
      ) : null}

      <ul className="space-y-3">
          {variants.map((v) => (
            <li key={v.id} className="card flex items-center justify-between">
              <div>
                <p className="text-base font-medium">{v.title}</p>
                <p className="text-sm text-neutral-500">
                  {v.topic ? (
                    <>
                      <span>{v.topic}</span>
                      <span className="mx-1.5">·</span>
                    </>
                  ) : null}
                  {v.questionsCount} вопросов · {v.durationMinutes} мин (опц. таймер)
                </p>
              </div>
              <button
                className="btn-primary"
                disabled={!guest || startingId === v.id || v.questionsCount === 0}
                onClick={() => startVariant(v.id)}
              >
                {startingId === v.id ? "Запуск..." : "Начать"}
              </button>
            </li>
          ))}
      </ul>
    </div>
  );
}
