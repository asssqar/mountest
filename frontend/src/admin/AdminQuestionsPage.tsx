import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { adminApi } from "./AdminApi";
import { ApiError, resolveImageUrl } from "../api/client";
import type { Question, Variant } from "../api/types";

type OptionDraft = { id?: string; text: string; isCorrect: boolean };

type QuestionForm = {
  id: string | null;
  text: string;
  position: number;
  imageUrl: string | null;
  options: OptionDraft[];
};

const emptyForm: QuestionForm = {
  id: null,
  text: "",
  position: 0,
  imageUrl: null,
  options: [
    { text: "", isCorrect: false },
    { text: "", isCorrect: false },
  ],
};

const importExample = `1. Файловая система подразделяется на
* внутреннюю и внешнюю
* простую и сложную
* простую и иерархическую
* динамическую и статическую

2. Процесс, в результате которого в несколько раз уменьшается размер файлов
* сжатие информации
* распаковка
* разархивация
* удаление информации`;

export default function AdminQuestionsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const variantId = searchParams.get("variantId") ?? "";

  const [variants, setVariants] = useState<Variant[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [form, setForm] = useState<QuestionForm>(emptyForm);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [importText, setImportText] = useState("");
  const [importReplace, setImportReplace] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importOk, setImportOk] = useState<string | null>(null);
  const importFileRef = useRef<HTMLInputElement>(null);

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
      imageUrl: q.imageUrl ?? null,
      options: q.options.map((o) => ({ id: o.id, text: o.text, isCorrect: o.isCorrect })),
    });
  }

  async function onPickImage(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 5 * 1024 * 1024) {
      setErr("Файл слишком большой (макс. 5 МБ)");
      e.target.value = "";
      return;
    }
    setUploading(true);
    setErr(null);
    try {
      const { url } = await adminApi.uploadImage(file);
      setForm((f) => ({ ...f, imageUrl: url }));
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось загрузить картинку");
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  function clearImage() {
    setForm((f) => ({ ...f, imageUrl: null }));
  }

  async function onImportFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      setImportText(text);
      setImportOk(null);
    } catch {
      setErr("Не удалось прочитать файл");
    }
    if (importFileRef.current) importFileRef.current.value = "";
  }

  async function onImportSubmit() {
    if (!variantId) {
      setErr("Сначала выберите вариант");
      return;
    }
    if (!importText.trim()) {
      setErr("Вставьте текст вопросов или загрузите файл");
      return;
    }
    if (importReplace && !confirm("Удалить все текущие вопросы варианта и загрузить новые?")) {
      return;
    }
    setImporting(true);
    setErr(null);
    setImportOk(null);
    try {
      const res = await adminApi.importQuestions(variantId, importText, importReplace);
      setImportOk(`Импортировано вопросов: ${res.imported}`);
      setImportText("");
      await reloadQuestions();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось импортировать");
    } finally {
      setImporting(false);
    }
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
          imageUrl: form.imageUrl,
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
          imageUrl: form.imageUrl,
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
          <div className="card space-y-4">
            <h2 className="text-base font-medium">Импорт вопросов</h2>
            <p className="text-sm text-neutral-600">
              Вставьте текст или загрузите файл <code className="text-xs">.txt</code> /{" "}
              <code className="text-xs">.json</code>. Вопросы добавятся к выбранному варианту.
            </p>
            <details className="text-sm text-neutral-600">
              <summary className="cursor-pointer font-medium text-neutral-800">Формат текста</summary>
              <pre className="mt-2 overflow-x-auto rounded-md bg-neutral-50 p-3 text-xs whitespace-pre-wrap">
                {importExample}
              </pre>
              <ul className="mt-2 list-inside list-disc space-y-1 text-xs">
                <li>
                  Вопрос: <code>1.</code> или <code>1)</code> в начале строки
                </li>
                <li>
                  Ответы: строки с <code>*</code> — все варианты переносятся; правильные отметьте
                  вручную после импорта (кнопка «Изменить»)
                </li>
                <li>
                  Опционально: <code>+ текст</code> — сразу правильный ответ при импорте
                </li>
                <li>Пустая строка между вопросами — необязательна</li>
              </ul>
            </details>
            <textarea
              className="input min-h-48 font-mono text-sm"
              value={importText}
              onChange={(e) => {
                setImportText(e.target.value);
                setImportOk(null);
              }}
              placeholder={importExample}
            />
            <div className="flex flex-wrap items-center gap-3">
              <input
                ref={importFileRef}
                type="file"
                accept=".txt,.json,text/plain,application/json"
                onChange={onImportFile}
                className="text-sm file:mr-2 file:rounded-md file:border file:border-neutral-200 file:bg-white file:px-3 file:py-1.5 file:text-sm"
              />
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="h-4 w-4 accent-neutral-900"
                  checked={importReplace}
                  onChange={(e) => setImportReplace(e.target.checked)}
                />
                Заменить все вопросы варианта
              </label>
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                className="btn-primary"
                disabled={importing}
                onClick={() => void onImportSubmit()}
              >
                {importing ? "Импорт..." : "Импортировать"}
              </button>
              {importOk ? <span className="text-sm text-emerald-700">{importOk}</span> : null}
            </div>
          </div>

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
              <label className="label">Картинка (необязательно)</label>
              {form.imageUrl ? (
                <div className="flex items-start gap-3">
                  <img
                    src={resolveImageUrl(form.imageUrl) ?? ""}
                    alt="Превью"
                    className="max-h-48 rounded-md border border-neutral-200 bg-white object-contain"
                  />
                  <button type="button" className="btn-ghost text-red-600" onClick={clearImage}>
                    Удалить картинку
                  </button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    onChange={onPickImage}
                    disabled={uploading}
                    className="text-sm file:mr-2 file:rounded-md file:border file:border-neutral-200 file:bg-white file:px-3 file:py-1.5 file:text-sm file:font-medium hover:file:bg-neutral-50"
                  />
                  {uploading ? (
                    <span className="text-xs text-neutral-500">Загрузка...</span>
                  ) : (
                    <span className="text-xs text-neutral-500">JPG, PNG или WebP, до 5 МБ</span>
                  )}
                </div>
              )}
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
                    {q.imageUrl ? (
                      <img
                        src={resolveImageUrl(q.imageUrl) ?? ""}
                        alt=""
                        className="max-h-40 rounded-md border border-neutral-200 bg-white object-contain"
                      />
                    ) : null}
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
