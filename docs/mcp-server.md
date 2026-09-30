# Встроенный MCP-сервер kusec

MCP-сервер (Model Context Protocol) для AI-агентов **встроен в процесс kusec** и работает
на отдельном HTTP-порту (streamable HTTP transport). Агент видит структуру
app/secret/configmap/item и управляет ею, но **значения секретов не видит никогда** —
ни существующие, ни создаваемые.

- Код: `internal/handler/mcp` — транспортный слой поверх usecase-слоя (значения секретов
  не покидают процесс).
- Включение: `MCP_ENABLED=true`, порт: `MCP_PORT` (по умолчанию `5060`).

## Модель безопасности

1. **Чтение**: у item-ов секретов поле `value` вырезается; вместо него — `value_chars`
   (длина в символах), `value_bytes` и `value_hash` (усечённый HMAC-отпечаток с серверным
   ключом `AUDIT_HASH_KEY` — для сравнения значений между собой; сравним с `value_hash`
   основного API, аудита и журнала sync). Значения item-ов **конфигмапов** не секретны
   и видны полностью.
2. **Запись значений** — только декларативно через `value_source`:
   - `generate` — случайное значение через crypto/rand (`format`: `alnum` | `ascii` |
     `digits` | `hex` | `base64url` | `uuid`; `length`, по умолчанию 32). При указании
     `name` сохраняется в реестре сессии для переиспользования.
   - `reuse` — взять значение из реестра сессии по `name` (тот же токен для нескольких
     item-ов, в том числе в разных app). Реестр живёт в памяти MCP-сессии.
   - `copy_item` — скопировать значение существующего item-а по `item_id` (из любого
     app, доступного пользователю). С `name` — также зарегистрировать для reuse.
   - `template` — собрать значение из шаблона с плейсхолдерами `{{имя}}` (например
     DSN с паролем БД). Каждая переменная резолвится через `vars` (те же kind-ы:
     `generate`/`reuse`/`copy_item`/`literal`) либо, без записи в `vars`, из реестра
     сессии по имени. С `name` — итог также регистрируется для reuse. Итоговое
     значение агенту не показывается.
   - `literal` — явное значение от агента (для несекретного: хосты, порты, url).
3. **Аутентификация — по api-ключу** (`Authorization: Bearer ksk_…`) на каждом запросе.
   Сессия наследует права владельца ключа (`app_ids`, `is_admin`) — все проверки
   usecase-слоя работают как для обычного пользователя. Ключ со `scope="mcp_only"`
   принимается **только** MCP-эндпоинтом: основной API его отвергает, поэтому агент,
   даже достав ключ из своего окружения, не может обойти маскирование прямым запросом
   к API. Подмена MCP-сессии исключена: ключ каждого запроса сверяется с ключом,
   которым сессия была открыта.
4. **Scope записи**: create/update работают в любом app, доступном владельцу ключа
   (`app_ids`, `is_admin` — проверки usecase-слоя); `create_secret`/`create_configmap`
   принимают целевой `app` явно. Delete-инструментов нет.
5. **Ошибки** фильтруются: из текстов вычищаются все значения секретов, прошедшие через
   сессию (`[REDACTED]`).

Оговорка: точная длина значения раскрывается — осознанный выбор. Отпечаток — HMAC с
серверным ключом, словарный перебор без ключа невозможен.

## Инструменты (30)

| Группа | Инструменты |
|---|---|
| Чтение | `list_app`, `get_app`, `list_secret`, `get_secret`, `list_configmap`, `get_configmap`, `list_item`, `get_item` (маскированные), `list_config_item`, `get_config_item` |
| Сессия | `list_value_name` — имена значений в реестре сессии для reuse |
| Запись | `create_app`, `update_app`, `create_secret`, `update_secret`, `create_configmap`, `update_configmap`, `create_item`, `update_item`, `create_config_item`, `update_config_item` |
| Кластер | `sync` — применить секреты и конфигмапы в Kubernetes |
| Импорт из кластера (только админ) | `list_cluster_secret`, `list_cluster_configmap` — объекты, уже живущие в кластере (имена ключей, без значений); `import_secret`, `import_configmap` — перенести один объект в kusec; `import_secret_batch`, `import_configmap_batch` — несколько объектов за один вызов |
| Мониторинг | `audit_list` — аудит изменений (без значений), `sync_run_list` — журнал запусков sync |

`sync` вызывает тот же usecase, что и `POST /kube/sync`: синхронизирует указанный
`app` (id, slug_name или имя), с `all_apps=true` — все app, доступные владельцу ключа
(`syncScope`). Работает только когда kusec запущен внутри кластера; лок «один sync
одновременно» общий с основным API. Результат содержит только имена объектов
(`namespace/name`) и счётчики; тексты ошибок по отдельным объектам проходят скраб
значений секретов.

### Импорт из кластера (`list_cluster_*`, `import_*`, `import_*_batch`)

Перенос уже существующих секретов кластера в kusec агентом или скриптом. Оба
инструмента — обёртки над тем же сервисом, что `GET /kube/cluster-secret` и
`POST /kube/import-secret` (кнопка «Import from cluster» в админке); доступны
**только админу** (MCP-ключ наследует права владельца). Значения из кластера идут
`service → БД` внутри процесса kusec: в ответ инструмента, в реестр значений сессии,
в логи и в тексты ошибок они не попадают — там только имена ключей.

`list_cluster_secret {"namespace": "…"}` (namespace необязателен, пусто — все, кроме
системных `kube-*`; служебные типы — токены ServiceAccount, helm-релизы — скрыты):

```
{"in_cluster": true,
 "secrets": [{"namespace": "billing", "name": "billing-db", "type": "",
              "keys": ["POSTGRES_PASSWORD", "POSTGRES_USER"], "managed": false}]}
```

`import_secret`:

| Поле | Описание |
|---|---|
| `app` | целевой app: id, slug_name или имя (как в `create_secret`) |
| `namespace`, `name` | секрет-источник в кластере |
| `secret_slug` | целевой секрет kusec: новый (создаётся, `kube_type` копируется из источника, slug валидируется как DNS-1123) или существующий (дозаполняется) |
| `keys` | необязательно; карта «ключ в кластере → имя item-а»: подмножество и переименование. Пустое имя — оставить как в кластере. Не задано — все ключи 1:1. Ключ, которого нет в источнике, — ошибка со списком недостающих имён; имена item-ов валидируются как ключи k8s-секрета |
| `overwrite` | необязательно, по умолчанию `false`: создать отсутствующие item-ы и заполнить существующие **пустые**, непустые не трогать (`skipped`). `true` — перезаписать совпавшие непустые значением из кластера (как REST-импорт и UI) |

Ответ — метаданные секрета и **имена** ключей по категориям (значений нигде нет):

```
{"secret_id": "…", "secret_slug": "db", "secret_created": false,
 "kube_type": "", "kube_secret_name": "kusec-billing-db",
 "created": ["POSTGRES_USER"], "filled": ["POSTGRES_PASSWORD"],
 "updated": [], "skipped": ["PG_DSN"]}
```

Импорт выполняется в одной транзакции и пишет аудит `action=import` с общим
`batch_id` на секрет и каждый item — как и REST-путь. REST `POST /kube/import-secret`
и UI по-прежнему переносят все ключи и перезаписывают совпавшие (сервису передаётся
`Overwrite: true`).

**Пакет** — `import_secret_batch`: несколько секретов кластера в один `app` за один вызов.
Общие `namespace` и `overwrite` задаются на пакет, у каждого секрета в `secrets` — свои
`name`, при необходимости `secret_slug` (по умолчанию равен `name`), `keys` и `overwrite`
(переопределяет общий). Семантика каждого секрета — как у `import_secret`. Секреты
обрабатываются по порядку, **каждый в своей транзакции**: ошибка одного (нет в кластере,
лишний ключ в `keys`, невалидный slug, сбой записи) попадает в его `error`, остальные
переносятся; повторный запуск с теми же спеками идемпотентен (уже перенесённое уходит в
`skipped`). Все записи аудита пакета получают общий `batch_id`. Один и тот же `secret_slug`
может встретиться несколько раз — каждый следующий дозаполняет предыдущий. На весь вызов
ошибка возвращается только по общим предусловиям: kusec вне кластера, app не найден,
пустой пакет (или больше 500 секретов), секрет без `name`/`namespace`.

```
import_secret_batch {"app": "billing", "namespace": "billing", "overwrite": false,
                     "secrets": [{"name": "billing-db", "secret_slug": "db",
                                  "keys": {"POSTGRES_PASSWORD": "", "POSTGRES_USER": "DB_USER"}},
                                 {"name": "billing-redis"},
                                 {"namespace": "shared", "name": "smtp", "overwrite": true}]}
→ {"ok": 2, "failed": 1,
   "results": [{"namespace": "billing", "name": "billing-db", "error": "",
                "secret_id": "…", "secret_slug": "db", "secret_created": true, …,
                "created": ["DB_USER", "POSTGRES_PASSWORD"], "filled": [], "updated": [], "skipped": []},
               {"namespace": "billing", "name": "billing-redis", "error": "",
                "secret_id": "…", "secret_slug": "billing-redis", …},
               {"namespace": "shared", "name": "smtp", "secret_slug": "smtp",
                "error": "get cluster secret shared/smtp: … not found",
                "secret_id": "", "created": [], "filled": [], "updated": [], "skipped": []}]}
```

Каждый элемент `results` содержит все поля ответа `import_secret` плюс `namespace`, `name`
и `error` (пустая строка = успех); у упавшего секрета заполнены только источник,
`secret_slug` и `error`, списки ключей пустые.

**Configmap-ы** — те же три инструмента с теми же правилами (`keys`, `overwrite`,
категории `created / filled / updated / skipped`, пакет с ошибкой по элементу, аудит
`action=import`): `list_cluster_configmap` (ответ `{in_cluster, configmaps: [{namespace,
name, keys, managed}]}`, служебный `kube-root-ca.crt` скрыт), `import_configmap` и
`import_configmap_batch`. Отличия только в именах: `configmap_slug` вместо `secret_slug`,
список `configmaps` вместо `secrets`, в ответе `configmap_id`, `configmap_slug`,
`configmap_created`, `kube_configmap_name`, поля `kube_type` нет. Ключи `data` ложатся
текстом (`encoding=plain`), `binaryData` — в base64. REST-аналоги для админки:
`GET /kube/cluster-configmap` и `POST /kube/import-configmap` (все ключи, совпавшие
перезаписываются — как у секретов).

Значения item-ов configmap-ов в MCP не маскируются. Если в configmap-е кластера лежит
что-то чувствительное (DSN с паролем, токен), после `import_configmap` агент сможет
прочитать это через `list_config_item`. Такие ключи не переносите в configmap: исключите
их через `keys` и заведите в секрете.

```
import_configmap_batch {"app": "billing", "namespace": "billing",
                        "configmaps": [{"name": "billing-config", "configmap_slug": "config"},
                                       {"name": "billing-nginx", "keys": {"nginx.conf": ""}}]}
```

Сценарий «взять под управление kusec живой секрет `billing-db`»:

```
list_cluster_secret {"namespace": "billing"}
import_secret {"app": "billing", "namespace": "billing", "name": "billing-db",
               "secret_slug": "billing-db"}
update_secret {"id": "<secret_id>", "exact_slug": true}   # имя k8s-секрета = slug как есть
sync {"app": "billing"}                                   # kusec усыновляет объект, данные те же
```

`exact_slug` (только админ) доступен в `create_secret`/`update_secret` и
`create_configmap`/`update_configmap`. Включить его у секрета с активными item-ами
без значений нельзя (`invalid_request` с объяснением): иначе sync усыновил бы живой
объект с тем же именем и затёр его пустыми значениями. Сначала значения (например,
через `import_secret`), потом флаг.


Пагинация zero-based (`page` с 0), `page_size` по умолчанию 100.

При `initialize` сервер отдаёт клиенту `instructions` (константа `serverInstructions`
в `internal/handler/mcp/handler.go`) — правила работы для агента: адресация записи
по `app`/id, семантика `value_source`, запрет на запрос/придумывание значений секретов,
импорт из кластера через `import_secret` (а не руками через `literal`). Отдельно
прописывать эти правила в CLAUDE.md проектов-потребителей не нужно — туда имеет смысл
выносить только проектные соглашения (какой app какому чарту соответствует и т.п.).

## API-ключи (`/api-key`)

Управлять ключами можно в админке (пункт «API keys» в шапке): выпуск с показом значения
один раз, включение/отключение, удаление; админ видит ключи всех пользователей и может
выпускать ключи для них. Ниже — то же самое через API.

Долгоживущие учётные данные для машинных клиентов. В БД хранится только sha256-хэш;
значение ключа возвращается **один раз** при создании. Ключ наследует права владельца.
2FA на api-key-аутентификацию не распространяется, поэтому агентские ключи создавайте
со `scope="mcp_only"` и/или на пользователя с минимальным набором app.

Область доступа ключа задаёт `scope`: `full` (полный доступ), `read_only` (белый список
методов чтения основного API, значения маскируются; MCP такие ключи не принимает — см.
docs/monitoring-api.md) и `mcp_only` (только MCP-эндпоинт). `full` и `read_only`
выпускают **только админы**; не-админ может создать себе лишь `mcp_only`-ключ и может
только сузить свой `full`-ключ через Update (иначе `no_permission`). Поле `mcp_only`
в API осталось как deprecated-алиас scope.

```bash
# выпустить агентский ключ (Bearer — обычный JWT после логина)
curl -X POST https://kusec.example.com/api/api-key \
  -H "Authorization: Bearer $JWT" \
  -d '{"name": "mcp agent mdb", "scope": "mcp_only"}'
# → {"id": "...", "key": "ksk_<hex>"}  — key больше нигде не получить

# список своих ключей (админ видит все, есть фильтр usr_id)
curl "https://kusec.example.com/api/api-key?list_params.page_size=100" -H "..."

# отключить / удалить
curl -X PUT    .../api/api-key/{id} -d '{"active": false}' -H "..."
curl -X DELETE .../api/api-key/{id} -H "..."
```

Не-админ управляет только своими ключами; админ может выпускать ключи другим
пользователям (`usr_id` в Create). `last_used_at` обновляется при использовании
(не чаще раза в минуту). Отзыв: `active=false` или удаление — действует сразу.

## Конфигурация kusec (env)

| Переменная | Описание |
|---|---|
| `MCP_ENABLED` | `true` — поднять MCP-эндпоинт (по умолчанию выключен). |
| `MCP_PORT` | Порт MCP HTTP-сервера (по умолчанию `5060`). |

Порт выставляется наружу отдельно от основного API (ingress/service) — можно дать
агентам доступ только к MCP, не открывая `/api`.

## Подключение агента (Claude Code)

`.mcp.json` проекта (ключ — через подстановку из окружения, не строкой в репо):

```json
{
  "mcpServers": {
    "kusec": {
      "type": "http",
      "url": "https://kusec-mcp.example.com/",
      "headers": {
        "Authorization": "Bearer ${KUSEC_MCP_API_KEY}"
      }
    }
  }
}
```

Типовой сценарий агента:

```
create_secret {"app": "billing", "slug_name": "db", "description": "Postgres"}
create_item {"secret_id": "<id>", "key": "POSTGRES_PASSWORD",
             "value_source": {"kind": "generate", "name": "db_password", "length": 32}}
create_item {"secret_id": "<id2>", "key": "DATABASE_URL_PASSWORD",
             "value_source": {"kind": "reuse", "name": "db_password"}}
create_item {"secret_id": "<id2>", "key": "PG_DSN",
             "value_source": {"kind": "template",
                              "template": "postgres://billing:{{db_password}}@pg:5432/billing"}}
sync {"app": "billing"}
```

Сборка `PG_DSN` из **уже существующего** `PG_PASSWORD` (значение заведено ранее,
в реестре сессии его нет) — переменная описывается в `vars` через `copy_item`:

```
create_item {"secret_id": "<id>", "key": "PG_DSN",
             "value_source": {"kind": "template",
                              "template": "postgres://app:{{pg_password}}@pg:5432/app",
                              "vars": {"pg_password": {"kind": "copy_item",
                                                       "item_id": "<id item-а PG_PASSWORD>"}}}}
```
