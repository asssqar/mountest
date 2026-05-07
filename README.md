# MounTest — MVP тренажёра ЕНТ

Стек: **Go** (chi + pgx) · **PostgreSQL 16** · **Vite + React + TypeScript + Tailwind**.

> Деплой в прод (VPS + Docker + Caddy + автоHTTPS) — см. [DEPLOY.md](./DEPLOY.md).

Структура: **Предметы → Варианты → Вопросы**. На старте сидится два предмета — *Математика* и *Информатика* — и один демо-вариант с вопросами.

## Возможности

Ученик (без регистрации):
- Вводит имя и фамилию — создаётся гостевая сессия (хранится в `localStorage` + строка в БД).
- Выбирает предмет → вариант → начинает попытку.
- Один вопрос на экран, кнопки **Назад / Вперёд**, сверху сетка номеров вопросов с переходом по клику.
- Опциональный таймер: ученик включает кнопкой, длительность задаёт админ при создании варианта.
- Каждый ответ автосохраняется на бэкенд (можно обновить страницу — состояние сохранится).
- Завершение → экран результата: балл, % и ошибки. По каждой ошибке: вопрос, **выбор ученика**, **правильные ответы** (без объяснений).

Вопрос засчитывается **только если множество выбранных опций точно совпадает с множеством правильных** (multi-select, без частичных баллов).

Админ (`/admin`):
- Логин по JWT в `HttpOnly` cookie.
- CRUD предметов / вариантов / вопросов / вариантов ответов.
- При создании вопроса можно отметить **один или несколько** правильных ответов.

## Структура репозитория

```
backend/                 # Go backend
  cmd/server/main.go
  internal/
    auth/                # JWT + middleware
    config/
    db/                  # connect + embedded migrations
      migrations/001_init.sql
    handlers/            # public + admin
    httpx/
    seed/                # seed admin & demo data
docker-compose.yml       # локальный Postgres
.env.example
frontend/                # Vite + React + TS + Tailwind
  src/
    admin/               # админка
    api/                 # клиент + типы
    components/
    hooks/
    pages/
```

## Быстрый старт

### 1. Скопировать env
```bash
cp .env.example .env
```
По желанию подправьте `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`.

### 2. Поднять Postgres
```bash
docker compose up -d
```
По умолчанию слушает `localhost:5432`, БД `mountest`.

### 3. Запустить backend
Из корня репозитория:
```bash
cd backend
go mod tidy
# подхватим переменные из корневого .env (или экспортируйте сами)
set -a && . ../.env && set +a
go run ./cmd/server
```

При старте бэкенд:
- применяет миграции (`internal/db/migrations/*.sql`),
- создаёт админа из `ADMIN_USERNAME` / `ADMIN_PASSWORD`, если его ещё нет,
- если `SEED_DEMO=true` — создаёт *Математику*, *Информатику* и демо-вариант с тремя вопросами.

API доступен на `http://localhost:8080`.

### 4. Запустить frontend
В **отдельном терминале** из корня репозитория:
```bash
cd frontend
npm install
npm run dev
```
Открыть `http://localhost:5173`.

> Если в текущем терминале вы уже находитесь в `backend/`, путь будет `cd ../frontend`.

> Фронт читает `VITE_API_BASE` (по умолчанию `http://localhost:8080/api`). Запросы идут с `credentials: "include"`, поэтому в `CORS_ORIGIN` бэкенда указан origin Vite.

### 5. Войти в админку
- Откройте `http://localhost:5173/admin`.
- Логин/пароль — из `.env` (по умолчанию `admin` / `admin123`).

## Переменные окружения

См. `.env.example`. Ключевые:

| Переменная | Назначение |
|------------|------------|
| `DATABASE_URL` | DSN (имеет приоритет над `POSTGRES_*`) |
| `APP_PORT` | Порт backend (по умолчанию `8080`) |
| `JWT_SECRET` | Секрет подписи JWT (поменяйте в проде) |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | Дефолтный админ при первом старте |
| `SEED_DEMO` | `true` — сидер демо-данных |
| `CORS_ORIGIN` | Origin, которому разрешены credentials-запросы |
| `VITE_API_BASE` | Базовый URL API для фронта |

## API (краткое описание)

Публичные:
- `GET /api/subjects` — список предметов.
- `GET /api/subjects/{id}/variants` — варианты предмета.
- `POST /api/guest-sessions {firstName, lastName}` — создать гостевую сессию.
- `GET /api/guest-sessions/{id}` — получить гостя.
- `POST /api/attempts {variantId, guestSessionId}` — начать попытку (отдаёт вопросы и опции **без флага правильности**).
- `GET /api/attempts/{id}` — состояние попытки + сохранённые ответы.
- `PUT /api/attempts/{id}/answer {questionId, selectedOptionIds[]}` — автосохранение ответа.
- `POST /api/attempts/{id}/finish` — завершить и получить результат.
- `GET /api/attempts/{id}/result` — получить результат завершённой попытки.

Админ (cookie `mountest_admin`):
- `POST /api/admin/login {username, password}` / `POST /api/admin/logout` / `GET /api/admin/me`
- `GET|POST /api/admin/subjects`, `PUT|DELETE /api/admin/subjects/{id}`
- `GET /api/admin/variants?subjectId=...`, `POST`, `PUT|DELETE /{id}`
- `GET /api/admin/questions?variantId=...`, `GET /{id}`, `POST`, `PUT|DELETE /{id}` — пейлоад вопросов содержит массив `options` с `text` и `isCorrect`.

## Сидер

Сидер живёт в `backend/internal/seed/seed.go`:
- `EnsureAdmin` — создаёт админа, если такого username ещё нет.
- `EnsureDemo` — добавляет *Математику* и *Информатику*, плюс один вариант (*Демо-вариант №1*, 30 минут) с тремя вопросами:
  - однозначный multiple choice (один правильный),
  - выбор всех простых чисел,
  - все корни уравнения `x² = 9`.

Чтобы пересоздать демо — почистите соответствующие предметы/вариант или удалите volume Postgres (`docker compose down -v`).

## Известные допущения MVP

- Гостевая сессия живёт по `localStorage` в браузере; защиты «чужой попытки» нет — кто знает `attemptId`, тот и видит. Для прода стоит подписать `attemptId`/добавить cookie гостя.
- Только русская локаль.
- Только multiple choice (по требованию). Без открытых ответов и объяснений.
- Без частичных баллов: вопрос засчитан только при точном совпадении множеств.
- Миграции встроены в бинарь и применяются на старте; одного `001_init.sql` достаточно для MVP.
- Linter/тестов backend в комплекте нет — только `go vet ./...`.
# mountest
# mountest
