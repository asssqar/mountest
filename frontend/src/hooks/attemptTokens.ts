// Хранилище токенов попыток в localStorage.
// Бэкенд требует X-Attempt-Token на каждом обращении к /attempts/{id}/*.
// Токен возвращается один раз — при создании попытки. Здесь мы его сохраняем
// и потом достаём по attemptId на чтение/сохранение/завершение.

const KEY = "mountest_attempt_tokens";

type TokenMap = Record<string, string>;

function readAll(): TokenMap {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") return parsed as TokenMap;
    return {};
  } catch {
    return {};
  }
}

function writeAll(map: TokenMap) {
  try {
    localStorage.setItem(KEY, JSON.stringify(map));
  } catch {
    // QuotaExceeded и т.п. — просто игнорим, токены восстановятся при следующем старте попытки.
  }
}

export function getAttemptToken(attemptId: string): string | null {
  return readAll()[attemptId] ?? null;
}

export function saveAttemptToken(attemptId: string, token: string) {
  const map = readAll();
  map[attemptId] = token;
  writeAll(map);
}

export function clearAttemptToken(attemptId: string) {
  const map = readAll();
  delete map[attemptId];
  writeAll(map);
}
