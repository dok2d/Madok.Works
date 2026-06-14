# План: сервис «Чат» в экосистеме канбана

_Ветка: `feature/ecosystem` · дата: 2026-06-14 (актуализировано)_

> **Память сессии восстановлена** в `~/.claude/.../memory/`:
> `kanban-ecosystem-state` (состояние работ), `kanban-ecosystem-service-pattern`
> (конвенции добавления сервиса), `kanban-ecosystem-workflow` (процесс/запреты).

## 1. Текущее состояние

### Экосистема (закоммичено)
- `9f846ef` — скелет экосистемы: app rail, реестр сервисов, админ-тогглы.
- `fccdc43` + `9ced692` — Wiki (бэкенд + фронтенд) как эталон качества.

Механика сервисов уже работает:
- `internal/handler/services.go` — `serviceOrder`, `serviceAvailable`, `serviceDefaultOn`,
  `serviceEnabled()`, `GET/POST /api/services`. Состояние вкл/выкл хранится в
  `app_settings` ключом `svc_<name>_enabled`.
- `web/templates/index.html` — app rail (`renderAppRail`), бейджи (`setRailBadge`),
  админ-секция (`renderServicesSection`, `toggleService`), безопасные заглушки
  представлений (`renderServiceStub`, `renderChatView` → стаб).

### Чат — backend (готов, **не закоммичен**)
Рабочая копия содержит полностью реализованный и **компилирующийся** (`go build ./...` = 0,
`go vet` чисто) бэкенд чата:

| Файл | Статус | Содержимое |
|------|--------|-----------|
| `internal/model/model.go` | M | `ChatChannel`, `ChatMessage`, `ChatSearchHit` |
| `internal/db/store.go` | M | миграции: `chat_channels`, `chat_members`, `chat_messages` + индексы |
| `internal/db/store_chat.go` | ?? (новый) | полный store: каналы, участники, сообщения, DM, поиск, непрочитанное |
| `internal/handler/chat.go` | ?? (новый) | SSE-хаб, хендлеры каналов/сообщений/DM/поиска/unread/events, @-упоминания |
| `internal/handler/handler.go` | M | роуты `/api/chat/*`, поле `sse *sseHub`, исключение `/read` из read-only |
| `internal/handler/services.go` | M | `chat: true` в `serviceAvailable` |

Покрытая функциональность бэкенда:
- **Каналы**: публичные (`channel`), приватные группы (`group`), личные (`dm`).
  CRUD, список с подсчётом непрочитанного и участников, права (admin/создатель).
- **Участники**: добавление/удаление, самоприсоединение к публичным каналам.
- **Сообщения**: пагинация (before/after), создание, редактирование, soft-delete.
- **DM**: `GetOrCreateDM` — ленивое создание парного канала.
- **Прочитано**: `MarkChatRead` (last_read_id), счётчики unread + `TotalChatUnread` для бейджа.
- **Поиск**: `SearchChatMessages` (LIKE) по видимым каналам.
- **Реалтайм**: SSE-хаб `/api/chat/events` (события `chat_message|chat_edit|chat_delete|chat_channels`),
  heartbeat, адресная рассылка по участникам/broadcast.
- **Уведомления**: `@username` → in-app + Telegram (`notifyChatMentions`).

### Чат — frontend (**не сделано**)
- `web/static/js/chat.js` отсутствует; `renderChatView` — заглушка (`renderServiceStub('chat')`).
- В `index.html` нет `<script src="/static/js/chat.js">`.
- Не подключены: подписка на SSE, обновление бейджа из `/api/chat/unread`.

### Тесты
- Есть только `internal/handler/sanitize_test.go`. Тестов чата нет.

---

## 2. План действий

### Шаг 0. Зафиксировать готовый бэкенд ✅ приоритет
Бэкенд собирается и проходит vet — закоммитить как самостоятельный инкремент
(правило: коммитить каждый успех).
```
git add internal/model/model.go internal/db/store.go internal/db/store_chat.go \
        internal/handler/chat.go internal/handler/handler.go internal/handler/services.go
git commit -m "feat(chat): backend — channels, DMs, messages, SSE, search, mentions"
```

### Шаг 1. Тесты бэкенда чата
Создать `internal/db/store_chat_test.go` (и при необходимости `internal/handler/chat_test.go`):
- создание каналов разных типов, валидация типа/имени;
- `GetOrCreateDM` — идемпотентность, запрет DM самому себе;
- пагинация before/after, сортировка по возрастанию id;
- soft-delete очищает текст;
- `MarkChatRead` / подсчёт unread (автор не считает свои сообщения);
- права: не-участник не видит приватный канал, поиск не выдаёт чужое.
- Коммит.

### Шаг 2. Frontend чата — `web/static/js/chat.js`
По образцу `wiki.js` (state + `renderChatView` + под-рендеры). Реализовать:
- **Layout**: список каналов слева (публичные / группы / DM, бейджи непрочитанного),
  лента сообщений справа, поле ввода.
- **Лента**: загрузка `GET /api/chat/channels/{id}/messages`, пагинация вверх (before),
  автоскролл вниз, группировка по автору/дате.
- **Отправка/редактирование/удаление**: POST/PUT/DELETE, оптимистичный рендер.
- **Прочитано**: `POST /api/chat/channels/{id}/read` при просмотре.
- **Создание каналов/групп**, добавление участников, **DM** через `POST /api/chat/dm`.
- **Поиск**: `GET /api/chat/search?q=`.
- **@-упоминания**: автодополнение по пользователям.
- Экранирование текста (XSS), переиспользовать существующие хелперы (`esc`, и т.п.).
- Коммит.

### Шаг 3. Реалтайм + бейджи в `index.html`
- Подключить `<script src="/static/js/chat.js">` (рядом с `wiki.js`, строка 12).
- Открыть один `EventSource('/api/chat/events')` на сессию; по событиям обновлять
  активную ленту и список каналов.
- При старте и по событиям дёргать `/api/chat/unread` → `setRailBadge('chat', n)`.
- Корректно закрывать/переоткрывать SSE при logout/login.
- Коммит.

### Шаг 4. Проверка end-to-end
- `go build ./...`, `go vet`, `go test ./...`.
- Ручная проверка **через `go run` (НЕ `kanban.sh` — он использует podman, запрещён)**:
  создать канал, написать из двух пользователей, проверить realtime, DM, упоминания,
  бейджи, поиск, тоггл сервиса.
- Коммит при зелёном результате.

### Шаг 5 (опционально). Доработки
- FTS5 для поиска вместо LIKE (если важна скорость на объёме).
- Загрузка файлов/картинок в сообщения (есть готовый `triggerFileAttach`).
- Индикатор «печатает», реакции, треды — по запросу.

---

## 3. Следующие сервисы экосистемы (будущие фазы)
`calendar` и `mail` помечены `serviceAvailable: false`. Делать по той же схеме:
backend → тесты → frontend → realtime/интеграция, каждый в своём наборе коммитов.

## 4. Ограничения процесса и техники
- **Без `git push` и `podman`** — запрещены, в `deny`. Локальный запуск — только `go run`.
- **Только отдельная ветка** (`feature/ecosystem`), не main.
- **Коммит после каждого рабочего инкремента.**
- Файлы читать/править инструментами Read/Edit/Write; нестандартное — через `python3`.
- **БД: `SetMaxOpenConns(1)`** — открытый курсор держит единственное соединение,
  поэтому в store-методах сначала читать все строки, и только потом делать follow-up
  запросы (как в `ListChatChannels`).
- Новый сервис подключается по чек-листу из `kanban-ecosystem-service-pattern`
  (реестр сервисов, исключения read-only, SSE-хаб, i18n `svc_<name>`, app rail).
