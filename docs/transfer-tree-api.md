# Kusec API: GET /transfer/tree

Выгрузка всех записей Kusec (Secret Management System) деревом
**без значений секретов** — безопасно для анализа внешним
инструментом/ИИ-агентом. Документ самодостаточен.

## Базовый URL

Пути указаны относительно базового URL HTTP API:

| Окружение | База |
|---|---|
| Через админку / ingress | `http://<host>/api` |
| Прямо в HTTP-gateway сервиса | `http://<host>:8080/api` |

## Аутентификация

**Не требуется.** Ручка сознательно открыта: значения item-ов в ответе
замаскированы, а выдавать агенту токен нельзя — с ним через остальные
ручки API стали бы доступны сами значения секретов. Это единственная
ручка API, доступная без токена (кроме логина); на остальные запросы без
`Authorization` сервис отвечает кодом `not_authorized`.

## Общие соглашения

- **int64-поля сериализуются строками** (`"value_size": "11"`).
- Временные метки — RFC 3339 (`"2026-06-11T10:11:34.952073Z"`).
- Ошибки приходят с HTTP 400 и телом
  `{ "code": "...", "message": "...", "fields": {} }`.

## Доменная модель

Дерево из трёх уровней:

```
app (приложение)        — namespace, name, slug_name, description, active
└─ secret (секрет)      — slug_name, description, active
   └─ item (ключ)       — key, value_format, encoding, file_name,
                          content_type, description, active
```

- Натуральные ключи: app — пара `(namespace, slug_name)` (уникальна);
  secret — `slug_name` внутри app; item — `key` внутри secret.
- `value_format`: `text` | `yaml` | `json` — подсказка редактора.
- `encoding`: `plain` (значение хранится как есть) | `base64` (значение —
  base64 бинарного файла; тогда обычно заполнены `file_name`/`content_type`).
- При синхронизации в Kubernetes секрет получает имя
  `{app.slug_name}-{secret.slug_name}` в namespace `app.namespace`,
  ключи — из item-ов; участвуют только записи с `active=true`.

## Запрос

```sh
curl -s "$BASE/transfer/tree"
```

Возвращаются **все** записи, включая неактивные (`active=false`).
Сортировка детерминированная: apps по `(namespace, slug_name)`,
secrets по `slug_name`, items по `key`.

## Маскирование значений

**Значения item-ов не возвращаются.** Вместо `value` отдаётся
`value_size` — размер хранимой строки в байтах. Для `encoding=base64`
это размер base64-представления, а не декодированного файла.
`value_size: "0"` означает пустое значение.

## Ответ (пример)

```json
{
  "apps": [
    {
      "id": "b5d70167-...",
      "namespace": "payments",
      "name": "Payments API",
      "slug_name": "payments-api",
      "description": "",
      "active": true,
      "updated_at": "2026-06-11T10:11:34.952073Z",
      "secrets": [
        {
          "id": "a0f9af5c-...",
          "slug_name": "db",
          "description": "",
          "active": true,
          "updated_at": "2026-06-11T10:11:34.935198Z",
          "items": [
            {
              "id": "d5b09ca9-...",
              "key": "DB_HOST",
              "value_format": "text",
              "encoding": "plain",
              "file_name": "",
              "content_type": "",
              "description": "",
              "active": true,
              "updated_at": "2026-06-11T10:11:34.937768Z",
              "value_size": "11"
            }
          ]
        }
      ]
    }
  ]
}
```

## Типовой сценарий для агента

1. `GET /transfer/tree` — получить и проанализировать структуру
   (значения скрыты, токен не нужен).
2. Пустой ответ `{"apps":[]}` означает, что записей в системе нет.

Для массового изменения данных существует парная ручка
`POST /transfer/import` — она требует токен администратора
(см. `transfer-import-api.md`).
