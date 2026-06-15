# User Guide

The kanban board is the main service, but other ecosystem services (wiki, chat,
calendar, mail) sit alongside it. They all open from the **service rail on the
left** (app rail). For full details see [Ecosystem Services](ecosystem.md); the
sections below give brief usage scenarios.

## Board

The main screen is a kanban board with columns. Each column represents a work stage (e.g., "To Do", "In Progress", "Done"). Tasks are displayed as cards that can be dragged between columns.

### Column Management

- Administrators can create, rename, reorder, and delete columns
- The first (Backlog) and last (Done) columns cannot be deleted
- Column order is configured via drag and drop

### Filtering

Filters are available in the board header:

- **⬡ kanban** — show all tasks (no filtering)
- **Sprint** — filter by sprint (defaults to the oldest active sprint)
- **Epic** — filter by epic

## Tasks

### Creating a Task

Click "+" in the desired column. Specify:

- **Title** — brief task description
- **Description** — details in Markdown format (syntax highlighting supported)
- **Priority** — none, low, medium, high, critical
- **Deadline** — optional date, displayed on the card
- **Sprint** — assign to a sprint (or backlog)
- **Epic** — assign to an epic
- **Tags** — arbitrary labels
- **Assignee** — assigned user

### Priorities

Priority is displayed as a colored bar on the left side of the card:

| Priority | Color  |
|----------|--------|
| Low      | blue   |
| Medium   | yellow |
| High     | orange |
| Critical | red    |

### Markdown in Descriptions

Task descriptions support Markdown:

- Headings, lists, links, images
- Code blocks with syntax highlighting (highlight.js)
- **TODO lists** — interactive checkboxes (`- [ ]` / `- [x]`), can be toggled directly on the card

### Dependencies

Tasks can have dependencies:

- **Depends on** — the task cannot be completed until another is closed
- **Blocks** — this task blocks another from being completed

### Attachments

- **Images** — paste via Ctrl+V (automatically compressed to ~500 KB)
- **Files** — PDF, text files, archives, and other formats

## Sprints

Sprints help organize work into iterations.

### Sprint Statuses

1. **Planning** — sprint created, tasks being added
2. **Active** — work in progress
3. **Completed** — sprint closed

### Completing a Sprint

When a sprint is completed, unclosed tasks are automatically carried over to the next active sprint.

### Backlog

Tasks without a sprint go to the backlog — a general list of tasks awaiting planning.

## Epics

Epics are large initiatives that group multiple tasks. Each epic has:

- A title and description
- Color coding (displayed on cards)
- A progress indicator (proportion of completed tasks)

## Tags

Tags are arbitrary labels for categorizing tasks. A task can have multiple tags. Tags are searchable.

## Comments

- Threaded comments with replies
- Markdown and syntax highlighting support
- **@mentions** — mentioned users receive a notification

## Search

Search is available in the board header. Two modes are supported:

- **Plain text** — substring search
- **Regex** — regular expression search

Search works across: task titles, descriptions, comments, tags, epics.

### Global Search

Besides the board search there is a **single global search** across all services.
Open it with **Ctrl/Cmd+K** or from the search box in the header. The box lets you
pick a scope: **all / kanban / wiki / chat / calendar / mail**; it defaults to the
current service. Selecting a result jumps to that service.

## Notifications

Notifications are sent when:

- A task is assigned to you
- You are @mentioned in a comment
- A task you're subscribed to is updated
- A new comment is added to a subscribed task

You can subscribe to a task via the subscribe button on the card. Notifications are available in the app and via the [Telegram bot](telegram.md).

## Activity Feed

Each user has an activity feed with a detailed history of changes: task creation, status changes, comments, and other actions. The most recent 100 entries are displayed.

## Settings

All settings live on a **single page**. Open it with the **gear at the bottom of
the service rail on the left**. (The old gear in the top-right corner is gone.)
Which tabs you see depends on your role.

### Personal Settings

- **General** — theme, interface language, font size, time zone.
- **Mail** — rules for sorting incoming mail into folders (see [Mail](#mail)).
- **Integrations** — link your **Telegram** account for notifications (see the
  [Telegram bot](telegram.md)).

### Administrator Settings

Administrators get additional tabs:

- **Site** — **"Application name"**. Changes the browser tab title, the name shown
  in the header and on the login page, and the names of backup files.
- **Services** — enable or disable services (including the kanban board itself)
  and change their order in the service rail.

For more on managing services see [Ecosystem Services](ecosystem.md).

## Wiki

Confluence-style documentation. The space is selected **in the header** (a
dropdown); the page tree is in the sidebar.

Pages are edited in a **Markdown editor** with live preview and a formatting
toolbar: headings, bold / italic / strikethrough, inline code and code blocks,
lists, quote, link, image insertion, table (size picked with the mouse), and
callouts. There is no separate visual (WYSIWYG) mode.

Each page has a **revision history** — you can view and restore earlier versions.
Wiki search goes through the shared [global search](#global-search).

See [Ecosystem Services](ecosystem.md) for more.

## Chat

Telegram-style messaging: **channels**, **groups**, and **direct messages** (in
the DM section every user is available as a contact).

- Messages appear as **bubbles** with **avatars**.
- **Replies** (quoting) that jump to the original message.
- **Emoji reactions**.
- **Pinned messages** — shown in a bar at the top.
- A message actions menu under the **"⋯"** button (reply, react, pin, edit,
  delete).
- A "**typing…**" indicator and **read receipts** in direct messages.
- **Attachments** (files and images) and **system messages** (chat created,
  joined/left, renamed).
- The chat list is sorted by the most recent message.

Chat search goes through the shared [global search](#global-search). See
[Ecosystem Services](ecosystem.md) for more.

## Calendar

The view (month / week / day / agenda) and the list of calendars are selected
**in the header**. The calendar supports **recurring events**, **customizable
reminders**, and a read-only overlay of **task and sprint deadlines** from the
board.

### Connecting a Google Calendar (ICS)

You can subscribe to an external, read-only calendar:

1. In Google Calendar open **Settings for that calendar → "Secret address in iCal
   format"** and copy the address.
2. In the app, open the **"Calendars"** dropdown, choose **"+ Google Calendar
   (ICS)"**, and paste that secret address.

Events appear as a **read-only** overlay and sync automatically. The subscription
is **private** — only you can see it.

See [Ecosystem Services](ecosystem.md) for more.

## Mail

Folders and the **"Compose"** button are **in the header**. The interface is
**two-pane**: the message list on the left, the selected message on the right; you
can drag the divider between them.

- **Sorting rules** for incoming mail are configured in
  [Settings](#personal-settings) on the Mail tab.
- There is a **"Spam"** folder (antispam heuristics).
- Mail search goes through the shared [global search](#global-search).

See [Ecosystem Services](ecosystem.md) for more.
