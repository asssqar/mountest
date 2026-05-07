import { FormEvent, useEffect, useState } from "react";
import { adminApi } from "./AdminApi";
import { ApiError } from "../api/client";
import type { Subject } from "../api/types";

export default function AdminSubjectsPage() {
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");

  async function reload() {
    setLoading(true);
    try {
      setSubjects(await adminApi.listSubjects());
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ошибка загрузки");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void reload();
  }, []);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    try {
      await adminApi.createSubject(name.trim());
      setName("");
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось создать");
    }
  }

  async function onSaveEdit() {
    if (!editingId || !editingName.trim()) return;
    try {
      await adminApi.updateSubject(editingId, editingName.trim());
      setEditingId(null);
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось сохранить");
    }
  }

  async function onDelete(id: string) {
    if (!confirm("Удалить предмет вместе со всеми его вариантами?")) return;
    try {
      await adminApi.deleteSubject(id);
      await reload();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Не удалось удалить");
    }
  }

  return (
    <div className="space-y-6">
      <form onSubmit={onCreate} className="card flex items-end gap-3">
        <div className="flex-1">
          <label className="label">Новый предмет</label>
          <input
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Название предмета"
          />
        </div>
        <button className="btn-primary">Добавить</button>
      </form>

      {err ? <p className="text-sm text-red-600">{err}</p> : null}

      {loading ? (
        <p className="text-neutral-500">Загрузка...</p>
      ) : subjects.length === 0 ? (
        <p className="text-neutral-500">Пока нет предметов.</p>
      ) : (
        <ul className="space-y-2">
          {subjects.map((s) => (
            <li key={s.id} className="card flex items-center justify-between gap-3">
              {editingId === s.id ? (
                <input
                  className="input flex-1"
                  value={editingName}
                  onChange={(e) => setEditingName(e.target.value)}
                />
              ) : (
                <div>
                  <p className="font-medium">{s.name}</p>
                  <p className="text-xs text-neutral-500">{s.variantsCount} вариантов</p>
                </div>
              )}
              <div className="flex gap-2">
                {editingId === s.id ? (
                  <>
                    <button className="btn-primary" onClick={onSaveEdit}>
                      Сохранить
                    </button>
                    <button
                      className="btn-ghost"
                      onClick={() => setEditingId(null)}
                    >
                      Отмена
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      className="btn-secondary"
                      onClick={() => {
                        setEditingId(s.id);
                        setEditingName(s.name);
                      }}
                    >
                      Изменить
                    </button>
                    <button className="btn-danger" onClick={() => onDelete(s.id)}>
                      Удалить
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
