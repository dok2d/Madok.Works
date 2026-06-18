# REST API

Все эндпоинты возвращают JSON. Требуется аутентификация (session cookie), кроме `/api/auth/setup` при первом запуске и страницы логина.

Большинство списочных эндпоинтов привязаны к проекту: передайте `?project=<id>`, чтобы выбрать активный проект. Задачи имеют попроектный `number` и отображаются как `KEY-<number>`.

## Аутентификация

| Метод | Путь                      | Описание                                |
|-------|---------------------------|-----------------------------------------|
| POST  | `/api/auth/setup`         | Создание первого администратора         |
| POST  | `/api/auth/login`         | Вход (опциональный `{code}` для 2FA; возвращает `{totp_required:true}`, когда нужен второй шаг) |
| POST  | `/api/auth/logout`        | Выход                                   |
| GET   | `/api/auth/me`            | Информация о текущем пользователе       |
| POST  | `/api/auth/reset-request` | Запрос сброса пароля (через Telegram)   |
| POST  | `/api/auth/reset-confirm` | Подтверждение сброса с кодом            |
| POST  | `/api/auth/sso/config`    | Настройка OIDC/LDAP (admin)            |
| POST  | `/api/auth/oidc/login`    | Начало OIDC-авторизации                 |
| POST  | `/api/auth/oidc/callback` | Обработка ответа OIDC-провайдера        |

## Пользователи (admin)

| Метод  | Путь             | Описание                    |
|--------|------------------|-----------------------------|
| GET    | `/api/users`     | Список пользователей        |
| POST   | `/api/users`     | Создание пользователя       |
| PUT    | `/api/users/:id` | Изменение роли/пароля       |
| DELETE | `/api/users/:id` | Удаление пользователя       |

## Двухфакторная аутентификация (TOTP)

| Метод | Путь               | Описание                                                |
|-------|--------------------|----------------------------------------------------------|
| GET   | `/api/2fa/status`  | Включена ли 2FA (`{enabled}`)                            |
| POST  | `/api/2fa/setup`   | Начало подключения → `{secret, uri}` (provisioning URI)  |
| POST  | `/api/2fa/enable`  | Подтверждение кода `{code}` → `{recovery_codes}`         |
| POST  | `/api/2fa/disable` | Отключение 2FA `{code}`                                  |

## Проекты

| Метод  | Путь                          | Описание                                          |
|--------|-------------------------------|---------------------------------------------------|
| GET    | `/api/projects`               | Список доступных проектов                         |
| POST   | `/api/projects`               | Создание проекта: `{name, description, visibility}` |
| GET    | `/api/projects/{id}`          | Детали проекта                                    |
| PUT    | `/api/projects/{id}`          | Обновление проекта (админ проекта)                |
| DELETE | `/api/projects/{id}`          | Удаление проекта (админ проекта)                  |
| GET    | `/api/projects/{id}/members`  | Участники проекта                                 |
| POST   | `/api/projects/{id}/members`  | Добавить участника                                |
| DELETE | `/api/projects/{id}/members`  | Удалить участника                                 |

Остальные ресурсы привязаны к активному проекту через `?project=<id>`.

## Доска и задачи

| Метод  | Путь               | Описание                                              |
|--------|---------------------|-------------------------------------------------------|
| GET    | `/api/board`        | Полная доска (колонки, задачи, эпики, спринты, теги) |
| GET    | `/api/tasks`        | Список задач                                          |
| POST   | `/api/tasks`        | Создание задачи                                       |
| GET    | `/api/tasks/:id`    | Детали задачи                                         |
| PUT    | `/api/tasks/:id`    | Обновление задачи                                     |
| DELETE | `/api/tasks/:id`    | Удаление задачи                                       |
| POST   | `/api/tasks/move`   | Перемещение задачи между колонками                    |
| GET    | `/api/tasks/:id/gitevents` | Связанная git-активность по задаче             |

## Колонки

| Метод | Путь                   | Описание              |
|-------|------------------------|-----------------------|
| GET   | `/api/columns`         | Список колонок        |
| POST  | `/api/columns`         | Создание колонки      |
| PUT   | `/api/columns/:id`     | Обновление колонки    |
| DELETE| `/api/columns/:id`     | Удаление колонки      |
| POST  | `/api/columns/reorder` | Переупорядочивание    |

## Спринты

| Метод | Путь                        | Описание                              |
|-------|-----------------------------|---------------------------------------|
| GET   | `/api/sprints`              | Список спринтов                       |
| POST  | `/api/sprints`              | Создание спринта                      |
| GET   | `/api/sprints/:id`          | Спринт с задачами                     |
| PUT   | `/api/sprints/:id`          | Обновление спринта                    |
| DELETE| `/api/sprints/:id`          | Удаление спринта                      |
| POST  | `/api/sprints/:id/complete` | Завершение спринта (перенос задач)    |

## Эпики

| Метод | Путь              | Описание            |
|-------|--------------------|---------------------|
| GET   | `/api/epics`       | Список эпиков       |
| POST  | `/api/epics`       | Создание эпика      |
| GET   | `/api/epics/:id`   | Эпик с задачами     |
| PUT   | `/api/epics/:id`   | Обновление эпика    |
| DELETE| `/api/epics/:id`   | Удаление эпика      |

## Теги

| Метод | Путь             | Описание        |
|-------|-------------------|-----------------|
| GET   | `/api/tags`       | Список тегов    |
| POST  | `/api/tags`       | Создание тега   |
| DELETE| `/api/tags/:id`   | Удаление тега   |

## Комментарии

| Метод | Путь                 | Описание              |
|-------|----------------------|-----------------------|
| POST  | `/api/comments`      | Добавление            |
| PUT   | `/api/comments/:id`  | Редактирование        |
| DELETE| `/api/comments/:id`  | Удаление              |

## Уведомления и подписки

| Метод | Путь                         | Описание                     |
|-------|------------------------------|------------------------------|
| GET   | `/api/notifications`         | Уведомления пользователя     |
| POST  | `/api/notifications/read`    | Отметить как прочитанное     |
| POST  | `/api/notifications/read-all`| Отметить все прочитанными    |
| POST  | `/api/subscribe`             | Подписаться на задачу        |
| POST  | `/api/unsubscribe`           | Отписаться от задачи         |

## Файлы и изображения

| Метод | Путь              | Описание                    |
|-------|--------------------|-----------------------------|
| POST  | `/api/images`      | Загрузка изображения (base64)|
| GET   | `/api/images/:id`  | Получение изображения       |
| POST  | `/api/files`       | Загрузка файла (base64)     |
| GET   | `/api/files/:id`   | Скачивание файла            |

## Telegram

| Метод | Путь                           | Описание                     |
|-------|--------------------------------|------------------------------|
| GET   | `/api/settings/telegram`       | Настройки бота (admin)       |
| POST  | `/api/settings/telegram`       | Установка токена/имени бота  |
| GET   | `/api/settings/telegram/status`| Статус настройки бота        |
| POST  | `/api/user/telegram/link`      | Генерация хеша привязки      |
| POST  | `/api/user/telegram/unlink`    | Отвязка Telegram             |

## Настройки

| Метод | Путь                     | Описание                                       |
|-------|--------------------------|------------------------------------------------|
| GET   | `/api/settings/timezone` | Текущий часовой пояс                           |
| POST  | `/api/settings/timezone` | Установка часового пояса                        |
| GET   | `/api/settings/app`      | Брендинг: `{name, default_lang, wap_enabled}`   |
| POST  | `/api/settings/app`      | Имя приложения, язык по умолчанию, переключатель WAP: `{name, lang, wap}` (admin) |
| GET   | `/api/settings/sso`      | Настройки LDAP/OIDC (admin)                     |
| POST  | `/api/settings/sso`      | Сохранение настроек LDAP/OIDC (admin)          |

## Сервисы экосистемы

| Метод | Путь                     | Описание                                                    |
|-------|--------------------------|-------------------------------------------------------------|
| GET   | `/api/services`          | Список сервисов и их состояние (вкл/выкл)                    |
| POST  | `/api/services`          | Включить/выключить сервис: `{name, enabled}` (admin)        |
| POST  | `/api/services/reorder`  | Порядок сервисов в панели: `{order: [...]}` (admin)         |

## Резервное копирование (admin)

| Метод  | Путь                  | Описание                    |
|--------|------------------------|-----------------------------|
| GET    | `/api/backups`         | Список бэкапов              |
| POST   | `/api/backups`         | Создать бэкап               |
| GET    | `/api/backups/{name}`  | Скачать бэкап               |
| POST   | `/api/backups/{name}`  | Восстановить из бэкапа      |
| DELETE | `/api/backups/{name}`  | Удалить бэкап               |

## Поиск

| Метод | Путь                 | Описание                              |
|-------|----------------------|---------------------------------------|
| GET   | `/api/search?q=...`  | Поиск по задачам (текст / regex)      |
| GET   | `/api/search/global` | Глобальный поиск по всем сервисам. Параметры: `q`, `scope` (`all`/`kanban`/`wiki`/`chat`/`mail`) |

## Прочее

| Метод | Путь                      | Описание                   |
|-------|---------------------------|----------------------------|
| GET   | `/api/export`             | Экспорт доски в JSON       |
| POST  | `/api/import`             | Импорт доски из JSON       |
| GET   | `/api/user/activity/:id`  | Лента активности           |
| POST  | `/api/user/password`      | Смена своего пароля        |

## Wiki

| Метод  | Путь                              | Описание                                  |
|--------|-----------------------------------|-------------------------------------------|
| GET    | `/api/wiki/spaces`                | Список пространств                        |
| POST   | `/api/wiki/spaces`                | Создание пространства                     |
| GET    | `/api/wiki/spaces/:id`            | Пространство с деревом страниц            |
| PUT    | `/api/wiki/spaces/:id`            | Обновление пространства                   |
| DELETE | `/api/wiki/spaces/:id`            | Удаление пространства                     |
| GET    | `/api/wiki/spaces/:id/members`    | Участники пространства (ACL)              |
| POST   | `/api/wiki/spaces/:id/members`    | Добавить участника                        |
| DELETE | `/api/wiki/spaces/:id/members`    | Удалить участника (`?user_id=`)           |
| POST   | `/api/wiki/pages`                 | Создание страницы                        |
| GET    | `/api/wiki/pages/:id`             | Страница                                  |
| PUT    | `/api/wiki/pages/:id`             | Обновление страницы (новая ревизия)       |
| DELETE | `/api/wiki/pages/:id`             | Удаление страницы                         |
| POST   | `/api/wiki/pages/:id/move`        | Перемещение страницы в дереве             |
| GET    | `/api/wiki/pages/:id/revisions`   | История ревизий страницы                  |
| POST   | `/api/wiki/pages/:id/restore`     | Откат к ревизии                           |
| GET    | `/api/wiki/revisions/:id`         | Содержимое конкретной ревизии             |
| GET    | `/api/wiki/search?q=...`          | Поиск по страницам                        |

## Чат

| Метод  | Путь                                    | Описание                                          |
|--------|-----------------------------------------|---------------------------------------------------|
| GET    | `/api/chat/channels`                    | Список каналов/групп/DM пользователя              |
| POST   | `/api/chat/channels`                    | Создание канала/группы: `{name, topic, type, members}` |
| GET    | `/api/chat/channels/:id`                | Информация о канале                               |
| PUT    | `/api/chat/channels/:id`                | Переименование/тема/переключение `{read_only}`    |
| DELETE | `/api/chat/channels/:id`                | Удаление канала                                   |
| POST   | `/api/chat/channels/:id/mute`           | Отключить уведомления: `{until}` (0 = навсегда, иначе unix-секунды) |
| DELETE | `/api/chat/channels/:id/mute`           | Включить уведомления                              |
| GET    | `/api/chat/channels/:id/members`        | Участники канала                                  |
| POST   | `/api/chat/channels/:id/members`        | Добавить участника / задать право записи: `{user_id, can_write}` |
| DELETE | `/api/chat/channels/:id/members`        | Удалить участника (`?user_id=`)                   |
| GET    | `/api/chat/channels/:id/messages`       | Сообщения (`before`, `after`, `limit`)            |
| POST   | `/api/chat/channels/:id/messages`       | Отправка сообщения: `{text, attachments, reply_to}` |
| GET    | `/api/chat/channels/:id/pins`           | Закреплённые сообщения                            |
| POST   | `/api/chat/channels/:id/read`           | Отметить прочитанным: `{last_id}`                 |
| POST   | `/api/chat/channels/:id/typing`         | Индикатор набора текста                           |
| GET    | `/api/chat/messages/:id`                | Сообщение                                         |
| PUT    | `/api/chat/messages/:id`                | Редактирование (только автор)                     |
| DELETE | `/api/chat/messages/:id`                | Удаление сообщения                                |
| POST   | `/api/chat/messages/:id/pin`            | Закрепить/открепить: `{pinned}`                   |
| POST   | `/api/chat/messages/:id/react`          | Реакция-эмодзи (toggle): `{emoji}`                |
| POST   | `/api/chat/dm`                          | Открыть/создать личный диалог: `{user_id}`        |
| GET    | `/api/chat/search?q=...`                | Поиск по сообщениям                               |
| GET    | `/api/chat/unread`                      | Счётчик непрочитанных                             |
| GET    | `/api/chat/events`                      | Поток событий (SSE)                               |

## Звонки (сигнализация WebRTC)

Медиа передаётся peer-to-peer; сервер только ретранслирует сигнализацию.

| Метод  | Путь                 | Описание                                                                    |
|--------|----------------------|-----------------------------------------------------------------------------|
| GET    | `/api/calls/events`  | SSE-поток сигналов звонка                                                    |
| POST   | `/api/calls/signal`  | Ретрансляция сигнала: `{to, kind, call_id, channel_id, members, payload}` (kind: `invite`/`accept`/`decline`/`cancel`/`offer`/`answer`/`ice`/`hangup`) |
| GET    | `/api/calls/config`  | ICE-серверы: `{iceServers:[...]}` (STUN + опционально TURN)                  |
| GET    | `/api/calls/turn`    | Настройки TURN (admin): `{turn_url, turn_user, turn_cred}`                   |
| POST   | `/api/calls/turn`    | Сохранить настройки TURN (admin): `{url, user, cred}`                        |

## Интеграция с Git

| Метод  | Путь                                       | Описание                                                      |
|--------|--------------------------------------------|---------------------------------------------------------------|
| GET    | `/api/integrations/git/repos`              | Список привязанных репозиториев (по проекту через `?project=`) |
| POST   | `/api/integrations/git/repos`              | Привязать репозиторий (admin)                                 |
| PUT    | `/api/integrations/git/repos/{id}`         | Обновить репозиторий (admin)                                  |
| DELETE | `/api/integrations/git/repos/{id}`         | Отвязать репозиторий (admin)                                  |
| POST   | `/api/integrations/git/webhook/{repoID}`   | Публичный входящий вебхук (проверка по подписи/секрету провайдера: GitHub/Gitea HMAC-SHA256, GitLab token) |

## Календарь

| Метод  | Путь                                       | Описание                                       |
|--------|--------------------------------------------|------------------------------------------------|
| GET    | `/api/calendar/calendars`                  | Список календарей                              |
| POST   | `/api/calendar/calendars`                  | Создание календаря                             |
| PUT    | `/api/calendar/calendars/:id`              | Обновление календаря                           |
| DELETE | `/api/calendar/calendars/:id`              | Удаление календаря                             |
| GET    | `/api/calendar/events`                     | События (`from`, `to`)                         |
| POST   | `/api/calendar/events`                     | Создание события                               |
| GET    | `/api/calendar/events/:id`                 | Событие                                        |
| PUT    | `/api/calendar/events/:id`                 | Обновление события                             |
| DELETE | `/api/calendar/events/:id`                 | Удаление события                               |
| GET    | `/api/calendar/upcoming`                   | Ближайшие события                              |
| GET    | `/api/calendar/stream`                     | Поток событий (SSE)                            |
| GET    | `/api/calendar/subscriptions`              | Внешние ICS-подписки (пользовательские)        |
| POST   | `/api/calendar/subscriptions`              | Добавить ICS-подписку: `{name, url, color}`    |
| DELETE | `/api/calendar/subscriptions/:id`          | Удалить подписку                               |
| POST   | `/api/calendar/subscriptions/:id/sync`     | Принудительная синхронизация подписки          |

## Почта

| Метод  | Путь                              | Описание                                  |
|--------|-----------------------------------|-------------------------------------------|
| GET    | `/api/mail/folders`               | Системные папки и счётчики                |
| GET    | `/api/mail/folders/custom`        | Пользовательские папки                    |
| POST   | `/api/mail/folders/custom`        | Создать папку                            |
| PUT    | `/api/mail/folders/custom/:id`    | Переименовать папку                       |
| DELETE | `/api/mail/folders/custom/:id`    | Удалить папку                            |
| GET    | `/api/mail/unread`                | Счётчик непрочитанных                     |
| GET    | `/api/mail/audit`                 | Журнал отправки (admin)                   |
| GET    | `/api/mail/rules`                 | Правила фильтрации                       |
| POST   | `/api/mail/rules`                 | Создать правило                          |
| PUT    | `/api/mail/rules/:id`             | Обновить правило                         |
| DELETE | `/api/mail/rules/:id`             | Удалить правило                          |
| GET    | `/api/mail/settings`              | Настройки почты (admin)                   |
| POST   | `/api/mail/settings`              | Сохранить настройки почты (admin)         |
| GET    | `/api/mail/aliases`               | Псевдонимы (admin)                        |
| POST   | `/api/mail/aliases`               | Создать псевдоним (admin)                 |
| DELETE | `/api/mail/aliases/:id`           | Удалить псевдоним (admin)                 |
| GET    | `/api/mail/messages`              | Список писем папки                        |
| GET    | `/api/mail/messages/:id`          | Письмо (открытие помечает прочитанным)    |
| DELETE | `/api/mail/messages/:id`          | Удалить письмо                           |
| POST   | `/api/mail/messages/:id/flag`     | Флаги письма (прочитано/звезда и т.п.)    |
| POST   | `/api/mail/messages/:id/move`     | Переместить письмо в папку                |
| GET    | `/api/mail/thread/:id`            | Цепочка переписки                        |
| POST   | `/api/mail/send`                  | Отправить письмо                         |
| GET    | `/api/mail/stream`                | Поток событий (SSE)                       |

## WAP

Для совместимости со старыми мобильными устройствами доступен WML-интерфейс:

| Путь              | Описание          |
|-------------------|-------------------|
| `/wap/`           | Обзор доски       |
| `/wap/column/:id` | Просмотр колонки  |
| `/wap/task/:id`   | Детали задачи     |
| `/wap/backlog`    | Бэклог            |
| `/wap/login`      | Вход (WAP)        |
