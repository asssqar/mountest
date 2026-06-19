import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api, ApiError, resolveImageUrl, withAttemptToken } from "../api/client";
import type { Attempt, AttemptResult } from "../api/types";
import NoCopy from "../components/NoCopy";
import { getAttemptToken, clearActiveAttempt } from "../hooks/attemptTokens";
import { useProtectTestContent } from "../hooks/useProtectTestContent";

export default function AttemptPage() {
  useProtectTestContent();
  const { id } = useParams<{ id: string }>();
  const nav = useNavigate();

  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [activeIdx, setActiveIdx] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string[]>>({});
  const [savingMap, setSavingMap] = useState<Record<string, boolean>>({});
  const [submitting, setSubmitting] = useState(false);

  const [timerEnabled, setTimerEnabled] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(0);
  const finishedRef = useRef(false);

  useEffect(() => {
    if (!id) return;
    const token = getAttemptToken(id);
    if (!token) {
      // Без токена бэкенд вернёт 401 — пусть пользователь начнёт попытку заново.
      setErr("Эта попытка не привязана к этому браузеру. Начните заново на странице предмета.");
      setLoading(false);
      return;
    }
    api
      .get<Attempt>(`/attempts/${id}`, withAttemptToken(token))
      .then((a) => {
        if (a.finishedAt) {
          clearActiveAttempt(a.variantId);
          nav(`/attempts/${a.id}/result`, { replace: true });
          return;
        }
        setAttempt(a);
        setAnswers(a.answers ?? {});
      })
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Не удалось загрузить попытку"))
      .finally(() => setLoading(false));
  }, [id, nav]);

  // Предупреждаем браузер если пользователь пытается закрыть вкладку во время теста
  useEffect(() => {
    if (!attempt) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [attempt]);

  const finishAttempt = useCallback(async () => {
    if (!attempt || finishedRef.current) return;
    finishedRef.current = true;
    setSubmitting(true);
    try {
      const token = getAttemptToken(attempt.id);
      const res = await api.post<AttemptResult>(
        `/attempts/${attempt.id}/finish`,
        undefined,
        withAttemptToken(token),
      );
      clearActiveAttempt(attempt.variantId);
      nav(`/attempts/${attempt.id}/result`, { replace: true, state: { result: res } });
    } catch (e) {
      finishedRef.current = false;
      setErr(e instanceof ApiError ? e.message : "Не удалось завершить тест");
    } finally {
      setSubmitting(false);
    }
  }, [attempt, nav]);

  useEffect(() => {
    if (!timerEnabled || !attempt) return;
    if (secondsLeft <= 0) {
      void finishAttempt();
      return;
    }
    const t = setTimeout(() => setSecondsLeft((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [timerEnabled, secondsLeft, attempt, finishAttempt]);

  function startTimer() {
    if (!attempt) return;
    setTimerEnabled(true);
    setSecondsLeft(attempt.durationMinutes * 60);
  }

  function stopTimer() {
    setTimerEnabled(false);
    setSecondsLeft(0);
  }

  const activeQuestion = useMemo(
    () => (attempt ? attempt.questions[activeIdx] : null),
    [attempt, activeIdx],
  );

  async function saveAnswer(questionId: string, selected: string[]) {
    if (!attempt) return;
    setAnswers((prev) => ({ ...prev, [questionId]: selected }));
    setSavingMap((m) => ({ ...m, [questionId]: true }));
    try {
      const token = getAttemptToken(attempt.id);
      await api.put(
        `/attempts/${attempt.id}/answer`,
        { questionId, selectedOptionIds: selected },
        withAttemptToken(token),
      );
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось сохранить ответ");
    } finally {
      setSavingMap((m) => {
        const c = { ...m };
        delete c[questionId];
        return c;
      });
    }
  }

  function toggleOption(optionId: string) {
    if (!activeQuestion) return;
    const current = answers[activeQuestion.id] ?? [];
    const next = current.includes(optionId)
      ? current.filter((x) => x !== optionId)
      : [...current, optionId];
    void saveAnswer(activeQuestion.id, next);
  }

  if (loading) return <p className="text-neutral-500">Загрузка...</p>;
  if (err) return <p className="text-red-600">{err}</p>;
  if (!attempt) return null;

  const total = attempt.questions.length;
  const answeredCount = Object.values(answers).filter((a) => a && a.length > 0).length;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm text-neutral-500">
            {attempt.subjectName} · {attempt.guest ? `${attempt.guest.firstName} ${attempt.guest.lastName}` : ""}
          </p>
          <h1 className="text-lg font-semibold sm:text-xl">{attempt.variantTitle}</h1>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {timerEnabled ? (
            <>
              <span className="rounded-md bg-neutral-900 px-3 py-1 text-sm font-mono text-white">
                {formatTime(secondsLeft)}
              </span>
              <button className="btn-ghost text-sm" onClick={stopTimer}>
                Выкл.
              </button>
            </>
          ) : (
            <button className="btn-secondary text-sm" onClick={startTimer}>
              <span className="hidden sm:inline">Включить таймер ({attempt.durationMinutes} мин)</span>
              <span className="sm:hidden">Таймер {attempt.durationMinutes} мин</span>
            </button>
          )}
        </div>
      </div>

      <QuestionGrid
        total={total}
        activeIdx={activeIdx}
        answers={answers}
        questions={attempt.questions}
        onPick={setActiveIdx}
      />

      <div>
        <div className="mb-1 flex justify-between text-xs text-neutral-500">
          <span>Отвечено</span>
          <span>{answeredCount} / {total}</span>
        </div>
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-neutral-100">
          <div
            className="h-1.5 rounded-full bg-neutral-900 transition-all duration-300"
            style={{ width: total > 0 ? `${(answeredCount / total) * 100}%` : "0%" }}
          />
        </div>
      </div>

      {activeQuestion ? (
        <NoCopy className="card space-y-4">
          <div className="flex items-baseline justify-between">
            <h2 className="text-base font-medium">
              Вопрос {activeIdx + 1} из {total}
            </h2>
            <span className="text-xs text-neutral-500">
              {savingMap[activeQuestion.id] ? "Сохранение..." : ""}
            </span>
          </div>
          <p className="whitespace-pre-wrap text-neutral-900">{activeQuestion.text}</p>
          {activeQuestion.imageUrl ? (
            <img
              src={resolveImageUrl(activeQuestion.imageUrl) ?? ""}
              alt=""
              draggable={false}
              className="max-h-96 w-full rounded-md border border-neutral-200 bg-white object-contain"
            />
          ) : null}
          <ul className="space-y-2">
            {activeQuestion.options.map((opt) => {
              const selected = (answers[activeQuestion.id] ?? []).includes(opt.id);
              return (
                <li key={opt.id}>
                  <label
                    className={`flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors ${
                      selected
                        ? "border-neutral-900 bg-neutral-50"
                        : "border-neutral-200 hover:bg-neutral-50"
                    }`}
                  >
                    <input
                      type="checkbox"
                      className="mt-1 h-4 w-4 accent-neutral-900"
                      checked={selected}
                      onChange={() => toggleOption(opt.id)}
                    />
                    <span className="whitespace-pre-wrap text-sm">{opt.text}</span>
                  </label>
                </li>
              );
            })}
          </ul>
        </NoCopy>
      ) : null}

      <div className="flex items-center justify-between gap-2">
        <button
          className="btn-secondary min-w-[80px]"
          disabled={activeIdx === 0}
          onClick={() => setActiveIdx((i) => Math.max(0, i - 1))}
        >
          ← Назад
        </button>
        {activeIdx < total - 1 ? (
          <button
            className="btn-primary min-w-[80px]"
            onClick={() => setActiveIdx((i) => Math.min(total - 1, i + 1))}
          >
            Вперёд →
          </button>
        ) : (
          <button className="btn-primary min-w-[80px]" disabled={submitting} onClick={finishAttempt}>
            {submitting ? "Завершение..." : "Завершить тест"}
          </button>
        )}
      </div>
    </div>
  );
}

function QuestionGrid({
  total,
  activeIdx,
  answers,
  questions,
  onPick,
}: {
  total: number;
  activeIdx: number;
  answers: Record<string, string[]>;
  questions: Attempt["questions"];
  onPick: (idx: number) => void;
}) {
  return (
    <div className="rounded-lg border border-neutral-200 bg-white p-3">
      <div className="max-h-32 overflow-y-auto sm:max-h-none">
        <div className="flex flex-wrap gap-1.5 sm:gap-2">
          {Array.from({ length: total }).map((_, i) => {
            const q = questions[i];
            const answered = q && (answers[q.id] ?? []).length > 0;
            const isActive = i === activeIdx;
            return (
              <button
                key={i}
                onClick={() => onPick(i)}
                className={[
                  "h-8 w-8 rounded-md border text-xs font-medium transition-colors sm:h-9 sm:w-9 sm:text-sm",
                  isActive
                    ? "border-neutral-900 bg-neutral-900 text-white"
                    : answered
                      ? "border-neutral-300 bg-neutral-100 text-neutral-900"
                      : "border-neutral-200 bg-white text-neutral-500 hover:bg-neutral-50",
                ].join(" ")}
                aria-current={isActive ? "true" : undefined}
              >
                {i + 1}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function formatTime(total: number): string {
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}
