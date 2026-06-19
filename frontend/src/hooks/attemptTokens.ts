// Хранилище токенов попыток в localStorage.
// Бэкенд требует X-Attempt-Token на каждом обращении к /attempts/{id}/*.
// Токен возвращается один раз — при создании попытки. Здесь мы его сохраняем
// и потом достаём по attemptId на чтение/сохранение/завершение.

const KEY = "mountest_attempt_tokens";
const ACTIVE_KEY = "mountest_active_attempts"; // variantId → attemptId

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

// --- активные попытки (variantId → attemptId) ---

function readActive(): TokenMap {
  try {
    const raw = localStorage.getItem(ACTIVE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") return parsed as TokenMap;
    return {};
  } catch {
    return {};
  }
}

function writeActive(map: TokenMap) {
  try {
    localStorage.setItem(ACTIVE_KEY, JSON.stringify(map));
  } catch {}
}

export function saveActiveAttempt(variantId: string, attemptId: string) {
  const m = readActive();
  m[variantId] = attemptId;
  writeActive(m);
}

export function getActiveAttemptId(variantId: string): string | null {
  return readActive()[variantId] ?? null;
}

export function clearActiveAttempt(variantId: string) {
  const m = readActive();
  delete m[variantId];
  writeActive(m);
}
