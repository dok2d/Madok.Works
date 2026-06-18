# REST API

All endpoints return JSON. Authentication (session cookie) is required, except for `/api/auth/setup` on first launch and the login page.

Most list endpoints are project-scoped: pass `?project=<id>` to select the active project. Tasks carry a per-project `number` and are shown as `KEY-<number>`.

## Authentication

| Method | Path                      | Description                             |
|--------|---------------------------|-----------------------------------------|
| POST   | `/api/auth/setup`         | Create the first administrator          |
| POST   | `/api/auth/login`         | Log in (optional `{code}` for 2FA; returns `{totp_required:true}` when a second step is needed) |
| POST   | `/api/auth/logout`        | Log out                                 |
| GET    | `/api/auth/me`            | Current user info                       |
| POST   | `/api/auth/reset-request` | Password reset request (via Telegram)   |
| POST   | `/api/auth/reset-confirm` | Confirm reset with code                 |
| POST   | `/api/auth/sso/config`    | Configure OIDC/LDAP (admin)             |
| POST   | `/api/auth/oidc/login`    | Initiate OIDC authorization             |
| POST   | `/api/auth/oidc/callback` | Handle OIDC provider response           |

## Users (admin)

| Method | Path             | Description                 |
|--------|------------------|-----------------------------|
| GET    | `/api/users`     | List users                  |
| POST   | `/api/users`     | Create user                 |
| PUT    | `/api/users/:id` | Update role/password        |
| DELETE | `/api/users/:id` | Delete user                 |

## Two-Factor Authentication (TOTP)

| Method | Path               | Description                                              |
|--------|--------------------|----------------------------------------------------------|
| GET    | `/api/2fa/status`  | Whether 2FA is enabled (`{enabled}`)                     |
| POST   | `/api/2fa/setup`   | Begin enrollment → `{secret, uri}` (provisioning URI)    |
| POST   | `/api/2fa/enable`  | Confirm code `{code}` → `{recovery_codes}`               |
| POST   | `/api/2fa/disable` | Disable 2FA `{code}`                                     |

## Projects

| Method | Path                          | Description                                       |
|--------|-------------------------------|---------------------------------------------------|
| GET    | `/api/projects`               | List accessible projects                          |
| POST   | `/api/projects`               | Create project: `{name, description, visibility}` |
| GET    | `/api/projects/{id}`          | Project details                                   |
| PUT    | `/api/projects/{id}`          | Update project (project admin)                    |
| DELETE | `/api/projects/{id}`          | Delete project (project admin)                    |
| GET    | `/api/projects/{id}/members`  | Project members                                   |
| POST   | `/api/projects/{id}/members`  | Add member                                        |
| DELETE | `/api/projects/{id}/members`  | Remove member                                     |

Other resources are scoped to the active project via `?project=<id>`.

## Board & Tasks

| Method | Path               | Description                                           |
|--------|--------------------|-------------------------------------------------------|
| GET    | `/api/board`       | Full board (columns, tasks, epics, sprints, tags)     |
| GET    | `/api/tasks`       | List tasks                                            |
| POST   | `/api/tasks`       | Create task                                           |
| GET    | `/api/tasks/:id`   | Task details                                          |
| PUT    | `/api/tasks/:id`   | Update task                                           |
| DELETE | `/api/tasks/:id`   | Delete task                                           |
| POST   | `/api/tasks/move`  | Move task between columns                             |
| GET    | `/api/tasks/:id/gitevents` | Linked git activity for a task                |

## Columns

| Method | Path                   | Description           |
|--------|------------------------|-----------------------|
| GET    | `/api/columns`         | List columns          |
| POST   | `/api/columns`         | Create column         |
| PUT    | `/api/columns/:id`     | Update column         |
| DELETE | `/api/columns/:id`     | Delete column         |
| POST   | `/api/columns/reorder` | Reorder columns       |

## Sprints

| Method | Path                        | Description                           |
|--------|-----------------------------|---------------------------------------|
| GET    | `/api/sprints`              | List sprints                          |
| POST   | `/api/sprints`              | Create sprint                         |
| GET    | `/api/sprints/:id`          | Sprint with tasks                     |
| PUT    | `/api/sprints/:id`          | Update sprint                         |
| DELETE | `/api/sprints/:id`          | Delete sprint                         |
| POST   | `/api/sprints/:id/complete` | Complete sprint (carry over tasks)    |

## Epics

| Method | Path              | Description         |
|--------|-------------------|---------------------|
| GET    | `/api/epics`      | List epics          |
| POST   | `/api/epics`      | Create epic         |
| GET    | `/api/epics/:id`  | Epic with tasks     |
| PUT    | `/api/epics/:id`  | Update epic         |
| DELETE | `/api/epics/:id`  | Delete epic         |

## Tags

| Method | Path            | Description     |
|--------|-----------------|-----------------|
| GET    | `/api/tags`     | List tags       |
| POST   | `/api/tags`     | Create tag      |
| DELETE | `/api/tags/:id` | Delete tag      |

## Comments

| Method | Path                 | Description         |
|--------|----------------------|---------------------|
| POST   | `/api/comments`      | Add comment         |
| PUT    | `/api/comments/:id`  | Edit comment        |
| DELETE | `/api/comments/:id`  | Delete comment      |

## Notifications & Subscriptions

| Method | Path                          | Description                  |
|--------|-------------------------------|------------------------------|
| GET    | `/api/notifications`          | User notifications           |
| POST   | `/api/notifications/read`     | Mark as read                 |
| POST   | `/api/notifications/read-all` | Mark all as read             |
| POST   | `/api/subscribe`              | Subscribe to a task          |
| POST   | `/api/unsubscribe`            | Unsubscribe from a task      |

## Files & Images

| Method | Path              | Description                     |
|--------|-------------------|---------------------------------|
| POST   | `/api/images`     | Upload image (base64)           |
| GET    | `/api/images/:id` | Get image                       |
| POST   | `/api/files`      | Upload file (base64)            |
| GET    | `/api/files/:id`  | Download file                   |

## Telegram

| Method | Path                            | Description                  |
|--------|---------------------------------|------------------------------|
| GET    | `/api/settings/telegram`        | Bot settings (admin)         |
| POST   | `/api/settings/telegram`        | Set bot token/username       |
| GET    | `/api/settings/telegram/status` | Bot setup status             |
| POST   | `/api/user/telegram/link`       | Generate linking hash        |
| POST   | `/api/user/telegram/unlink`     | Unlink Telegram              |

## Settings

| Method | Path                     | Description                                  |
|--------|--------------------------|----------------------------------------------|
| GET    | `/api/settings/timezone` | Current timezone                             |
| POST   | `/api/settings/timezone` | Set timezone                                 |
| GET    | `/api/settings/app`      | Branding: `{name, default_lang, wap_enabled}` |
| POST   | `/api/settings/app`      | Set app name, default language, WAP toggle: `{name, lang, wap}` (admin) |
| GET    | `/api/settings/sso`      | LDAP/OIDC settings (admin)                   |
| POST   | `/api/settings/sso`      | Save LDAP/OIDC settings (admin)              |

## Ecosystem Services

| Method | Path                     | Description                                              |
|--------|--------------------------|---------------------------------------------------------|
| GET    | `/api/services`          | List services and their state (enabled/disabled)        |
| POST   | `/api/services`          | Toggle a service: `{name, enabled}` (admin)             |
| POST   | `/api/services/reorder`  | Service order in the app rail: `{order: [...]}` (admin) |

## Backups (admin)

| Method | Path                   | Description              |
|--------|------------------------|--------------------------|
| GET    | `/api/backups`         | List backups             |
| POST   | `/api/backups`         | Create backup            |
| GET    | `/api/backups/{name}`  | Download backup          |
| POST   | `/api/backups/{name}`  | Restore from backup      |
| DELETE | `/api/backups/{name}`  | Delete backup            |

## Search

| Method | Path                 | Description                           |
|--------|----------------------|---------------------------------------|
| GET    | `/api/search?q=...`  | Search tasks (text / regex)           |
| GET    | `/api/search/global` | Cross-service search. Params: `q`, `scope` (`all`/`kanban`/`wiki`/`chat`/`mail`) |

## Other

| Method | Path                      | Description              |
|--------|---------------------------|--------------------------|
| GET    | `/api/export`             | Export board as JSON      |
| POST   | `/api/import`             | Import board from JSON    |
| GET    | `/api/user/activity/:id`  | Activity feed             |
| POST   | `/api/user/password`      | Change own password       |

## Wiki

| Method | Path                              | Description                               |
|--------|-----------------------------------|-------------------------------------------|
| GET    | `/api/wiki/spaces`                | List spaces                               |
| POST   | `/api/wiki/spaces`                | Create space                              |
| GET    | `/api/wiki/spaces/:id`            | Space with page tree                      |
| PUT    | `/api/wiki/spaces/:id`            | Update space                              |
| DELETE | `/api/wiki/spaces/:id`            | Delete space                             |
| GET    | `/api/wiki/spaces/:id/members`    | Space members (ACL)                       |
| POST   | `/api/wiki/spaces/:id/members`    | Add member                               |
| DELETE | `/api/wiki/spaces/:id/members`    | Remove member (`?user_id=`)              |
| POST   | `/api/wiki/pages`                 | Create page                              |
| GET    | `/api/wiki/pages/:id`             | Page                                     |
| PUT    | `/api/wiki/pages/:id`             | Update page (new revision)               |
| DELETE | `/api/wiki/pages/:id`             | Delete page                             |
| POST   | `/api/wiki/pages/:id/move`        | Move page in the tree                    |
| GET    | `/api/wiki/pages/:id/revisions`   | Page revision history                    |
| POST   | `/api/wiki/pages/:id/restore`     | Restore a revision                       |
| GET    | `/api/wiki/revisions/:id`         | Content of a specific revision           |
| GET    | `/api/wiki/search?q=...`          | Search pages                             |

## Chat

| Method | Path                                    | Description                                       |
|--------|-----------------------------------------|---------------------------------------------------|
| GET    | `/api/chat/channels`                    | List user's channels/groups/DMs                   |
| POST   | `/api/chat/channels`                    | Create channel/group: `{name, topic, type, members}` |
| GET    | `/api/chat/channels/:id`                | Channel info                                      |
| PUT    | `/api/chat/channels/:id`                | Rename / set topic / toggle `{read_only}`         |
| DELETE | `/api/chat/channels/:id`                | Delete channel                                    |
| POST   | `/api/chat/channels/:id/mute`           | Mute channel: `{until}` (0 = forever, else unix seconds) |
| DELETE | `/api/chat/channels/:id/mute`           | Unmute channel                                    |
| GET    | `/api/chat/channels/:id/members`        | Channel members                                   |
| POST   | `/api/chat/channels/:id/members`        | Add member / set write permission: `{user_id, can_write}` |
| DELETE | `/api/chat/channels/:id/members`        | Remove member (`?user_id=`)                       |
| GET    | `/api/chat/channels/:id/messages`       | Messages (`before`, `after`, `limit`)             |
| POST   | `/api/chat/channels/:id/messages`       | Post message: `{text, attachments, reply_to}`     |
| GET    | `/api/chat/channels/:id/pins`           | Pinned messages                                   |
| POST   | `/api/chat/channels/:id/read`           | Mark read: `{last_id}`                            |
| POST   | `/api/chat/channels/:id/typing`         | Typing indicator                                  |
| GET    | `/api/chat/messages/:id`                | Message                                           |
| PUT    | `/api/chat/messages/:id`                | Edit (author only)                                |
| DELETE | `/api/chat/messages/:id`                | Delete message                                    |
| POST   | `/api/chat/messages/:id/pin`            | Pin/unpin: `{pinned}`                             |
| POST   | `/api/chat/messages/:id/react`          | Toggle emoji reaction: `{emoji}`                  |
| POST   | `/api/chat/dm`                          | Open/create a direct message: `{user_id}`         |
| GET    | `/api/chat/search?q=...`                | Search messages                                   |
| GET    | `/api/chat/unread`                      | Unread counter                                    |
| GET    | `/api/chat/events`                      | Event stream (SSE)                                |

## Calls (WebRTC signaling)

Media is peer-to-peer; the server only relays signaling.

| Method | Path                 | Description                                                                 |
|--------|----------------------|-----------------------------------------------------------------------------|
| GET    | `/api/calls/events`  | SSE stream of call signals                                                  |
| POST   | `/api/calls/signal`  | Relay a signal: `{to, kind, call_id, channel_id, members, payload}` (kinds: `invite`/`accept`/`decline`/`cancel`/`offer`/`answer`/`ice`/`hangup`) |
| GET    | `/api/calls/config`  | ICE servers: `{iceServers:[...]}` (STUN + optional TURN)                    |
| GET    | `/api/calls/turn`    | TURN config (admin): `{turn_url, turn_user, turn_cred}`                      |
| POST   | `/api/calls/turn`    | Save TURN config (admin): `{url, user, cred}`                               |

## Git Integration

| Method | Path                                       | Description                                                   |
|--------|--------------------------------------------|---------------------------------------------------------------|
| GET    | `/api/integrations/git/repos`              | List linked repositories (project-scoped via `?project=`)     |
| POST   | `/api/integrations/git/repos`              | Link a repository (admin)                                     |
| PUT    | `/api/integrations/git/repos/{id}`         | Update repository (admin)                                     |
| DELETE | `/api/integrations/git/repos/{id}`         | Unlink repository (admin)                                     |
| POST   | `/api/integrations/git/webhook/{repoID}`   | Public inbound webhook (verified by per-provider signature/secret: GitHub/Gitea HMAC-SHA256, GitLab token) |

## Calendar

| Method | Path                                       | Description                                    |
|--------|--------------------------------------------|------------------------------------------------|
| GET    | `/api/calendar/calendars`                  | List calendars                                 |
| POST   | `/api/calendar/calendars`                  | Create calendar                                |
| PUT    | `/api/calendar/calendars/:id`              | Update calendar                                |
| DELETE | `/api/calendar/calendars/:id`              | Delete calendar                                |
| GET    | `/api/calendar/events`                     | Events (`from`, `to`)                          |
| POST   | `/api/calendar/events`                     | Create event                                   |
| GET    | `/api/calendar/events/:id`                 | Event                                          |
| PUT    | `/api/calendar/events/:id`                 | Update event                                   |
| DELETE | `/api/calendar/events/:id`                 | Delete event                                   |
| GET    | `/api/calendar/upcoming`                   | Upcoming events                                |
| GET    | `/api/calendar/stream`                     | Event stream (SSE)                             |
| GET    | `/api/calendar/subscriptions`              | External ICS subscriptions (per-user)          |
| POST   | `/api/calendar/subscriptions`              | Add ICS subscription: `{name, url, color}`     |
| DELETE | `/api/calendar/subscriptions/:id`          | Remove subscription                            |
| POST   | `/api/calendar/subscriptions/:id/sync`     | Force-sync a subscription                       |

## Mail

| Method | Path                              | Description                               |
|--------|-----------------------------------|-------------------------------------------|
| GET    | `/api/mail/folders`               | System folders and counters               |
| GET    | `/api/mail/folders/custom`        | Custom folders                            |
| POST   | `/api/mail/folders/custom`        | Create folder                            |
| PUT    | `/api/mail/folders/custom/:id`    | Rename folder                            |
| DELETE | `/api/mail/folders/custom/:id`    | Delete folder                           |
| GET    | `/api/mail/unread`                | Unread counter                           |
| GET    | `/api/mail/audit`                 | Send audit log (admin)                    |
| GET    | `/api/mail/rules`                 | Filter rules                            |
| POST   | `/api/mail/rules`                 | Create rule                            |
| PUT    | `/api/mail/rules/:id`             | Update rule                            |
| DELETE | `/api/mail/rules/:id`             | Delete rule                            |
| GET    | `/api/mail/settings`              | Mail settings (admin)                     |
| POST   | `/api/mail/settings`              | Save mail settings (admin)                |
| GET    | `/api/mail/aliases`               | Aliases (admin)                          |
| POST   | `/api/mail/aliases`               | Create alias (admin)                      |
| DELETE | `/api/mail/aliases/:id`           | Delete alias (admin)                      |
| GET    | `/api/mail/messages`              | List messages in a folder                 |
| GET    | `/api/mail/messages/:id`          | Message (opening marks it read)           |
| DELETE | `/api/mail/messages/:id`          | Delete message                          |
| POST   | `/api/mail/messages/:id/flag`     | Message flags (read/star, etc.)           |
| POST   | `/api/mail/messages/:id/move`     | Move message to a folder                  |
| GET    | `/api/mail/thread/:id`            | Conversation thread                      |
| POST   | `/api/mail/send`                  | Send message                            |
| GET    | `/api/mail/stream`                | Event stream (SSE)                       |

## WAP

A WML interface is available for compatibility with legacy mobile devices:

| Path              | Description        |
|-------------------|--------------------|
| `/wap/`           | Board overview     |
| `/wap/column/:id` | View column        |
| `/wap/task/:id`   | Task details       |
| `/wap/backlog`    | Backlog            |
| `/wap/login`      | Login (WAP)        |
