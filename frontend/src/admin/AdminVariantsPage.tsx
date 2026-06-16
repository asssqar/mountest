import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { AdminMe, Subject, Variant } from "../api/types";

type FormState = {
  id: string | null;
  subjectId: string;
  title: string;
  topic: string;
  durationMinutes: number;
};

const emptyForm: FormState = {
  id: null,
  subjectId: "",
  title: "",
  topic: "",
  durationMinutes: 60,
};

export default function AdminVariantsPage() {
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [variants, setVariants] = useState<Variant[]>([]);
  const [me, setMe] = useState<AdminMe | null>(null);
  const [filter, setFilter] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);

  async function reload() {
    setLoading(true);
    try {
      const [s, v, m] = await Promise.all([
        adminApi.listSubjects(),
        adminApi.listVariants(filter || undefined),
        adminApi.me(),
      ]);
      setSubjects(s);
      setVariants(v);
      setMe(m);
      if (!form.subjectId && s.length > 0) {
        setForm((f) => ({ ...f, subjectId: s[0].id }));
      }
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ошибка загрузки");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!form.subjectId || !form.title.trim()) {
      setErr("Заполните предмет и название");
      return;
    }
    try {
      if (form.id) {
        await adminApi.updateVariant(form.id, {
          subjectId: form.subjectId,
          title: form.title.trim(),
          topic: form.topic.trim(),
          durationMinutes: form.durationMinutes,
        });
      } else {
        await adminApi.createVariant({
          subjectId: form.subjectId,
          title: form.title.trim(),
          topic: form.topic.trim(),
          durationMinutes: form.durationMinutes,
        });
      }
      setForm({ ...emptyForm, subjectId: form.subjectId });
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось сохранить");
    }
  }

  async function onDelete(id: string) {
    if (!confirm("Удалить вариант со всеми вопросами?")) return;
    try {
      await adminApi.deleteVariant(id);
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось удалить");
    }
  }

  async function onTogglePublish(v: Variant) {
    const next = !v.isPublished;
    // Оптимистично переключаем — если бэкенд откажет, откатим в catch.
    setVariants((vs) => vs.map((x) => (x.id === v.id ? { ...x, isPublished: next } : x)));
    try {
      await adminApi.setVariantPublished(v.id, next);
    } catch (e) {
      setVariants((vs) => vs.map((x) => (x.id === v.id ? { ...x, isPublished: !next } : x)));
      setErr(e instanceof ApiError ? e.message : "Не удалось изменить видимость");
    }
  }

  return (
    <div className="space-y-6">
      <form onSubmit={onSubmit} className="card grid gap-3 md:grid-cols-4">
        <div>
          <label className="label">Предмет</label>
          <select
            className="input"
            value={form.subjectId}
            onChange={(e) => setForm({ ...form, subjectId: e.target.value })}
          >
            <option value="">—</option>
            {subjects.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="label">Название теста</label>
          <input
            className="input"
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            placeholder="Напр., Теория №1"
          />
        </div>
        <div>
          <label className="label">Тема (необязательно)</label>
          <input
            className="input"
            value={form.topic}
            onChange={(e) => setForm({ ...form, topic: e.target.value })}
            placeholder="Напр., Файловые системы"
          />
        </div>
        <div>
          <label className="label">Длительность, мин</label>
          <input
            type="number"
            min={1}
            className="input"
            value={form.durationMinutes}
            onChange={(e) =>
              setForm({ ...form, durationMinutes: Number(e.target.value) || 60 })
            }
          />
        </div>
        <div className="md:col-span-4 flex items-center gap-2">
          <button className="btn-primary">{form.id ? "Сохранить" : "Создать"}</button>
          {form.id ? (
            <button
              type="button"
              className="btn-ghost"
              onClick={() => setForm({ ...emptyForm, subjectId: form.subjectId })}
            >
              Отмена
            </button>
          ) : null}
        </div>
      </form>

      <div className="flex items-end gap-3">
        <div>
          <label className="label">Фильтр по предмету</label>
          <select
            className="input"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          >
            <option value="">Все</option>
            {subjects.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {err ? <p className="text-sm text-red-600">{err}</p> : null}

      {loading ? (
        <p className="text-neutral-500">Загрузка...</p>
      ) : variants.length === 0 ? (
        <p className="text-neutral-500">Вариантов нет.</p>
      ) : (
        <ul className="space-y-2">
          {variants.map((v) => {
            const subj = subjects.find((s) => s.id === v.subjectId);
            const published = v.isPublished !== false;
            return (
              <li key={v.id} className="card flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="font-medium">{v.title}</p>
                    <span
                      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                        published
                          ? "bg-emerald-100 text-emerald-800"
                          : "bg-neutral-200 text-neutral-700"
                      }`}
                    >
                      {published ? "опубликован" : "скрыт"}
                    </span>
                  </div>
                  <p className="text-xs text-neutral-500">
                    {v.topic ? `${v.topic} · ` : ""}
                    {subj?.name ?? "—"} · {v.questionsCount} вопросов · {v.durationMinutes} мин
                    {me?.role === "superadmin" && v.createdByUsername
                      ? ` · автор: ${v.createdByUsername}`
                      : null}
                  </p>
                </div>
                <div className="flex flex-wrap justify-end gap-2">
                  <Link
                    className="btn-secondary"
                    to={`/admin/questions?variantId=${v.id}`}
                  >
                    Вопросы
                  </Link>
                  <button
                    className="btn-secondary"
                    onClick={() => void onTogglePublish(v)}
                    title={
                      published
                        ? "Скрыть из публичного каталога"
                        : "Показать в публичном каталоге"
                    }
                  >
                    {published ? "Скрыть" : "Опубликовать"}
                  </button>
                  <button
                    className="btn-secondary"
                    onClick={() =>
                      setForm({
                        id: v.id,
                        subjectId: v.subjectId,
                        title: v.title,
                        topic: v.topic ?? "",
                        durationMinutes: v.durationMinutes,
                      })
                    }
                  >
                    Изменить
                  </button>
                  <button className="btn-danger" onClick={() => onDelete(v.id)}>
                    Удалить
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
