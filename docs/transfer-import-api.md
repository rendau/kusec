# Kusec API: POST /transfer/import

Массовая заливка данных в Kusec (Secret Management System): дерево
app → secret → item одним запросом, upsert по натуральным ключам.
Документ самодостаточен — можно отдать внешнему инструменту/ИИ-агенту.

## Базовый URL и аутентификация

Пути указаны относительно базового URL HTTP API:

| Окружение | База |
|---|---|
| Через админку / ingress | `http://<host>/api` |
| Прямо в HTTP-gateway сервиса | `http://<host>:8080/api` |

Запрос требует JWT в заголовке `Authorization: Bearer <jwt>`.
Доступ: **только администратор** (иначе код `no_permission`).

Получение токена:

```sh
curl -s -X POST "$BASE/usr/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"<user>","password":"<pass>"}'
# → {"jwt":"...","refresh_token":"..."}
```

Access-токен живёт ~15 минут. При ответе с кодом `not_authorized` —
обновить пару токенов:

```sh
curl -s -X POST "$BASE/usr/token/refresh" \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'
# → новая пара {"jwt":"...","refresh_token":"..."}
```

## Общие соглашения

- **int64-поля сериализуются строками** (`"unchanged": "2"`).
- Ошибки запроса приходят с HTTP 400 и телом:
  ```json
  { "code": "not_authorized", "message": "...", "fields": {} }
  ```
  Значимые коды: `not_authorized` (нет/просрочен токен), `no_permission`
  (нужна роль администратора), `invalid_request` (пустой запрос).

## Доменная модель

```
app (приложение)        — namespace, name, slug_name, description, active
└─ secret (секрет)      — slug_name, description, active
   └─ item (ключ)       — key, value, value_format, encoding, file_name,
                          content_type, description, active
```

- Натуральные ключи: app — пара `(namespace, slug_name)` (уникальна);
  secret — `slug_name` внутри app; item — `key` внутри secret.
- При синхронизации в Kubernetes секрет получает имя
  `{app.slug_name}-{secret.slug_name}` в namespace `app.namespace`,
  ключи — из item-ов; участвуют только записи с `active=true`.

## Семантика импорта

- Сопоставление по натуральным ключам: найдена запись — обновляется,
  нет — создаётся. **`id` в запросе не передаются.**
- Обновление происходит **только при реальных отличиях** полей;
  полностью совпадающие записи считаются `unchanged` — повторный импорт
  идемпотентен.
- Импорт **ничего не удаляет**: записи базы, отсутствующие в запросе,
  не трогаются.
- Ошибки отдельных записей не прерывают импорт — они собираются в
  `errors` с путём `namespace/app-slug/secret-slug/key`.

## Поля и дефолты

| Поле | Обязательное | Дефолт при создании |
|---|---|---|
| app: `namespace`, `slug_name`, `name` | да | — |
| app/secret/item: `active` | нет | `true` |
| secret: `slug_name` | да | — |
| item: `key` | да | — |
| item: `value` | нет | `""` |
| item: `value_format` | нет | `text` (допустимы `text`/`yaml`/`json`) |
| item: `encoding` | нет | `plain` (допустимы `plain`/`base64`) |
| `description`, `file_name`, `content_type` | нет | `""` |

Важно про `encoding=base64`: значение должно быть **уже** закодировано в
base64 (так оно хранится в системе) — импорт не кодирует на лету.
`value_format` — подсказка редактора, на содержимое не влияет.

## Запрос

```sh
curl -s -X POST "$BASE/transfer/import" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d @payload.json
```

`payload.json` (пример):

```json
{
  "apps": [
    {
      "namespace": "payments",
      "name": "Payments API",
      "slug_name": "payments-api",
      "description": "card processing",
      "active": true,
      "secrets": [
        {
          "slug_name": "db",
          "description": "postgres credentials",
          "items": [
            { "key": "DB_HOST", "value": "pg.internal" },
            { "key": "DB_PASSWORD", "value": "s3cr3t" },
            {
              "key": "config.yaml",
              "value": "a: 1\nb: 2",
              "value_format": "yaml"
            },
            {
              "key": "ca.pem",
              "value": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...",
              "encoding": "base64",
              "file_name": "ca.pem",
              "content_type": "application/x-pem-file"
            }
          ]
        }
      ]
    }
  ]
}
```

## Ответ

```json
{
  "apps_created": "1",
  "apps_updated": "0",
  "secrets_created": "1",
  "secrets_updated": "0",
  "items_created": "4",
  "items_updated": "0",
  "unchanged": "0",
  "errors": []
}
```

`errors` (пример): `["item \"payments/payments-api/db/\": key is required"]`.

## Типовой сценарий для агента

1. `POST /usr/login` (учётка администратора) → получить `jwt`.
2. Опционально: `GET /transfer/tree` — посмотреть текущую структуру без
   значений; токен для неё не нужен (см. `transfer-tree-api.md`).
3. Сформировать дерево и отправить в `POST /transfer/import`.
4. Проверить счётчики и `errors` в ответе; при ошибках поправить
   соответствующие записи и повторить (идемпотентно).
5. На `not_authorized` — обновить токен через `/usr/token/refresh` и
   повторить запрос.
