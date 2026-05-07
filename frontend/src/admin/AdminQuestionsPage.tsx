import { FormEvent, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { Question, Variant } from "../api/types";

type OptionDraft = { id?: string; text: string; isCorrect: boolean };

type QuestionForm = {
  id: string | null;
  text: string;
  position: number;
  options: OptionDraft[];
};

const emptyForm: QuestionForm = {
  id: null,
  text: "",
  position: 0,
  options: [
    { text: "", isCorrect: false },
    { text: "", isCorrect: false },
  ],
};

export default function AdminQuestionsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const variantId = searchParams.get("variantId") ?? "";

  const [variants, setVariants] = useState<Variant[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [form, setForm] = useState<QuestionForm>(emptyForm);

  useEffect(() => {
    adminApi
      .listVariants()
      .then(setVariants)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Ошибка"));
  }, []);

  async function reloadQuestions() {
    if (!variantId) {
      setQuestions([]);
      return;
    }
    setLoading(true);
    try {
      setQuestions(await adminApi.listQuestions(variantId));
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void reloadQuestions();
    setForm(emptyForm);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [variantId]);

  const variant = useMemo(
    () => variants.find((v) => v.id === variantId),
    [variants, variantId],
  );

  function selectVariant(id: string) {
    if (id) setSearchParams({ variantId: id });
    else setSearchParams({});
  }

  function startEdit(q: Question) {
    setForm({
      id: q.id,
      text: q.text,
      position: q.position,
      options: q.options.map((o) => ({ id: o.id, text: o.text, isCorrect: o.isCorrect })),
    });
  }

  async function onDelete(id: string) {
    if (!confirm("Удалить вопрос?")) return;
    try {
      await adminApi.deleteQuestion(id);
      await reloadQuestions();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось удалить");
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!variantId) {
      setErr("Сначала выберите вариант");
      return;
    }
    if (!form.text.trim()) {
      setErr("Введите текст вопроса");
      return;
    }
    if (form.options.length < 2) {
      setErr("Нужно минимум 2 варианта ответа");
      return;
    }
    if (form.options.some((o) => !o.text.trim())) {
      setErr("Все варианты должны иметь текст");
      return;
    }
    if (!form.options.some((o) => o.isCorrect)) {
      setErr("Отметьте хотя бы один правильный ответ");
      return;
    }
    try {
      if (form.id) {
        await adminApi.updateQuestion(form.id, {
          variantId,
          text: form.text.trim(),
          position: form.position,
          options: form.options.map((o) => ({
            id: o.id,
            text: o.text.trim(),
            isCorrect: o.isCorrect,
          })),
        });
      } else {
        await adminApi.createQuestion({
          variantId,
          text: form.text.trim(),
          options: form.options.map((o) => ({ text: o.text.trim(), isCorrect: o.isCorrect })),
        });
      }
      setForm(emptyForm);
      await reloadQuestions();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось сохранить");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-64">
          <label className="label">Вариант</label>
          <select
            className="input"
            value={variantId}
            onChange={(e) => selectVariant(e.target.value)}
          >
            <option value="">— выберите вариант —</option>
            {variants.map((v) => (
              <option key={v.id} value={v.id}>
                {v.title}
              </option>
            ))}
          </select>
        </div>
        {variant ? (
          <p className="text-sm text-neutral-500">
            {variant.questionsCount} вопросов · {variant.durationMinutes} мин
          </p>
        ) : null}
      </div>

      {err ? <p className="text-sm text-red-600">{err}</p> : null}

      {variantId ? (
        <>
          <form onSubmit={onSubmit} className="card space-y-4">
            <h2 className="text-base font-medium">
              {form.id ? "Редактировать вопрос" : "Новый вопрос"}
            </h2>
            <div>
              <label className="label">Текст вопроса</label>
              <textarea
                className="input min-h-24"
                value={form.text}
                onChange={(e) => setForm({ ...form, text: e.target.value })}
              />
            </div>
            <div>
              <div className="mb-2 flex items-center justify-between">
                <span className="label mb-0">Варианты ответа</span>
                <button
                  type="button"
                  className="btn-ghost"
                  onClick={() =>
                    setForm({
                      ...form,
                      options: [...form.options, { text: "", isCorrect: false }],
                    })
                  }
                >
                  + Добавить вариант
                </button>
              </div>
              <ul className="space-y-2">
                {form.options.map((opt, idx) => (
                  <li
                    key={idx}
                    className="flex items-start gap-2 rounded-md border border-neutral-200 bg-white p-2"
                  >
                    <label className="mt-2 flex items-center gap-1 text-xs">
                      <input
                        type="checkbox"
                        className="h-4 w-4 accent-neutral-900"
                        checked={opt.isCorrect}
                        onChange={(e) => {
                          const options = [...form.options];
                          options[idx] = { ...opt, isCorrect: e.target.checked };
                          setForm({ ...form, options });
                        }}
                      />
                      верно
                    </label>
                    <input
                      className="input flex-1"
                      value={opt.text}
                      onChange={(e) => {
                        const options = [...form.options];
                        options[idx] = { ...opt, text: e.target.value };
                        setForm({ ...form, options });
                      }}
                      placeholder={`Вариант ${idx + 1}`}
                    />
                    <button
                      type="button"
                      className="btn-ghost text-red-600"
                      disabled={form.options.length <= 2}
                      onClick={() => {
                        const options = form.options.filter((_, i) => i !== idx);
                        setForm({ ...form, options });
                      }}
                    >
                      ×
                    </button>
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-xs text-neutral-500">
                Засчитывается только при точном совпадении выбранного множества с правильным.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button className="btn-primary">{form.id ? "Сохранить" : "Создать"}</button>
              {form.id ? (
                <button
                  type="button"
                  className="btn-ghost"
                  onClick={() => setForm(emptyForm)}
                >
                  Отмена
                </button>
              ) : null}
            </div>
          </form>

          <div>
            <h2 className="mb-3 text-base font-medium">Вопросы варианта</h2>
            {loading ? (
              <p className="text-neutral-500">Загрузка...</p>
            ) : questions.length === 0 ? (
              <p className="text-neutral-500">Пока нет вопросов.</p>
            ) : (
              <ul className="space-y-2">
                {questions.map((q, idx) => (
                  <li key={q.id} className="card space-y-2">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-xs text-neutral-500">№ {idx + 1} · позиция {q.position}</p>
                        <p className="font-medium">{q.text}</p>
                      </div>
                      <div className="flex gap-2">
                        <button className="btn-secondary" onClick={() => startEdit(q)}>
                          Изменить
                        </button>
                        <button className="btn-danger" onClick={() => onDelete(q.id)}>
                          Удалить
                        </button>
                      </div>
                    </div>
                    <ul className="space-y-1 text-sm">
                      {q.options.map((o) => (
                        <li key={o.id} className="flex items-center gap-2">
                          <span
                            className={`inline-block h-2 w-2 rounded-full ${
                              o.isCorrect ? "bg-emerald-500" : "bg-neutral-300"
                            }`}
                          />
                          <span>{o.text}</span>
                        </li>
                      ))}
                    </ul>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </>
      ) : (
        <p className="text-neutral-500">Выберите вариант, чтобы управлять вопросами.</p>
      )}
    </div>
  );
}
