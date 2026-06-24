# CLAUDE.md — admin SPA

Руководство по работе с фронтендом админки (`apps/admin`). Стек и запуск — в
[README.md](README.md). Здесь только **тонкости и неочевидные правила**, которые
легко нарушить.

Vue 3 (`<script setup lang="ts">`, Composition API) + Vite + Pinia + Vue Router +
Naive UI. Раздаётся Go-бэкендом с `/`, API под `/api`.

---

## Пакетный менеджер: ТОЛЬКО pnpm

- Зависимости ставить через `pnpm add` / `pnpm add -D`, lockfile — `pnpm-lock.yaml`.
- **npm и yarn ломаются**: в дереве есть `link:`-зависимости (протокол pnpm) →
  `npm error EUNSUPPORTEDPROTOCOL`, плюс peer-конфликты по vite. Не используй их.
- **`pnpm-workspace.yaml` (рядом с `package.json`) содержит `allowBuilds:`** — явный
  allowlist пакетов, которым pnpm (v10+/11) разрешает запускать post-install build-скрипты
  (`esbuild`, `vue-demi`). По умолчанию pnpm **блокирует** их из соображений безопасности.
  Добавил зависимость с нативным/postinstall-билдом и видишь warning «Ignored build
  scripts» — допиши пакет в `allowBuilds` (или `pnpm approve-builds`), иначе он молча не
  соберётся. Этот файл нужен и Docker-сборке (копируется в install-слой).
- **После смены зависимостей коммить актуальный `pnpm-lock.yaml`.** Docker-сборка ставит
  пакеты с `--frozen-lockfile` — рассинхрон lockfile с `package.json` уронит сборку.

## Naive UI

- Компоненты импортируются по месту (`import { NButton } from 'naive-ui'`) —
  глобальной регистрации нет.
- Провайдеры (`NMessageProvider`, `NDialogProvider`, `NNotificationProvider`,
  `NLoadingBarProvider`, `NConfigProvider`) живут в `App.vue`. Хуки
  `useMessage()` / `useDialog()` работают только внутри этого дерева (т.е. в
  компонентах под `RouterView`), но **не** в модулях `api/`, `stores/`, `router/`.
- Тема следует за ОС (`useOsTheme` + `darkTheme`). Не хардкодь цвета —
  используй переменные темы / `NText :depth`.
- Иконки — `@vicons/tabler`, через `<NIcon :component="X" />`. Перед
  использованием убедись, что иконка существует:
  `ls node_modules/@vicons/tabler/<Name>.js`.
- В `NDataTable` колонки рендерятся через `h()` (render-функции), не слоты.

## API-слой (`src/api/`)

- Все аутентифицированные запросы идут через `apiFetch` ([http.ts](src/api/http.ts)).
  Несколько **неаутентифицированных** POST (login, bootstrap, TOTP
  enroll/confirm по setup-токену) используют обычный `fetch` — см. `postPublic`
  в [usr.ts](src/api/usr.ts).
- **gRPC-gateway возвращает HTTP 400 на ВСЕ бизнес-ошибки** с телом
  `{ code, message, fields }`. Поэтому успех определяется по `response.ok`, а вид
  ошибки — по `code`, а не по HTTP-статусу.
- `ApiError.code` — машинный код, **дословно совпадает с Go-константами
  `internal/errs`** (`not_authorized`, `no_permission`, `totp_invalid`, …).
  Ветвись по `error.code`; для текста пользователю — `apiErrorMessage(err, fallback)`.
- Бэкенд маршалит с `EmitUnpopulated` → в JSON **всегда присутствуют нулевые
  поля**. Не полагайся на отсутствие поля, проверяй значение.
- `int64`-id сериализуются gateway как **строки**. В типах они `number`, но
  при сравнении приводи к строке (см. `currentUserId` / `String(row.id)`).

## Аутентификация и сессия

- Токены — в `localStorage` (`kusec_admin_token` + `kusec_admin_refresh_token`):
  короткоживущий access-JWT + долгоживущий refresh (ротируется). Это **не**
  httpOnly — осознанный компромисс.
- Тихое обновление: при auth-ошибке `apiFetch` зовёт `renewTokenOnce`
  (общий in-flight) и повторяет запрос один раз. Если refresh отвергнут —
  чистит сессию и шлёт `window` event `auth:required`, который ловит `App.vue`
  → logout + переход на login. Не дублируй эту логику в компонентах.
- `useAuthStore().initialize()` вызывается один раз в `router.beforeEach` до
  первой навигации.
- **2FA-флоу логина** (см. [usr.ts](src/api/usr.ts), [stores/auth.ts](src/stores/auth.ts)):
  `login()` возвращает `UsrLoginRep` с тремя исходами — пара токенов /
  `totp_required` (нужен код) / `totp_setup_required` + `setup_token`
  (обязательная привязка для админа). Setup-токен годится только для
  TOTP-эндпоинтов. Секрет TOTP **никогда** не отправляй во внешний QR-сервис —
  QR генерится локально (`qrcode`).

## Контракт API (зеркалит правила бэкенда)

- **Пагинация zero-based**: в API `page` с 0. Naive UI пагинация 1-based →
  всегда слать `page - 1` (см. `fetchUsers`). Сборка query — `buildListQuery`.
- **Update — это `PUT`** (не PATCH).
- REST-пути в единственном числе (`/usr`, `/secret`, `/item`).

## Роутинг

- `meta`: `public`, `requiresAuth`, `requiresAdmin` (см. [router/index.ts](src/router/index.ts)).
  Гард в `beforeEach`: public-страницы редиректят залогиненных на home;
  `requiresAdmin` — не-админов на home.

## Адаптив / мобильные

- `useBreakpoint(maxWidth=768)` → `isMobile`. Плотные `NDataTable`-вью на
  мобильном рендерят **стопку карточек** вместо таблицы (см. `UsrListView`),
  сайдбар становится overlay-drawer. Для новых таблиц делай так же.
- Полноэкранные layout'ы — `min-height: 100dvh` (с `100vh` фоллбэком), иначе
  мобильный браузер даёт лишний скролл «через шапку».
- Числовые поля (коды, OTP):
  `:input-props="{ inputmode: 'numeric', autocomplete: 'one-time-code' }"`.

## Формы

- Единая reactive-`model`; `NForm :model :rules` валидирует `model[path]`, где
  `path` — **ключ верхнего уровня** модели (не вложенный ref).
- `await formRef.value?.validate()` бросает при невалидности → оборачивай в
  try/catch и `return`.

## State и переиспользование

- Pinia — setup-сторы (композиционный стиль). При деструктуризации сохраняй
  реактивность через `storeToRefs`.
- Повторяющиеся паттерны вынесены в `composables/` (`useEntityForm`,
  `useDrawerResource`, `useClipboard`, `useBreakpoint`, `use*Options`). Сначала
  ищи готовый composable, потом пиши новый.

## Сборка и деплой

- `vite.config.ts`: `inlineDynamicImports: true` → **один JS-бандл** (без
  code-split), `chunkSizeWarningLimit` поднят. Предупреждение о крупном бандле
  ожидаемо.
- В прод API относительный (`/api`, `VITE_API_BASE_URL` пуст) → один origin с
  бэком, поэтому CORS на бэке по умолчанию выключен.
- **`.env.*` работают из коробки** (Vite грузит по режиму: `pnpm dev` →
  `.env.development`, `pnpm build` → `.env.production`). `.env.development` /
  `.env.production` / `.env.example` закоммичены (`.env.production` принудительно,
  `!.env.production` в `.gitignore`). В бандл попадает и **виден в браузере только**
  `VITE_`-префикс — **секретов в `.env.*` не клади**. Локальный оверрайд — `.env.local`
  (gitignored), не коммить.
- В docker-образ `dist/` вшивается как `admin-dist/` (`make docker-build` в
  корне репо) и раздаётся Go-бэкендом. После изменений фронта для образа нужна
  пересборка. Docker собирает фронт сам (этап `admin-builder`); локальный `dist/`
  в образ не попадает (исключён в `.dockerignore`).

## Линт / формат

- `pnpm lint` (eslint --fix), `pnpm build` (vue-tsc type-check + сборка).
- ⚠️ `pnpm format` (`prettier --write src/`) проходит по **всему** `src/` и
  переформатирует чужие файлы. Не коммить несвязанные reformat-правки —
  форматируй только свои файлы или откатывай остальные (`git checkout`).
