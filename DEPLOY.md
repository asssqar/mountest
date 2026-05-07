# Деплой MounTest на Hetzner CX22 (или любой другой VPS)

Стек прод-окружения:
- **Caddy** — единая точка входа (`:80` / `:443`), автоHTTPS через Let's Encrypt, раздаёт собранный фронт и проксирует `/api/*` и `/healthz` в backend.
- **backend** — Go-сервис, собирается в distroless-образ (~20 МБ), слушает только внутри docker-сети.
- **db** — PostgreSQL 16 в отдельном контейнере, данные на named volume.

Связка контейнеров: пользователь → `caddy:443` → `backend:8080` → `db:5432`.

## 0. Что нужно подготовить заранее

1. **Домен** (`mountest.example.com`). Купите у любого регистратора.
2. **VPS** (рекомендуется Hetzner CX22, ≈€4.5/мес). При создании выберите Ubuntu 24.04 и добавьте свой SSH-ключ.
3. **DNS**: создайте A-запись `mountest.example.com → <IP сервера>`. Дождитесь распространения (`dig +short mountest.example.com` должен показывать ваш IP). Без этого Let's Encrypt не выдаст сертификат.

## 1. Поставить Docker на сервер

SSH на сервер:
```bash
ssh root@<IP>
```

Поставить Docker одной командой:
```bash
curl -fsSL https://get.docker.com | sh
```

Опционально — создать невидимого юзера и работать из-под него:
```bash
adduser --disabled-password --gecos "" deploy
usermod -aG docker deploy
mkdir -p /home/deploy/.ssh
cp ~/.ssh/authorized_keys /home/deploy/.ssh/
chown -R deploy:deploy /home/deploy/.ssh
chmod 700 /home/deploy/.ssh && chmod 600 /home/deploy/.ssh/authorized_keys
```

Дальше работаем как `deploy` (`ssh deploy@<IP>`).

## 2. Залить код

Самый простой путь — `git clone` из вашего репозитория:
```bash
cd ~
git clone <url-вашего-репо> mountest
cd mountest
```

Если репозитория ещё нет — можно `scp -r` локальную папку, но git удобнее для будущих обновлений.

## 3. Настроить .env

Скопируйте шаблон и заполните:
```bash
cp .env.prod.example .env
nano .env
```

Обязательно:
- `DOMAIN=mountest.example.com` — точный домен с A-записью.
- `CADDY_EMAIL=you@example.com` — для уведомлений Let's Encrypt.
- `POSTGRES_PASSWORD=...` — сильный пароль (`openssl rand -hex 16`).
- `JWT_SECRET=...` — длинный секрет (`openssl rand -hex 32`).
- `ADMIN_PASSWORD=...` — сильный пароль для админа.
- `SEED_DEMO=false` — в проде демо не сидим.

## 4. Поднять стек

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

Что произойдёт:
1. Соберутся образы backend (Go) и caddy (Vite build → Caddy + dist).
2. Поднимется Postgres, бэкенд дождётся его healthcheck, применит миграции, создаст админа.
3. Caddy запустится, увидит ваш домен, получит сертификат Let's Encrypt и начнёт раздавать фронт + проксировать `/api`.

Через минуту откройте `https://mountest.example.com` в браузере. Админка — `https://mountest.example.com/admin`.

Логи — на случай если что-то не запустилось:
```bash
docker compose -f docker-compose.prod.yml logs -f --tail=100
```

Логи отдельного сервиса:
```bash
docker compose -f docker-compose.prod.yml logs -f caddy
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f db
```

## 5. Обновление приложения

После любых правок кода у себя локально:
```bash
git push
```
А на сервере:
```bash
cd ~/mountest
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

Compose пересоберёт только то, что изменилось. Postgres-данные сохраняются в volume, миграции применяются на старте.

## 6. Бэкап БД

Простой ручной дамп:
```bash
docker compose -f docker-compose.prod.yml exec -T db \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > backup_$(date +%F).sql
```

Автоматизировать через cron (на сервере):
```bash
crontab -e
# добавить строку:
0 3 * * * cd /home/deploy/mountest && docker compose -f docker-compose.prod.yml exec -T db pg_dump -U "$(grep POSTGRES_USER .env | cut -d= -f2)" "$(grep POSTGRES_DB .env | cut -d= -f2)" | gzip > /home/deploy/backups/mountest_$(date +\%F).sql.gz
```

И раз в неделю чистить старые бэкапы либо забирать их к себе через `scp`/`rclone`/`rsync`.

## 7. Файрвол (опционально, рекомендуется)

```bash
ufw allow OpenSSH
ufw allow 80
ufw allow 443
ufw --force enable
```

Postgres и backend наружу не открыты — они слушают только во внутренней docker-сети.

## 8. Полезные команды

```bash
# статус сервисов
docker compose -f docker-compose.prod.yml ps

# рестарт одного сервиса
docker compose -f docker-compose.prod.yml restart backend

# подключиться к Postgres
docker compose -f docker-compose.prod.yml exec db psql -U mountest -d mountest

# полностью остановить и удалить контейнеры (volume останутся → данные сохранятся)
docker compose -f docker-compose.prod.yml down

# полная очистка ВКЛЮЧАЯ БД (необратимо)
docker compose -f docker-compose.prod.yml down -v
```

## Частые проблемы

**Caddy не выпускает сертификат** — проверьте, что:
1. A-запись домена указывает на IP сервера: `dig +short mountest.example.com`.
2. На VPS открыты порты 80 и 443 (`ufw status`).
3. CADDY_EMAIL валидный.

Без работающего DNS Let's Encrypt не сможет пройти HTTP-challenge.

**Cookie админки «не сохраняется»** — это бывает на HTTP. В прод-конфиге `COOKIE_SECURE=true`, поэтому браузер ставит cookie только под HTTPS. Зайдите по `https://...`, не по `http://`.

**`bind: address already in use`** — на сервере уже что-то слушает 80/443 (например, системный nginx). Остановите его: `systemctl disable --now nginx`.

**Изменили `DOMAIN` или `JWT_SECRET`** — после правки `.env` нужен `docker compose -f docker-compose.prod.yml up -d` (для caddy/backend), чтобы переменные пересобрались.

## Минимальные правки кода под прод (уже сделаны)

- `auth.Service` принимает `secure bool` → cookie `Secure` ставится в проде.
- Если `CORS_ORIGIN` пустой — middleware CORS не подключается (один домен → CORS не нужен).
- `SEED_DEMO=false` — демо-данные не создаются.
- Frontend собирается с `VITE_API_BASE=/api` — все запросы относительные, не зависят от домена.
