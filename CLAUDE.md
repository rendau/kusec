# CLAUDE.md

Руководство для Claude Code по работе с этим репозиторием. Go-сервис на Clean Architecture + DDD,
с gRPC/grpc-gateway, Postgres (pgx), proto-генерацией через Makefile.

Детальные конвенции — в глобальных скиллах `crud`, `mobone`, `golang-service`, `golang-samber-lo` (триггерятся автоматически).

---

## Структура проекта

### Верхний уровень
- `cmd/main.go` — entrypoint, поднимает `internal/app.App`.
- `internal/` — бизнес-логика и инфраструктура (закрытые пакеты).
- `apps/admin/` — фронтенд админки (Vue 3 SPA). Конвенции и тонкости — в
  `apps/admin/CLAUDE.md` (пакетный менеджер pnpm, обработка ошибок API,
  сессии, адаптив).
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
  - `mcp/` — встроенный MCP-сервер для AI-агентов (streamable HTTP, отдельный порт,
    `MCP_ENABLED`/`MCP_PORT`): маскирование значений секретов + декларативный
    `value_source`; вход по api-ключу или OAuth (`MCP_OAUTH_ENABLED`, коннекторы Claude:
    OAuth-адреса — на MCP-порту, страница согласия — `/oauth/authorize` админки, подключение
    = mcp_only-ключ разрешившего пользователя; `internal/usecase/oauth`, `internal/domain/oauth`,
    чистка — `internal/service/oauthcleanup`). Подробности — `docs/mcp-server.md`.
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
- Неактивные элементы (только админ): `GET /inactive[?app_id=]` — app/secret/configmap/item-ы
  с `active=false` одним списком, `POST /inactive/delete` — удаление выбранного одной
  транзакцией с общим `batch_id`. Каскад с аудитом — `internal/service/cascade`; он не
  открывает транзакцию сам: вложенный `TxFn` закоммитил бы внешнюю.
- API-ключи для машинных клиентов: `/api-key` (значение `ksk_…` выдаётся один раз,
  хранится sha256-хэш; аутентификация — тем же заголовком `Authorization: Bearer`,
  интерсептор различает ключ и JWT по префиксу). Ключи с `mcp_only=true` принимает
  только встроенный MCP-эндпоинт. См. `docs/mcp-server.md`.
- **Update-методы используют HTTP `PUT`** (не `PATCH`). В proto-аннотациях
  (`google.api.http`) для `Update`/`Update*` указывать `put: "/<entity>/{id}"`,
  все клиенты API шлют `PUT`. CORS (`internal/app/grpc_gateway.go`) должен
  разрешать `PUT`.
- **Размер запроса**: тело HTTP-запроса на основном порту — не больше 8 МиБ
  (`httpMaxBodyBytes`, `internal/app/grpc_gateway.go`), иначе `413` с
  `{code: "invalid_request", message}`. Входящее gRPC-сообщение — не больше
  16 МиБ. `WriteTimeout` у сервера нет намеренно (sync идёт до 5 минут).
- **Текст ошибок**: клиенту уходит код из `internal/errs` (в `message` тот же
  код) либо `Desc`/`Fields` из `errs.ErrFull`. Любая другая ошибка —
  `service_not_available` и `internal error (request id: …)`, полный текст —
  в логе уровня Error с `request_id` и `method`. Текст для клиента оформляй
  через `errs.ErrFull`.
- gRPC reflection включён только при `DEBUG=true`.

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

### Аудит, журнал sync и read-only доступ

- **Аудит** (`internal/domain/audit` + регистратор `internal/service/audit`):
  каждая мутация app/secret/item/configmap/config_item/api_key/usr пишет запись
  в таблицу `audit` **в одной транзакции** с изменением (mobone `TxFn`; все
  `ModelStore` подключены к `TransactionManager`). Значения item (secret) в
  аудит не попадают никогда — только HMAC-отпечаток (`util.Fingerprint`, ключ
  `AUDIT_HASH_KEY`, 16 hex) и размер; значения config_item — целиком с лимитом
  4КБ; пароли/хэши ключей — никогда. Каскадные удаления и импорт пишут
  запись на каждого ребёнка с общим `batch_id`. Записи не редактируются и не
  удаляются через API; чистка — только фоновый ретеншн
  (`internal/service/retention`, `AUDIT_RETENTION_DAYS`, дефолт 180).
- **Журнал sync** (`internal/domain/syncrun`): каждый запуск sync — запись
  `sync_run` (создаётся на старте, финализируется статусом ok|partial|error) +
  `sync_run_object` по каждому k8s-объекту (op, имена изменившихся ключей,
  отпечаток содержимого — без значений). На созданные/обновлённые объекты
  ставятся аннотации `kusec.io/synced-at|sync-run-id|content-hash`; у
  secret/configmap ведутся `last_synced_at`/`last_synced_hash` (обновляются
  мимо `updated_at`).
- **Scope API-ключей**: `api_key.scope` = `full` | `read_only` | `mcp_only`.
  `read_only` проходит только по белому списку методов чтения
  (`readOnlyMethodWhitelist` в `internal/app/grpc.go` — новый метод по
  умолчанию закрыт), значения item/config_item маскируются в usecase
  (`value_size`/`value_hash`), в том числе значения config_item в записях
  `GET /audit`; MCP такие ключи не принимает. `full` и `read_only` выпускает
  только админ.
- **Ручки мониторинга** (контракт для pulse — `docs/monitoring-api.md`):
  `GET /audit`, `GET /sync-run[/{id}]`, `GET /app/resolve`, `GET /app/{id}/keys`,
  `GET /app/{id}/drift`; фильтры `updated_at_gte/lt` во всех List-методах.
  Drift, как и просмотр живого объекта, не-админу не сравнивает объект
  кластера, записанный kusec не для этой записи (`foreign=true`, списки
  ключей пусты); sync не-админа не усыновляет объект без лейбла kusec.
- **Env**: `AUDIT_HASH_KEY` обязателен (сервис не стартует без него; менять
  нельзя — сломается сравнимость отпечатков), `AUDIT_RETENTION_DAYS`.

### Вход, сессии и доступ к приложениям

- **Сессия строится на каждый запрос по записи пользователя.** Access-JWT
  несёт только `id` и `ver` (= `usr.session_version`); права (админ, список
  приложений) берутся из базы. Выключенный или удалённый пользователь и токен
  с другим `ver` отвергаются сразу, смена прав действует со следующего
  запроса. Интерсептор зовёт `usr.Usecase.SessionFromToken` (`UsrAuthI` в
  `internal/app/grpc.go`). Токены без `ver` (выданные до этой схемы) считаются
  `ver=0`. Сбой базы при проверке токена или ключа — ответ
  `service_not_available`, а не анонимный запрос (иначе короткая недоступность
  Postgres разлогинивала бы всех); порядок интерсепторов: recovery → error →
  session → read_only.
- `session_version` растёт при смене пароля, выключении пользователя и
  выходе. `POST /usr/logout` гасит все токены пользователя на всех
  устройствах.
- **Сроки браузерной сессии** (`internal/domain/session/service`): access —
  15 мин, refresh — 30 дней без обращений, но общий срок сессии — 7 дней от
  входа по паролю (`maxSessionAge`). Время входа лежит в refresh-токене
  (claim `ast`) и при продлении переносится в новый токен, поэтому украденный
  refresh-токен нельзя продлевать бесконечно; после 7 дней — вход заново.
  У refresh-токенов без `ast` время входа — их `iat`. Старый refresh-токен
  при продлении не гасится (живёт до своего `exp`). API-ключей и
  OAuth-подключений MCP этот срок не касается.
- **Доступ к приложениям**: админ — всё; не-админ с `usr.all_apps=true` — всё;
  иначе только `app_ids`, **пустой список = ничего**.
  `Session.AccessibleAppIds()` может вернуть (пусто, false) — вызывающий
  обязан сам дать пустой результат или отказ: репозитории пустой срез
  фильтром не считают, а kube-сервис понимает его как «все приложения».
- **Защита входа** (`internal/usecase/usr/limiter.go`, в памяти процесса):
  5 неверных паролей подряд → блокировка 1 мин, дальше 2, 4, 8,
  15 мин; успех сбрасывает. Ответ — `too_many_attempts` с
  `fields.retry_after` (секунды). Счётчик существующего пользователя ведётся
  по его id и чужими попытками не вытесняется; несуществующего логина — по
  логину с учётом регистра (как сравнивает база), в ограниченной карте.
  Снаружи несуществующий логин неотличим от существующего. То же ограничение
  на текущий пароль (`PUT /usr/profile`, выпуск API-ключа, согласие OAuth).
  Одновременных проверок пароля на входе — не больше `max(GOMAXPROCS, 2)`
  (`gate.go`): лишние ждут до 3 с, затем `service_not_available`. Журнал:
  `login failed` (WARN: username, client_ip, reason), `login ok`, `logout`.
  Блокировки живут в памяти: перезапуск сервиса их снимает.
- `PUT /usr/profile`: `current_password` обязателен при смене пароля или
  логина, иначе `current_password_invalid`. Смена пароля гасит все сессии,
  включая текущую.
- Выпуск API-ключа (`POST /api-key`) и согласие на OAuth-подключение MCP
  (`POST /oauth/authorize` с `approve=true`) требуют `current_password`
  (`usr.Usecase.ConfirmPassword`, ограничитель общий с профилем): ключ и
  подключение живут до удаления и переживают выход и смену пароля, поэтому
  украденная сессия не должна превращаться в бессрочный доступ.
  Свои пароль и логин админ меняет только в профиле: `PUT /usr/{id}` со
  своим паролем или логином — `no_permission`.
- Сессия по API-ключу или OAuth-подключению (`Session.ApiKeyId != ""`) не
  управляет учётками и ключами (профиль, `Usr.Create/Update/Delete`,
  `ApiKey.Create/Update/Delete`) — `no_permission`. Чтение (`GetProfile`,
  `ApiKey.List`) разрешено.
- `GET /usr` и `GET /usr/{id}` — только админ; свои данные — `GET /usr/profile`.
  Не-админ (в том числе с `all_apps`) в `GET /audit` видит только записи с
  приложением — записи об учётках, ключах и подключениях ему не отдаются;
  число пользователей в `GET /dashboard` — только админу.
- Поиск item (`search`) — не длиннее 200 символов, иначе `invalid_request`.
- 2FA нет (убрана миграцией `000013`): вход — только логин и пароль.

### Шифрование значений в базе

- `DATA_ENCRYPTION_KEY` — ключ шифрования `item.value`
  (AES-256-GCM, пакет `internal/infra/secretbox`). Формат: base64 от 32
  случайных байт (`openssl rand -base64 32`). **Пусто — шифрование выключено**,
  сервис работает как раньше; неверный формат — сервис не стартует.
- Шифрует и расшифровывает репозиторий; всё выше работает с открытым
  значением, отпечатки в аудите и журнале sync не меняются. Формат хранения —
  `enc:v1:` + base64(nonce ‖ шифртекст), контекст колонки уходит в AAD. Пустое
  значение остаётся пустым. Без ключа нельзя сохранить значение item,
  начинающееся с `enc:v1:`.
- При старте с ключом (после миграций, до серверов) ещё не зашифрованные
  строки шифруются пачками — `applyValueEncryption` в
  `internal/app/encryption.go`; повторный старт ничего не меняет. Старт без
  ключа или с чужим ключом на базе с зашифрованными строками — отказ запуска.
- Поиск по значению item при включённом шифровании идёт в памяти
  (`internal/domain/item/repo/db/search.go`), без шифрования — прежний SQL.
- **После включения ключ нельзя менять, убирать и терять, а откатываться на
  образ без поддержки шифрования нельзя**: старая версия отправила бы в
  кластер шифртекст как значение секрета. `config_item.value` не шифруется
  (несекретное).
- **Порядок включения — в два шага**: сначала выкатить версию с поддержкой
  шифрования БЕЗ ключа во все кластеры, и только потом задать ключ и
  перезапустить. Если задать ключ одновременно с первой выкладкой, новый под
  зашифрует значения, пока старый под (старый код) ещё обслуживает запросы —
  sync в это окно отправил бы шифртекст в кластер. Перед включением: копия
  базы и проверка `select count(*) from item where value like 'enc:v1:%'`
  (должно быть 0 — иначе сервис не стартует). После включения:
  `select count(*) from item where value <> '' and value not like 'enc:v1:%'`
  (должно быть 0, иначе перезапустить ещё раз).

### Защита периметра

- Админка отдаётся с защитными заголовками (`internal/app/spa.go`):
  `Content-Security-Policy` (скрипты, запросы и картинки — только со своего
  хоста; `'unsafe-inline'` — только для стилей, их вставляют Naive UI и
  CodeMirror), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`, `Permissions-Policy`; по https — ещё
  `Strict-Transport-Security`. Внешний скрипт, шрифт, CDN или API на другом
  хосте требуют правки `adminCSP`.
- `JWT_SECRET` короче 32 символов — предупреждение при старте (запуск не
  блокируется). Смена секрета разлогинивает всех и отключает OAuth-клиентов MCP.
- Образ собирается на закреплённой версии `alpine`; контейнер работает от
  root (порт по умолчанию 80) — смена пользователя требует согласованной
  правки чарта.

### Тестовый стенд

Интеграционные тесты (`internal/integration`) гоняются на одноразовом docker
Postgres и пропускаются без `TEST_PG_DSN`:

```
docker run --rm -d --name kusec-test-pg \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=kusec_test \
  -p 55432:5432 postgres:17

TEST_PG_DSN='postgres://postgres:postgres@localhost:55432/kusec_test?sslmode=disable' \
  go test ./internal/integration/

docker stop kusec-test-pg
```

- контейнер: `kusec-test-pg`, порт `55432`, логин/пароль `postgres`/`postgres`,
  БД `kusec_test`; миграции тесты применяют сами (`migrations/`).
- Набор требует чистой базы на каждый прогон (у приложений фиксированные
  slug-и): повторный прогон — после
  `docker exec kusec-test-pg psql -U postgres -d kusec_test -c 'drop schema public cascade; create schema public;'`
  либо на новом контейнере.
- `TEST_DATA_ENCRYPTION_KEY="$(openssl rand -base64 32)"` вместе с
  `TEST_PG_DSN` гоняет те же тесты поверх зашифрованных значений — прогоняй
  оба режима.

### Dev-стенд админки

Локальный бэкенд + vite на одноразовом docker Postgres с постоянными кредами —
для ручной/визуальной проверки фронта. Пересоздаётся с нуля за минуту:

```
docker run --rm -d --name kusec-dev-pg \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=kusec_dev \
  -p 55433:5432 postgres:17

PG_DSN='postgres://postgres:postgres@localhost:55433/kusec_dev?sslmode=disable' \
  JWT_SECRET=kusec-dev-jwt AUDIT_HASH_KEY=kusec-dev-audit \
  HTTP_PORT=18080 GRPC_PORT=15050 \
  HTTP_CORS=true HTTP_CORS_ALLOWED_ORIGINS=http://127.0.0.1:5199 \
  go run ./cmd/.

python3 scripts/devstand/seed.py      # админ + демо-app с секретами/конфигмапами

cd apps/admin && VITE_API_BASE_URL=http://127.0.0.1:18080/api \
  pnpm exec vite --port 5199 --host 127.0.0.1 --strictPort

docker stop kusec-dev-pg              # стенд сносится вместе с данными
```

- контейнер `kusec-dev-pg`, порт `55433`, БД `kusec_dev` (`postgres`/`postgres`);
  бэкенд — HTTP `18080` (API под `/api`), gRPC `15050`; админка — `http://127.0.0.1:5199`.
- MCP с OAuth на стенде: добавить к бэкенду `MCP_ENABLED=true MCP_PORT=15060
  MCP_OAUTH_ENABLED=true MCP_PUBLIC_URL=http://localhost:15060
  MCP_OAUTH_ADMIN_URL=http://127.0.0.1:5199`. Клиента регистрировать с адресом возврата
  `http://localhost:18099/callback` (loopback разрешён по умолчанию; там никто не слушает —
  код берётся из адресной строки после «Allow»), страница согласия —
  `http://127.0.0.1:5199/oauth/authorize?...`; «Allow» — с паролем `admin12345`.
- вход в админку: `admin` / `admin12345`.
- креды только для этого одноразового стенда, нигде больше не использовать.

### Flow проверки изменений
```
make generate-proto  →  gofmt  →  go test ./...  →  go run ./cmd/.
```
