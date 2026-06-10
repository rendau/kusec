# CLAUDE.md

Руководство для Claude Code по работе с этим репозиторием. Go-сервис на Clean Architecture + DDD,
с gRPC/grpc-gateway, Postgres (pgx), proto-генерацией через Makefile.

Для генерации CRUD новой сущности используется skill `crud` (см. `.claude/skills/crud/`).

---

## Структура проекта

### Верхний уровень
- `cmd/main.go` — entrypoint, поднимает `internal/app.App`.
- `internal/` — бизнес-логика и инфраструктура (закрытые пакеты).
- `api/proto/` — исходные `.proto` данного сервиса.
- `pkg/proto/` — сгенерированный код protobuf/grpc/gateway (**не редактировать вручную**).
- `migrations/` — SQL миграции Postgres.
- `docs/` — swagger JSON и статические доки, выдаются через `/docs/*`.
- `vendor-proto/` — внешние `.proto` зависимости (обновляются Makefile).
- `Dockerfile`, `Makefile` — сборка, запуск, генерация proto.
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
  - каждый сервис может иметь свои локальные модели `*/model/` (при необходимости).
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
  - `model/` — структуры данных, сущности (entity).
  - `service/` — доменные операции и инварианты. Может использовать только репозиторий.
  - `repo/` — доступ к хранилищам. Может содержать подпапки для разных типов хранилищ или моков.
- **Service** (Infrastructure/background/external integrations, `internal/service/*`):
  - Фоновые процессы, интеграции с внешними системами.
  - Выделенные или переиспользуемые логики.
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
- **REST-пути (route paths) всегда в единственном числе**, без plural:
  `/secret`, `/secret/{id}`, `/app`, `/item` (не `/secrets`, `/apps`, `/items`).
- **Пагинация: во всех list-запросах `page` начинается с 0** (zero-based).
  Первая страница — `page=0`. Относится к `common.ListParamsSt.page` и всем
  клиентам API. UI с 1-based пагинацией (напр. naive-ui) обязан конвертировать:
  `apiPage = uiPage - 1`.

### Ошибки и валидация
- Семантические ошибки — через `internal/errs` (см. gRPC interceptor в `internal/app/grpc.go`).
- Валидация параметров — в usecase.
- Нельзя пробрасывать ошибки наружу без wrapping (оборачивать в `fmt.Errorf("...: %w")`).

### Правила изменения кода
- Новые доменные сущности должны иметь `model/`, `service/`, `repo/` + usecase и handler слой.
  Иногда могут иметь только `model/`, если не храним записи.
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
- `make build` создаёт бинарник `cmd/build/svc`.
- Dockerfile копирует бинарник, `docs/` и `migrations/` в `/app`.

### Flow проверки изменений
```
make generate-proto  →  gofmt  →  go test ./...  →  go run ./cmd/.
```
