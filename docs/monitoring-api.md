# Monitoring API (контракт для pulse)

Read-only API kusec для мониторинга и расследования инцидентов: аудит
изменений, журнал sync, ключи приложения без значений, сравнение с кластером.

## Доступ

- Базовый URL внутри кластера: `http://kusec.kusec/api`.
- Аутентификация: `Authorization: Bearer ksk_…` — API-ключ со scope
  `read_only` (выпускается админом в админке, раздел «API-ключи»).
- Ключ `read_only` проходит только по белому списку методов чтения
  (см. ниже); любая мутация, `Kube.*` (включая чтение значений из кластера) и
  прочие методы возвращают `no_permission`. Значения item/config_item в
  ответах не отдаются: вместо них `value_size` (байты) и `value_hash`.
- Пагинация: `list_params.page` **с 0**, `list_params.page_size` обязателен и
  ≤ 100 (`list_params.with_total_count=true` — вернуть `total_count`).
  В query grpc-gateway это `list_params.page=0&list_params.page_size=100`.
- Времена — RFC3339 (`2026-09-17T10:00:00Z`), и в ответах, и в фильтрах.
- Формат ошибок: `{"code": "...", "message": "..."}` (HTTP 400).
- Нулевые поля присутствуют в JSON (`EmitUnpopulated`), **кроме optional-полей
  с явным presence**: незаполненные `not_synced_since`, `finished_at`,
  `batch_id`, `actor_api_key_id`, `old`/`new` и т.п. в JSON **отсутствуют**
  (не `null`) — читайте их как необязательные ключи.

Белый список read_only: `App/List|Get|Resolve|Keys|Drift`, `Secret/List|Get`,
`ConfigMap/List|Get`, `Item/List|Get`, `ConfigItem/List|Get`, `Audit/List`,
`SyncRun/List|Get`, `Transfer/Tree`.

## Отпечатки значений (value_hash / content_hash)

HMAC-SHA256 с серверным ключом (`AUDIT_HASH_KEY`), усечённый до 16 hex.
Значение по отпечатку не восстанавливается и не перебирается; отпечатки
сравнимы между собой во всех ответах (аудит, keys, журнал sync) — «значение
поменялось / вернулось к прежнему» видно без значений. `content_hash`
объекта — отпечаток канонизированного содержимого (отсортированные пары
ключ/значение); он же пишется в аннотацию `kusec.io/content-hash` k8s-объекта.

## Аннотации на синхронизированных k8s-объектах

Ставятся при создании/обновлении объекта sync-ом (на unchanged не
обновляются):

| Аннотация | Значение |
|---|---|
| `kusec.io/synced-at` | RFC3339 времени запуска sync, изменившего объект |
| `kusec.io/sync-run-id` | id записи `GET /sync-run/{id}` |
| `kusec.io/content-hash` | отпечаток содержимого на момент применения |
| `kusec.io/app-id`, `kusec.io/secret-id` / `kusec.io/configmap-id` | id записей kusec |

Reloader перезапускает поды после изменения объекта — аннотация
`reloader.stakater.com/last-reloaded-from` на pod template укажет объект,
а `kusec.io/sync-run-id` свяжет его с запуском sync и записями аудита
(`batch_id` = run id).

## GET /audit — лента изменений

Фильтры (все опциональны): `app_id`, `namespace`, `app_slug`, `kube_name`,
`entity_type` (`app|secret|item|configmap|config_item|api_key|usr|sync_run`),
`entity_id`, `action` (`create|update|delete|activate|deactivate|sync|import`),
`actor_usr_id`, `key`, `batch_id`, `created_at_gte`, `created_at_lt`.
Сортировка по умолчанию — новые первыми (`-created_at`).

Пример: что менялось у сервиса за последние 6 часов:

```
GET /api/audit?app_id=<id>&created_at_gte=2026-09-17T04:00:00Z&list_params.page_size=100
```

Запись:

```json
{
  "id": "42",
  "created_at": "2026-09-17T09:12:30Z",
  "actor_usr_id": "1",
  "actor_api_key_id": null,
  "actor_name": "Dauren",
  "source": "ui",
  "request_id": "…",
  "entity_type": "item",
  "entity_id": "…",
  "app_id": "…",
  "namespace": "caravan",
  "app_slug": "caravan",
  "kube_kind": "Secret",
  "kube_name": "kusec-caravan-main",
  "key": "PG_PASSWORD",
  "action": "update",
  "changes": [
    {"field": "value", "old_hash": "a1b2…", "new_hash": "c3d4…", "old_size": "32", "new_size": "48"}
  ],
  "batch_id": null
}
```

Правила `changes`:
- несекретные поля — `old`/`new` как есть (`null` — стороны нет:
  create/delete);
- значение item (secret) — только `old_hash/new_hash/old_size/new_size`;
- значение config_item — целиком, но длиннее 4 КБ — отпечаток и
  `truncated: true`;
- пароль/TOTP пользователя — маркер `***` (только факт смены);
- `batch_id` группирует каскадное удаление, импорт и sync (для sync
  `batch_id` = id запуска).

## GET /sync-run и GET /sync-run/{id} — журнал sync

Фильтры списка: `app_id`, `namespace` (запуски, затронувшие объекты
namespace), `status` (`running|ok|partial|error`), `started_at_gte/lt`.
Сортировка по умолчанию `-started_at`. `app_id: null` — sync всех доступных
app. `Get` дополнительно отдаёт `objects`:

```json
{
  "namespace": "caravan",
  "kube_kind": "Secret",
  "kube_name": "kusec-caravan-main",
  "op": "updated",
  "error": "",
  "content_hash": "c3d4…",
  "changed_keys": ["PG_PASSWORD", "PG_DSN"]
}
```

`op`: `created|updated|deleted|unchanged|error`. `changed_keys` для
created/deleted — все ключи объекта.

## GET /app/resolve?namespace=&kube_name= — объект кластера → app

pulse видит `kusec-caravan-main` в `envFrom` пода или в аннотации reloader:

```
GET /api/app/resolve?namespace=caravan&kube_name=kusec-caravan-main
```

```json
{
  "found": true,
  "app": {"id": "…", "namespace": "caravan", "slug_name": "caravan", "name": "Caravan", …},
  "kube_kind": "Secret",
  "object_id": "…",
  "object_slug": "main"
}
```

`found: false` — объект в kusec не описан (например, чужой секрет).

## GET /app/{id}/keys — ключи app без значений

```json
{"keys": [{
  "key": "PG_PASSWORD",
  "kube_kind": "Secret",
  "kube_name": "kusec-caravan-main",
  "is_secret": true,
  "active": true,
  "value_size": "48",
  "value_hash": "c3d4…",
  "updated_at": "2026-09-17T09:12:30Z",
  "updated_by": "Dauren",
  "synced": false,
  "item_id": "…",
  "parent_id": "…"
}]}
```

- `active` — ключ попадает в кластер (активны и ключ, и его secret/configmap);
- `updated_by` — последний актор из аудита (пусто — изменений после
  включения аудита не было);
- `synced` — изменение применено: `updated_at` не позже последнего sync
  объекта.

## GET /app/{id}/drift — kusec ↔ кластер

```json
{
  "in_cluster": true,
  "objects": [{
    "kube_kind": "Secret",
    "kube_name": "kusec-caravan-main",
    "namespace": "caravan",
    "object_id": "…",
    "exists_in_cluster": true,
    "managed": true,
    "missing_in_cluster": ["NEW_KEY"],
    "extra_in_cluster": [],
    "value_differs": ["PG_PASSWORD"],
    "not_synced_since": "2026-09-17T09:12:30Z"
  }]
}
```

- сравнение по байтам значений, наружу отдаются **только имена ключей**;
- `not_synced_since` — время последнего изменения объекта/его ключей в
  kusec, ещё не применённого в кластер (`null` — расхождений по времени нет);
- `in_cluster: false` — kusec запущен вне кластера, сравнение недоступно.

## Фильтры updated_at в List-методах

`GET /app`, `/secret`, `/configmap`, `/item`, `/config-item` (пути точные:
`/item?app_id=…`) принимают `updated_at_gte` и `updated_at_lt` (RFC3339) —
«что менялось за окно» без аудита.

## Метрики (системный порт 3003, /metrics)

С префиксом `<METRICS_NAMESPACE>_kusec_`:

- `changes_total{entity_type, action, source}` — счётчик записей аудита;
- `sync_runs_total{status}`, `sync_duration_seconds` — запуски sync;
- `last_sync_timestamp_seconds{namespace, app}` — время последнего успешного
  sync по app;
- `unsynced_objects{namespace, app}` — объекты, изменённые после последнего
  sync.

## MCP

Встроенный MCP-сервер kusec (порт `MCP_PORT`) дополнительно даёт read-only
инструменты `audit_list` и `sync_run_list` с теми же фильтрами.
