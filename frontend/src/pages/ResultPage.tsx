import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { AttemptResult } from "../api/types";

export default function ResultPage() {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const initial = (location.state as { result?: AttemptResult } | null)?.result ?? null;

  const [result, setResult] = useState<AttemptResult | null>(initial);
  const [loading, setLoading] = useState(!initial);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!id || result) return;
    api
      .get<AttemptResult>(`/attempts/${id}/result`)
      .then(setResult)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Не удалось загрузить результат"))
      .finally(() => setLoading(false));
  }, [id, result]);

  if (loading) return <p className="text-neutral-500">Загрузка...</p>;
  if (err) return <p className="text-red-600">{err}</p>;
  if (!result) return null;

  const percent = result.total === 0 ? 0 : Math.round((result.score / result.total) * 100);

  return (
    <div className="space-y-6">
      <div>
        <Link to="/" className="text-sm text-neutral-500 hover:text-neutral-900">
          ← На главную
        </Link>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Результат</h1>
        {result.guest ? (
          <p className="text-sm text-neutral-500">
            {result.guest.firstName} {result.guest.lastName}
          </p>
        ) : null}
      </div>

      <div className="card flex items-center justify-between">
        <div>
          <p className="text-sm text-neutral-500">Балл</p>
          <p className="text-3xl font-semibold">
            {result.score} / {result.total}
          </p>
        </div>
        <div className="text-right">
          <p className="text-sm text-neutral-500">Процент</p>
          <p className="text-3xl font-semibold">{percent}%</p>
        </div>
      </div>

      {result.errors.length === 0 ? (
        <div className="card">
          <p className="text-base font-medium">Без ошибок! Отличный результат.</p>
        </div>
      ) : (
        <section className="space-y-4">
          <h2 className="text-lg font-medium">Ошибки</h2>
          <ul className="space-y-3">
            {result.errors.map((e, idx) => {
              const correctSet = new Set(e.correctOptionIds);
              const selectedSet = new Set(e.selectedOptionIds);
              return (
                <li key={e.questionId} className="card space-y-3">
                  <p className="text-sm text-neutral-500">Вопрос {idx + 1}</p>
                  <p className="whitespace-pre-wrap font-medium">{e.questionText}</p>
                  <ul className="space-y-1.5">
                    {e.options.map((o) => {
                      const isCorrect = correctSet.has(o.id);
                      const isSelected = selectedSet.has(o.id);
                      return (
                        <li
                          key={o.id}
                          className={[
                            "flex items-start gap-2 rounded-md border px-3 py-2 text-sm",
                            isCorrect
                              ? "border-emerald-300 bg-emerald-50"
                              : isSelected
                                ? "border-red-300 bg-red-50"
                                : "border-neutral-200",
                          ].join(" ")}
                        >
                          <span className="mt-0.5 inline-flex w-20 shrink-0 items-center gap-1 text-xs font-medium uppercase tracking-wide">
                            {isCorrect ? (
                              <span className="text-emerald-700">верно</span>
                            ) : isSelected ? (
                              <span className="text-red-700">ваш выбор</span>
                            ) : (
                              <span className="text-neutral-400">—</span>
                            )}
                          </span>
                          <span className="whitespace-pre-wrap">{o.text}</span>
                        </li>
                      );
                    })}
                  </ul>
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </div>
  );
}
