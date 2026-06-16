import { useEffect, useMemo, useState } from "react";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { AdminAttemptRow, Variant } from "../api/types";

const PAGE_SIZE = 50;

type StatusFilter = "all" | "finished" | "active";

export default function AdminAttemptsPage() {
  const [variants, setVariants] = useState<Variant[]>([]);
  const [items, setItems] = useState<AdminAttemptRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [variantId, setVariantId] = useState<string>("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  // Список вариантов нужен только для селекта-фильтра. Грузим один раз.
  useEffect(() => {
    adminApi
      .listVariants()
      .then(setVariants)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Не удалось загрузить варианты"));
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    adminApi
      .listAttempts({ variantId: variantId || undefined, status, limit: PAGE_SIZE, offset })
      .then((page) => {
        if (cancelled) return;
        setItems(page.items);
        setTotal(page.total);
      })
      .catch((e) => {
        if (cancelled) return;
        setErr(e instanceof ApiError ? e.message : "Не удалось загрузить историю");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [variantId, status, offset]);

  // При смене фильтра возвращаемся на первую страницу.
  function changeVariant(id: string) {
    setVariantId(id);
    setOffset(0);
  }
  function changeStatus(s: StatusFilter) {
    setStatus(s);
    setOffset(0);
  }

  const pageInfo = useMemo(() => {
    if (total === 0) return { from: 0, to: 0, page: 1, pages: 1 };
    const from = offset + 1;
    const to = Math.min(offset + items.length, total);
    const page = Math.floor(offset / PAGE_SIZE) + 1;
    const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    return { from, to, page, pages };
  }, [offset, items.length, total]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-64">
          <label className="label">Вариант</label>
          <select
            className="input"
            value={variantId}
            onChange={(e) => changeVariant(e.target.value)}
          >
            <option value="">Все варианты</option>
            {variants.map((v) => (
              <option key={v.id} value={v.id}>
                {v.title}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="label">Статус</label>
          <select
            className="input"
            value={status}
            onChange={(e) => changeStatus(e.target.value as StatusFilter)}
          >
            <option value="all">Все</option>
            <option value="finished">Завершённые</option>
            <option value="active">В процессе</option>
          </select>
        </div>
        <div className="text-sm text-neutral-500">
          {loading ? "Загрузка..." : `Найдено: ${total}`}
        </div>
      </div>

      {err ? <p className="text-sm text-red-600">{err}</p> : null}

      {!loading && items.length === 0 ? (
        <div className="card">
          <p className="text-neutral-500">Пока нет попыток.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-neutral-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500">
              <tr>
                <th className="px-3 py-2">Ученик</th>
                <th className="px-3 py-2">Предмет / вариант</th>
                <th className="px-3 py-2 whitespace-nowrap">Балл</th>
                <th className="px-3 py-2 whitespace-nowrap">Начата</th>
                <th className="px-3 py-2 whitespace-nowrap">Длительность</th>
                <th className="px-3 py-2">Статус</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <AttemptsRow key={it.id} row={it} />
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex items-center justify-between">
        <p className="text-xs text-neutral-500">
          {total === 0
            ? "—"
            : `Показано ${pageInfo.from}–${pageInfo.to} из ${total} · стр. ${pageInfo.page} из ${pageInfo.pages}`}
        </p>
        <div className="flex gap-2">
          <button
            className="btn-secondary"
            disabled={offset === 0 || loading}
            onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
          >
            ← Назад
          </button>
          <button
            className="btn-secondary"
            disabled={offset + items.length >= total || loading}
            onClick={() => setOffset((o) => o + PAGE_SIZE)}
          >
            Вперёд →
          </button>
        </div>
      </div>
    </div>
  );
}

function AttemptsRow({ row }: { row: AdminAttemptRow }) {
  const finished = row.finishedAt != null;
  const percent =
    row.score != null && row.total != null && row.total > 0
      ? Math.round((row.score / row.total) * 100)
      : null;

  return (
    <tr className="border-t border-neutral-100">
      <td className="px-3 py-2">
        <div className="font-medium">
          {row.guest.firstName} {row.guest.lastName}
        </div>
      </td>
      <td className="px-3 py-2">
        <div>{row.subject.name}</div>
        <div className="text-xs text-neutral-500">
          {row.variant.title}
          {row.variant.topic ? ` · ${row.variant.topic}` : ""}
        </div>
      </td>
      <td className="px-3 py-2 whitespace-nowrap">
        {row.score != null && row.total != null ? (
          <span className="font-medium">
            {row.score} / {row.total}
            {percent != null ? (
              <span className="ml-1 text-xs text-neutral-500">({percent}%)</span>
            ) : null}
          </span>
        ) : (
          <span className="text-neutral-400">—</span>
        )}
      </td>
      <td className="px-3 py-2 whitespace-nowrap text-neutral-600">
        {formatDateTime(row.startedAt)}
      </td>
      <td className="px-3 py-2 whitespace-nowrap text-neutral-600">
        {formatDuration(row.startedAt, row.finishedAt)}
      </td>
      <td className="px-3 py-2">
        {finished ? (
          <span className="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800">
            завершена
          </span>
        ) : (
          <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800">
            идёт
          </span>
        )}
      </td>
    </tr>
  );
}

function formatDateTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString("ru-RU", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

function formatDuration(start: string, end: string | null): string {
  if (!end) return "—";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "—";
  const totalSec = Math.round(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}ч ${m}м`;
  if (m > 0) return `${m}м ${s}с`;
  return `${s}с`;
}
