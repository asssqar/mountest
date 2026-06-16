import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { api, ApiError, resolveImageUrl, withAttemptToken } from "../api/client";
import type { AttemptResult, ReviewEntry, ReviewStatus } from "../api/types";
import NoCopy from "../components/NoCopy";
import { getAttemptToken } from "../hooks/attemptTokens";
import { useProtectTestContent } from "../hooks/useProtectTestContent";

type Filter = "all" | "errors";

export default function ResultPage() {
  useProtectTestContent();
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const initial = (location.state as { result?: AttemptResult } | null)?.result ?? null;

  const [result, setResult] = useState<AttemptResult | null>(initial);
  const [loading, setLoading] = useState(!initial);
  const [err, setErr] = useState<string | null>(null);
  const [filter, setFilter] = useState<Filter>("all");

  useEffect(() => {
    if (!id || result) return;
    const token = getAttemptToken(id);
    if (!token) {
      setErr("Эта попытка не привязана к этому браузеру.");
      setLoading(false);
      return;
    }
    api
      .get<AttemptResult>(`/attempts/${id}/result`, withAttemptToken(token))
      .then(setResult)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Не удалось загрузить результат"))
      .finally(() => setLoading(false));
  }, [id, result]);

  const counts = useMemo(() => {
    const acc = { correct: 0, incorrect: 0, unanswered: 0 };
    if (!result) return acc;
    for (const r of result.review) acc[r.status] += 1;
    return acc;
  }, [result]);

  const visible = useMemo(() => {
    if (!result) return [] as ReviewEntry[];
    if (filter === "all") return result.review;
    return result.review.filter((r) => r.status !== "correct");
  }, [result, filter]);

  if (loading) return <p className="text-neutral-500">Загрузка...</p>;
  if (err) return <p className="text-red-600">{err}</p>;
  if (!result) return null;

  const percent = result.total === 0 ? 0 : Math.round((result.score / result.total) * 100);
  const errorsTotal = counts.incorrect + counts.unanswered;

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

      <div className="card grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Stat label="Балл" value={`${result.score} / ${result.total}`} />
        <Stat label="Процент" value={`${percent}%`} />
        <Stat label="Ошибки" value={String(counts.incorrect)} tone="red" />
        <Stat label="Без ответа" value={String(counts.unanswered)} tone="amber" />
      </div>

      {result.review.length > 0 ? (
        <section className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-lg font-medium">Разбор вопросов</h2>
            <div className="inline-flex rounded-md border border-neutral-200 bg-white p-0.5 text-sm">
              <FilterTab active={filter === "all"} onClick={() => setFilter("all")}>
                Все ({result.review.length})
              </FilterTab>
              <FilterTab
                active={filter === "errors"}
                onClick={() => setFilter("errors")}
                disabled={errorsTotal === 0}
              >
                Только ошибки ({errorsTotal})
              </FilterTab>
            </div>
          </div>

          {visible.length === 0 ? (
            <div className="card">
              <p className="text-base font-medium">
                {filter === "errors"
                  ? "Ошибок нет — всё верно!"
                  : "Здесь пусто."}
              </p>
            </div>
          ) : (
            <NoCopy>
              <ul className="space-y-3">
                {visible.map((entry) => (
                  <ReviewCard key={entry.questionId} entry={entry} />
                ))}
              </ul>
            </NoCopy>
          )}
        </section>
      ) : null}
    </div>
  );
}

function Stat({
  label,
  value,
  tone = "neutral",
}: {
  label: string;
  value: string;
  tone?: "neutral" | "red" | "amber";
}) {
  const valueClass =
    tone === "red"
      ? "text-red-700"
      : tone === "amber"
        ? "text-amber-700"
        : "text-neutral-900";
  return (
    <div>
      <p className="text-xs uppercase tracking-wide text-neutral-500">{label}</p>
      <p className={`text-2xl font-semibold ${valueClass}`}>{value}</p>
    </div>
  );
}

function FilterTab({
  active,
  disabled,
  onClick,
  children,
}: {
  active: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      aria-pressed={active}
      className={[
        "rounded-md px-3 py-1.5 transition-colors",
        active
          ? "bg-neutral-900 text-white"
          : "text-neutral-600 hover:text-neutral-900",
        disabled ? "cursor-not-allowed opacity-50 hover:text-neutral-600" : "",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

function StatusBadge({ status }: { status: ReviewStatus }) {
  if (status === "correct") {
    return (
      <span className="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800">
        Правильно
      </span>
    );
  }
  if (status === "unanswered") {
    return (
      <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800">
        Без ответа
      </span>
    );
  }
  return (
    <span className="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-800">
      Неправильно
    </span>
  );
}

function ReviewCard({ entry }: { entry: ReviewEntry }) {
  const correctSet = new Set(entry.correctOptionIds);
  const selectedSet = new Set(entry.selectedOptionIds);

  return (
    <li className="card space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs text-neutral-500">Вопрос {entry.position}</p>
          <p className="whitespace-pre-wrap font-medium">{entry.questionText}</p>
        </div>
        <StatusBadge status={entry.status} />
      </div>

      {entry.questionImageUrl ? (
        <img
          src={resolveImageUrl(entry.questionImageUrl) ?? ""}
          alt={entry.questionText}
          draggable={false}
          className="max-h-80 w-full rounded-md border border-neutral-200 bg-white object-contain"
        />
      ) : null}

      <ul className="space-y-1.5">
        {entry.options.map((o) => {
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
              <span className="mt-0.5 inline-flex w-24 shrink-0 items-center gap-1 text-xs font-medium uppercase tracking-wide">
                {labelFor(isCorrect, isSelected)}
              </span>
              <span className="whitespace-pre-wrap">{o.text}</span>
            </li>
          );
        })}
      </ul>
    </li>
  );
}

function labelFor(isCorrect: boolean, isSelected: boolean) {
  if (isCorrect && isSelected) return <span className="text-emerald-700">верно ✓</span>;
  if (isCorrect && !isSelected) return <span className="text-emerald-700">пропущено</span>;
  if (!isCorrect && isSelected) return <span className="text-red-700">ваш выбор</span>;
  return <span className="text-neutral-400">—</span>;
}
