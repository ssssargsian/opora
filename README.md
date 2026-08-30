# Опора

«Опора» — защищённая система сопровождения детей и работы специалистов образовательной организации. Репозиторий содержит modular monolith на Go, web-приложение Next.js и локальную инфраструктуру PostgreSQL, MinIO, ClamAV и ONLYOFFICE.

## Требования

- macOS arm64 и Homebrew в `/opt/homebrew`
- Go 1.27.x
- Node.js 24.x и pnpm
- Docker Desktop с 8–12 ГБ доступной памяти

Проверка локального toolchain:

```sh
make bootstrap
```

## Как запустить Opora

```sh
cp .env.example .env
```

Задайте в `.env` локальный пароль администратора. Значение не должно быть production-паролем:

```dotenv
DEV_ADMIN_PASSWORD=change-me-for-local-development
```

Затем установите зависимости и запустите проект:

```sh
pnpm install --frozen-lockfile
make dev
```

`make dev` поднимает инфраструктуру, ожидает PostgreSQL, применяет migrations и запускает API с frontend в foreground. `Ctrl+C` останавливает API и frontend, сохраняя Docker volumes.

Для раздельного запуска используйте три терминала:

```sh
make up
make migrate
```

```sh
make api
```

```sh
make web
```

## Как войти

Откройте `http://localhost:3000/login` и используйте:

```text
Email: admin@opora.local
Пароль: значение DEV_ADMIN_PASSWORD из локального .env
```

При старте API development bootstrap создаёт организацию, администратора и демонстрационного ребёнка. Bootstrap отключён при `APP_ENV=production` и не выполняется при пустом `DEV_ADMIN_PASSWORD`. Пароль хранится как Argon2id hash. Браузер получает persistent HttpOnly session cookie на семь дней, а PostgreSQL хранит только hash случайного session token.

## Как создать ребёнка

Откройте раздел «Дети» и нажмите «Добавить ребёнка». Заполните фамилию и имя, при необходимости дату рождения, отчество и класс. Карточка сразу появится в списке без перезагрузки. Кнопка доступна любому сотруднику с permission `students.create`; frontend не проверяет название роли.

## Как добавить специалиста и настроить доступ

1. Откройте раздел «Специалисты» и нажмите «Добавить специалиста».
2. Заполните ФИО, email и выберите одну из ролей организации.
3. Сохраните начальный пароль, который интерфейс покажет один раз. Email-рассылка в MVP не подключена.
4. Откройте карточку ребёнка, вкладку «Доступ» и нажмите «Настроить доступ».
5. Выберите специалиста и выдайте только нужные grants: просмотр, загрузку, скачивание и/или редактирование.

Создание пользователя доступно по permissions `users.create`, `users.invite` или `users.manage`, настройка доступа — по `access.manage`. Роль является набором permissions; backend не принимает решения по названию роли. Специалист без `all_students` видит только детей с явным grant.

## Как загрузить документ

1. Откройте `http://localhost:3000/students`.
2. Выберите ребёнка.
3. Нажмите «Загрузить документ».
4. Выберите `.docx` или `.pdf` размером до 25 МБ и подтвердите загрузку.

API проверяет extension и сигнатуру, вычисляет SHA-256, выполняет обязательный ClamAV scan и только затем сохраняет immutable object в private S3 bucket. При сбое антивируса upload закрывается безопасной ошибкой. Имена учащихся и исходные filenames не входят в object key.

## Как открыть DOCX в ONLYOFFICE

В карточке ребёнка нажмите на название DOCX. Страница редактора получает подписанную конфигурацию только после backend authorization. Название PDF открывает безопасный inline preview, который стримит файл через API после проверки `documents.view`. ONLYOFFICE читает конкретную версию через короткоживущий URL. После сохранения callback повторно проверяется, файл сканируется и создаётся новая `DocumentVersion`; предыдущий объект не перезаписывается.

Локальная сеть использует разные адреса:

- `ONLYOFFICE_PUBLIC_URL=http://localhost:8082` доступен браузеру.
- `ONLYOFFICE_INTERNAL_URL=http://localhost:8082` доступен API.
- `ONLYOFFICE_INTERNAL_API_URL=http://host.docker.internal:8080` доступен Document Server.
- `ONLYOFFICE_CALLBACK_ORIGIN=http://localhost` ограничивает допустимый origin URL, присланного callback.

Не переносите локальные secrets и `ALLOW_PRIVATE_IP_ADDRESS` в production. Production URLs, TLS и secrets должны поступать из deployment environment/secret manager.

## Как посмотреть MinIO

Откройте `http://localhost:9001`. Логин и пароль берутся из локального `.env`: `MINIO_ROOT_USER` и `MINIO_ROOT_PASSWORD`. Bucket `opora-documents` создаётся автоматически и остаётся private.

## Как посмотреть логи

```sh
make logs
```

Команда показывает Docker logs. `make api` и `make web` оставляют runtime logs в своих терминалах. API пишет structured request logs с request ID, методом, путём, статусом и duration, но не логирует passwords, cookies, session tokens, document bytes, JWT или временные URLs.

## Как остановить

Остановите foreground `make dev` через `Ctrl+C`, затем инфраструктуру:

```sh
make down
```

## Команды

```sh
make up              # запустить PostgreSQL, MinIO, ClamAV и ONLYOFFICE
make down            # остановить инфраструктуру, сохранив volumes
make logs            # Docker logs в foreground
make migrate         # применить goose migrations
make generate        # sqlc и TypeScript types из OpenAPI
make api             # Go API в foreground
make web             # Next.js dev server в foreground
make dev             # infrastructure + migrations + API + frontend
make test            # backend и frontend tests
make lint            # vet, golangci-lint, govulncheck, ESLint, TypeScript
make health          # API liveness и readiness
```

Полная локальная проверка:

```sh
cd apps/api
go test ./...
go vet ./...
golangci-lint run ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

cd ../../apps/web
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

## Адреса

- Frontend: `http://localhost:3000`
- API: `http://localhost:8080`
- Liveness: `http://localhost:8080/health/live`
- Readiness: `http://localhost:8080/health/ready`
- MinIO Console: `http://localhost:9001`
- ONLYOFFICE: `http://localhost:8082`

## Production deployment

Production stack runs entirely in Docker behind Caddy. The host publishes only SSH, HTTP, and HTTPS; API, web, PostgreSQL, MinIO, ClamAV, and ONLYOFFICE ports remain private. Initial provisioning, upgrades, first-admin creation, logs, and backups are documented in [deploy/README.md](deploy/README.md).

```sh
git clone https://github.com/ssssargsian/opora.git /opt/opora
cd /opt/opora
./deploy/provision-ubuntu.sh
```

The idempotent provisioner creates `.env.production` with random credentials and preserves it and all Docker volumes on repeat runs. Production never uses `DEV_ADMIN_PASSWORD`; the first administrator is created with the one-off `admin` container command from the deployment guide.

Frontend обращается к API через same-origin Next.js rewrite `/api/*`, поэтому cookie не требует небезопасного cross-origin режима. Прямой API CORS разрешает credentials только для конкретного `WEB_ORIGIN`.

## Структура

```text
apps/api/             Go API и предметные модули
apps/web/             Next.js App Router frontend
openapi/              OpenAPI 3.1 contract реализованных endpoints
infra/                production infrastructure foundation
docs/adr/             architecture decisions
scripts/              development scripts
.github/workflows/    pull request CI
```

Security boundary находится на backend: каждый tenant-owned запрос scoped по `organization_id`, permissions проверяются централизованно, restricted documents требуют явного student grant, audit append-only для приложения, а S3 bucket никогда не становится public.
# opora
