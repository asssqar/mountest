const API_BASE =
  (import.meta.env.VITE_API_BASE as string | undefined) ?? "http://localhost:8080/api";

// Бэкенд возвращает абсолютный путь /api/uploads/<file>, что удобно при общем домене.
// При локальной разработке (API на другом порту) подменяем префикс на API_BASE.
export function resolveImageUrl(url: string | null | undefined): string | null {
  if (!url) return null;
  if (url.startsWith("/api/")) {
    return API_BASE + url.slice(4);
  }
  return url;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(init.headers ?? {}),
    },
    ...init,
  });
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const msg = (data && (data.error as string)) || `Ошибка ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

// withAttemptToken — добавляет к запросу заголовок X-Attempt-Token.
// Каждое обращение к /attempts/{id}/* теперь требует его на бэкенде.
export function withAttemptToken(token: string | null | undefined): RequestInit {
  return token ? { headers: { "X-Attempt-Token": token } } : {};
}

async function uploadFile<T>(path: string, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    credentials: "include",
    body: form,
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const msg = (data && (data.error as string)) || `Ошибка ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

// extra — позволяет точечно прокидывать заголовки (например, X-Attempt-Token)
// без расшаривания глобального state'а или middleware.
export const api = {
  get: <T>(path: string, extra: RequestInit = {}) =>
    request<T>(path, { ...extra }),
  post: <T>(path: string, body?: unknown, extra: RequestInit = {}) =>
    request<T>(path, {
      method: "POST",
      body: body ? JSON.stringify(body) : undefined,
      ...extra,
    }),
  put: <T>(path: string, body?: unknown, extra: RequestInit = {}) =>
    request<T>(path, {
      method: "PUT",
      body: body ? JSON.stringify(body) : undefined,
      ...extra,
    }),
  delete: <T>(path: string, extra: RequestInit = {}) =>
    request<T>(path, { method: "DELETE", ...extra }),
  upload: <T>(path: string, file: File) => uploadFile<T>(path, file),
};
