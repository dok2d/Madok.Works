# REST API

Все эндпоинты возвращают JSON. Требуется аутентификация (session cookie), кроме `/api/auth/setup` при первом запуске и страницы логина.

## Аутентификация

| Метод | Путь                      | Описание                                |
|-------|---------------------------|-----------------------------------------|
| POST  | `/api/auth/setup`         | Создание первого администратора         |
| POST  | `/api/auth/login`         | Вход                                    |
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
| GET   | `/api/settings/app`      | Имя приложения / брендинг (`{name}`)            |
| POST  | `/api/settings/app`      | Установка имени приложения (admin)              |
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
| PUT    | `/api/chat/channels/:id`                | Переименование/тема канала                        |
| DELETE | `/api/chat/channels/:id`                | Удаление канала                                   |
| GET    | `/api/chat/channels/:id/members`        | Участники канала                                  |
| POST   | `/api/chat/channels/:id/members`        | Добавить участника: `{user_id}`                   |
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
