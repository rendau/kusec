# CLAUDE.md

Руководство для Claude Code по работе с этим репозиторием. Go-сервис на Clean Architecture + DDD,
с gRPC/grpc-gateway, Postgres (pgx), proto-генерацией через Makefile.

Детальные конвенции — в глобальных скиллах `crud`, `mobone`, `golang-service`, `golang-samber-lo` (триггерятся автоматически).

---

## Структура проекта

### Верхний уровень
- `cmd/main.go` — entrypoint, поднимает `internal/app.App`.
- `cmd/mcp/` — MCP-сервер для AI-агентов (stdio, `make build-mcp`): доступ к API kusec
  без раскрытия значений секретов (маскирование + декларативный `value_source`).
  Код — `internal/mcpserver/`, подробности — `docs/mcp-server.md`.
- `internal/` — бизнес-логика и инфраструктура (закрытые пакеты).
- `apps/admin/` — фронтенд админки (Vue 3 SPA). Конвенции и тонкости — в
  `apps/admin/CLAUDE.md` (пакетный менеджер pnpm, обработка ошибок API,
  сессии/2FA, адаптив).
- `api/proto/` — исходные `.proto` данного сервиса.
- `pkg/proto/` — сгенерированный код protobuf/grpc/gateway (**не редактировать вручную**).
- `migrations/` — SQL миграции Postgres.
- `docs/` — swagger JSON и статические доки, выдаются через `/docs/*`.
- `vendor-proto/` — внешние `.proto` зависимости (обновляются Makefile).
- `Makefile` — сборка, запуск, генерация proto, docker-образы.
- `deploy/docker/` — единственный `Dockerfile`: полная multi-stage сборка целиком в
  Docker (этап `admin-builder` — фронт через pnpm; этап `builder` — Go из исходников;
  финальный alpine-образ с бинарником + `docs/` + `migrations/` + админкой в `admin-dist/`).
  Helm-chart для локального подъёма живёт в отдельном репо `local_kube`
  (`charts/kusec`, поднимается там через helmfile).
- `.env.example`, `.migrate_scripts.example` — примеры окружения и миграций.

### Внутренние пакеты (`internal/`)
- `internal/app/` — сборка приложения: серверы, миграции, метрики, трассировка, HTTP-gateway, DI.
  - `app.go` — граф зависимостей и запуск компонентов.
  - `grpc.go` — gRPC сервер + интерсепторы ошибок/метрик/трейсинга.
  - `grpc_gateway.go` — HTTP-gateway, CORS, error handler.
  - `system_http_server.go` — системный HTTP-сервер (порт 3003): /healthcheck, /docs/*, /metrics.
  - `migration.go` — запуск миграций из `migrations/`.
- `internal/config/` — конфигурация через env (см. `config.go`).
- `internal/handler/` — транспортный слой.
  - `grpc/` — gRPC handlers.
  - `grpc/dto/` — преобразование protobuf ↔ domain models.
  - могут быть и другие транспортные каналы.
- `internal/usecase/` — usecase-слой (валидация, оркестрация сервисов и доменных сервисов).
- `internal/domain/` — доменная модель, сервисы и репозитории.
  - `*/model/` — доменные структуры (entity).
  - `*/service/` — доменные сервисы (инварианты/логика).
  - `*/repo/` — репозитории.
  - `common/` — общие модели/утилиты/PG базовый репозиторий.
- `internal/service/` — сервисы (фоновые/инфраструктурные), для переиспользования или выделения логики.
- `internal/errs/` и `internal/constant/` — общие коды ошибок и константы.

---

## Архитектура: слои и зависимости

- **Transport** (`internal/handler/grpc/*`):
  - Работает только с protobuf DTO и usecase-интерфейсами.
  - Не обращается напрямую к репозиториям и сервисам.
- **Usecase** (`internal/usecase/*`):
  - Входной слой от транспортного слоя (запросы от внешних систем).
  - Валидация входных параметров.
  - Оркестрация доменных сервисов и сервисов `internal/service/*`.
  - Вход/выход — доменные модели (не protobuf).
  - Желательно не обращается в соседние usecases.
- **Domain** (`internal/domain/*`):
  - `service/` — доменные операции и инварианты. Может использовать только репозиторий.
  - `repo/` — доступ к хранилищам. Может содержать подпапки для разных типов хранилищ или моков.
- **Service** (Infrastructure/background/external integrations, `internal/service/*`):
  - Фоновые процессы, интеграции с внешними системами; выделенные или переиспользуемые логики.
  - Может использовать другие сервисы `internal/service/*` и доменные сервисы `internal/domain/*/service`.
  - Не обращается в usecase слой.
- **Composition** (`internal/app/`):
  - Сборка зависимостей, запуск серверов, миграций, фоновых сервисов.

### Правило зависимостей
```
handler        → usecase
usecase        → domain service
usecase        → service
service        → service
service        → domain service
domain service → repo
```
- Обратные зависимости **запрещены**.
- К `repo` слою доступ только из `domain service`.

### Хранилища
- Postgres: все доменные entities (см. `migrations/*`).
- **Имена таблиц всегда в единственном числе**, без plural: `usr`, `app`, `secret`, `item`
  (не `usrs`, `apps`, `secrets`, `items`). То же значение указывается в `TableName` репозитория.

### Миграции
- Файлы в `migrations/` в формате `NNNNNN_<name>.up.sql` / `.down.sql` (golang-migrate).
- В `down`-миграциях во всех командах `DROP` обязательно указывать `CASCADE`
  (напр. `drop table if exists <table> cascade;`).
- В `down` объекты удаляются в порядке, обратном `up` (с учётом внешних ключей).

### API
- gRPC сервисы: `api/proto/kusec_v1/*`.
- HTTP-gateway: через grpc-gateway + swagger (`docs/api.swagger.json`).
- DTO маппинг: `internal/handler/grpc/dto`.
- Массовый импорт/экспорт: `POST /transfer/import` (upsert по натуральным
  ключам, см. `docs/transfer-import-api.md`) и `GET /transfer/tree`
  (всё дерево без значений item-ов, требуется аутентификация — для внешних
  агентов, см. `docs/transfer-tree-api.md`).
- **REST-пути (route paths) всегда в единственном числе**, без plural:
  `/secret`, `/secret/{id}`, `/app`, `/item` (не `/secrets`, `/apps`, `/items`).
- **Пагинация: во всех list-запросах `page` начинается с 0** (zero-based).
  Первая страница — `page=0`. Относится к `common.ListParamsSt.page` и всем
  клиентам API. UI с 1-based пагинацией (напр. naive-ui) обязан конвертировать:
  `apiPage = uiPage - 1`.
- API-ключи для машинных клиентов: `/api-key` (значение `ksk_…` выдаётся один раз,
  хранится sha256-хэш; аутентификация — тем же заголовком `Authorization: Bearer`,
  интерсептор различает ключ и JWT по префиксу). См. `docs/mcp-server.md`.
- **Update-методы используют HTTP `PUT`** (не `PATCH`). В proto-аннотациях
  (`google.api.http`) для `Update`/`Update*` указывать `put: "/<entity>/{id}"`,
  все клиенты API шлют `PUT`. CORS (`internal/app/grpc_gateway.go`) должен
  разрешать `PUT`.

### Ошибки и валидация
- Семантические ошибки — через `internal/errs` (см. gRPC interceptor в `internal/app/grpc.go`).
- Валидация параметров — в usecase.
- Нельзя пробрасывать ошибки наружу без wrapping (оборачивать в `fmt.Errorf("...: %w")`).

### Правила изменения кода
- Доменная сущность может иметь только `model/` (без `service/`/`repo/`), если записи не храним.
- gRPC DTO не должны протекать в доменные сервисы.
- `pkg/proto` и `docs/api.swagger.json` — генерируемые файлы (обновляются через Makefile).

---

## Runtime и конфигурация

### Запуск
- Entry: `cmd/main.go` → `internal/app.App`.
- На старте выполняются:
  - загрузка env (autoload `.env`),
  - настройка логгера/метрик/трейсинга,
  - pgx pool,
  - миграции (`internal/app/migration.go`),
  - запуск gRPC + HTTP-gateway.

### Переменные окружения
- Описаны в `internal/config/config.go`.
- Примеры: `.env.example`, `.migrate_scripts.example`.

### Системный HTTP-сервер
- Отдельный сервер на порту `3003` (`const systemHttpPort` в `internal/app/system_http_server.go`).
- Обслуживает служебные ручки: `/healthcheck`, `/docs/*`, `/metrics`.

### Метрики и трассировка
- Prometheus метрики на `/metrics` (системный сервер, порт 3003) при `WITH_METRICS=true`.
  Используется `metrics.Registry`, а не дефолтный `promhttp.Handler()`.
- Трейсинг Jaeger включается при `WITH_TRACING=true` и `JAEGER_ADDRESS`.

### Документация и healthcheck
- `/healthcheck` — HTTP healthcheck (200 OK), порт 3003.
- `/docs/*` — статические docs + swagger (`docs/api.swagger.json`), порт 3003.

### Сборка
- `make build` (дефолтная цель) создаёт бинарник `cmd/build/svc`
  (`go build`, `CGO_ENABLED=0`) — для локального запуска вне Docker.
- `make docker-build` собирает локальный образ `kusec:local` **целиком в Docker** через
  `deploy/docker/Dockerfile` (multi-stage): этап `admin-builder` собирает фронт (pnpm,
  `pnpm install --frozen-lockfile` → `pnpm build`), этап `builder` компилирует Go из
  исходников, финальный alpine-образ получает бинарник + `docs/` + `migrations/` +
  админку в `admin-dist/` (раздаётся бэком с `/`, API под `/api`).

### Flow проверки изменений
```
make generate-proto  →  gofmt  →  go test ./...  →  go run ./cmd/.
```
