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
  Неаутентифицированный POST логина использует обычный `fetch` — см. `postPublic`
  в [usr.ts](src/api/usr.ts).
- **gRPC-gateway возвращает HTTP 400 на ВСЕ бизнес-ошибки** с телом
  `{ code, message, fields }`. Поэтому успех определяется по `response.ok`, а вид
  ошибки — по `code`, а не по HTTP-статусу.
- `ApiError.code` — машинный код, **дословно совпадает с Go-константами
  `internal/errs`** (`not_authorized`, `no_permission`, `totp_invalid`, …).
  Ветвись по `error.code`; для текста пользователю — `apiErrorMessage(err, fallback)`.
- **`message` из ответа пользователю не показывай** — это не текст для человека:
  у известной ошибки там повторён сам код, у неожиданной —
  `internal error (request id: …)` (код `service_not_available`). Описание
  в `message` приходит только у ошибок, которые бэкенд поднял с описанием
  (например `invalid_request` — «keys not found in the cluster source: A, B»).
  Всё это разбирает `apiErrorMessage` ([errors.ts](src/api/errors.ts)): описание,
  если оно есть → текст по коду из таблицы `ERROR_TEXTS` → для неожиданной ошибки
  запасной текст + request id. **Новый код в `internal/errs` — добавь строку в
  `ERROR_TEXTS`**, иначе пользователь увидит запасной текст с кодом в скобках.
- `too_many_attempts` (вход, начало/подтверждение/отключение 2FA, смена пароля)
  несёт `fields.retry_after` — секунды строкой; `apiErrorMessage` сам строит «Try
  again in N seconds / minutes», отдельно обрабатывать не нужно.
- Бэкенд маршалит с `EmitUnpopulated` → в JSON **всегда присутствуют нулевые
  поля**. Не полагайся на отсутствие поля, проверяй значение.
- `int64`-id сериализуются gateway как **строки**. В типах они `number`, но
  при сравнении приводи к строке (см. `currentUserId` / `String(row.id)`).

## Аутентификация и сессия

- Токены — в `localStorage` (`kusec_admin_token` + `kusec_admin_refresh_token`):
  короткоживущий access-JWT + долгоживущий refresh (ротируется). Это **не**
  httpOnly — осознанный компромисс.
- **Права бэкенд берёт из базы на каждом запросе.** Выключенный пользователь и
  пользователь после выхода (он гасит токены везде) сразу получают
  `not_authorized` — и на обычный запрос, и на обновление токена. Если же база
  недоступна в момент проверки сессии, бэкенд отвечает `service_not_available`
  (а не `not_authorized`) — на любой запрос, включая `POST /usr/token/refresh`.
- Тихое обновление: при auth-ошибке `apiFetch` зовёт `renewTokenOnce`
  (общий in-flight) и повторяет запрос один раз. `renewTokenOnce` не бросает и
  возвращает один из трёх исходов ([auth-session.ts](src/api/auth-session.ts)):
  - `renewed` — новая пара сохранена, запрос повторяется;
  - `rejected` — бэкенд отверг refresh **по существу** (`not_authorized`) либо
    refresh-токена нет: сессия чистится, шлётся `window` event `auth:required`,
    его ловит `App.vue` → `dropSession()` + переход на login;
  - `unavailable` — узнать не удалось (`service_not_available`, ответ прокси не
    в JSON, сетевая ошибка, таймаут 15 с): **сессию не трогаем**, наружу летит
    причина сбоя обновления как обычная ошибка запроса — пользователь остаётся
    в системе и может повторить.
- **Сессию стирает только отвергнутый refresh.** Сбой самого запроса
  (`service_not_available`, сеть, таймаут) и даже отказ повтору с только что
  выданным токеном сессию не трогают — следующий запрос снова спросит refresh.
  На запрос максимум одно обновление и один повтор, повтор больше не обновляет —
  зациклиться негде. Если токен сменился, пока запрос был в пути, он просто
  повторяется с текущим, без лишнего refresh. Не дублируй эту логику в
  компонентах и не зови `dropSession()` на упавший запрос.
- **Два разных выхода** в [stores/auth.ts](src/stores/auth.ts), не путай:
  - `logout()` — выход по желанию пользователя: сначала `POST /usr/logout`
    (гасит все токены пользователя на сервере, то есть на всех устройствах),
    затем локальная очистка. Очистка происходит всегда, ошибка запроса
    игнорируется (нет сети, таймаут 5 с, токен уже мёртв).
  - `dropSession()` — только локальная очистка, без запроса на сервер: для
    сессии, которая на сервере уже недействительна (`auth:required`, смена
    пароля). Звать тут `logout()` нельзя — он пошёл бы на сервер с токеном,
    который сервер только что отверг.
- `useAuthStore().initialize()` вызывается один раз в `router.beforeEach` до
  первой навигации и не бросает. Если профиль при старте не загрузился из-за
  сбоя (сервер или база лежат, нет сети), **токены остаются**: пользователь
  попадает на login с пояснением (`loginNotice = 'session_unavailable'`), а
  перезагрузка страницы восстанавливает сессию и возвращает на адрес из
  `?redirect=`.
- **2FA-флоу логина** (см. [usr.ts](src/api/usr.ts), [stores/auth.ts](src/stores/auth.ts)):
  `login()` возвращает `UsrLoginRep` с двумя исходами — пара токенов /
  `totp_required` (нужен код). 2FA необязательна ни для кого, в том числе для
  админов: включается/выключается самим пользователем в профиле. Секрет TOTP **никогда** не отправляй во внешний QR-сервис —
  QR генерится локально (`qrcode`).
- **Включение 2FA** ([TotpSetupCard.vue](src/components/usr/TotpSetupCard.vue)) — два
  шага. Сначала текущий пароль: `POST /usr/totp/enroll` требует `current_password`,
  QR и ключ показываются только после него; `current_password_invalid` и
  `too_many_attempts` выводятся под полем пароля. Затем код из приложения.
- **`POST /usr/totp/confirm` меняет сессию**: в ответе пара токенов новой версии
  сессии, старые токены мертвы с момента ответа. Поэтому `confirmTotp` идёт через
  `replaceSession()`: новая пара сохраняется в тот же тик, что получен ответ, а
  пока вызов в пути, `apiFetch` придерживает все остальные запросы — со старым
  токеном ничего не уходит. Запрос, который уже был в пути и вернулся с отказом,
  повторяется с новой парой, а не тратит мёртвый refresh (иначе сессия была бы
  стёрта). Любой будущий вызов, в ответ на который бэкенд выдаёт новую пару
  взамен текущей, оформляй так же.
- На `POST /usr/login` код `not_authorized` значит «неверный логин или пароль»
  (или пользователь выключен), а не «сессия кончилась» — `LoginView` показывает
  свой текст (`loginErrorText`). Частые неудачи входа и кодов 2FA →
  `too_many_attempts` (см. выше).
- **Профиль** (`PUT /usr/profile`): смена пароля или логина требует
  `current_password`; неверный/пустой → `current_password_invalid`, показывается
  под полем «Current password». `username` и `password` шлём, только когда они
  реально меняются. **После смены пароля сессия кончается** (бэкенд гасит все
  токены): `updateProfile()` стора сам делает `dropSession()` и возвращает
  `true`, а `ProfileView` уводит на login, где один раз показывается «Password
  changed…» (`takeLoginNotice()`). Перечитывать профиль после смены пароля
  нельзя — запрос с мёртвым токеном поднимет `auth:required`.

## Доступ к приложениям и список пользователей

- У пользователя два поля: `all_apps` и `app_ids`. Не-админ с `all_apps=true`
  видит все приложения; с `all_apps=false` — только из `app_ids`, и **пустой
  список значит «не видит ничего»**, а не «все». Админ видит всё всегда.
- Состояние доступа читай только через `usrAccessKind()` / `usrAccessLabel()`
  ([utils/usrAccess.ts](src/utils/usrAccess.ts)): `all` / `listed` / `none`. Не
  проверяй `app_ids.length` по месту — так и появляется «пустой = все». Для
  текущего пользователя — `useAuthStore().appAccess`.
- В форме пользователя при включённом «All applications» список не отправляется
  (`app_ids: []`); при выключенном пустой выбор допустим, но с предупреждением.
- **`GET /usr` и `GET /usr/{id}` — только для админа** (не-админ получит
  `no_permission`). Зови их только с админских экранов и под условием
  `authStore.isAdmin`; свои данные не-админ берёт из `GET /usr/profile`
  (`authStore.profile`).
- **Сам себя админ на странице Users правит не полностью.** Свой логин, пароль и
  2FA меняются только в My profile (там спрашивается текущий пароль или код):
  `PUT /usr/{id}` со своим `username`/`password` и `POST /usr/{id}/totp/reset`
  для себя бэкенд отвергает (`no_permission`). Поэтому в `UsrFormModal` при
  правке себя (`isSelf`) логин заблокирован, поля пароля нет, оба не уходят в
  запрос; в `UsrListView` «Reset 2FA» в своей строке недоступен. Везде рядом —
  подсказка со ссылкой на профиль. Остальные свои поля правятся как у всех.
- Число пользователей на дашборде (`GET /dashboard`, поле `usr`) не-админу
  приходит нулями — плитку «Users» ему не показываем.

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
