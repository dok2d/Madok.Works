# User Guide

The kanban board is the main service, but other ecosystem services (wiki, chat,
calendar, mail) sit alongside it. They all open from the **service rail on the
left** (app rail). For full details see [Ecosystem Services](ecosystem.md); the
sections below give brief usage scenarios.

## Projects

Everything you work with — the board, wiki spaces and calendars — lives inside a
**project**. The **active project** is chosen from the **selector in the header**;
switching it switches the board, wiki and calendars you see. There is always a
default project (named "General") to start from.

### Creating a Project

If you may create projects, the selector has a **"New project"** entry. You give
it a **name**, a short **key** (uppercase, e.g. `GEN`), and a **visibility**:

- **Open (everyone)** — every user can see the project.
- **Restricted (members)** — only people on the project's member list can see it.

### Roles and Task Keys

Within a project each member has a **role**:

- **Admin** — manages the project, its members and settings.
- **Editor** — creates and edits tasks, pages and events.
- **Viewer** — read-only access.

Tasks are numbered per project and shown as **KEY-123** (the project key plus a
sequential number), so the same number can appear in different projects without
clashing.

### Moderating Projects

Administrators manage all projects under **Settings → Board → Projects**: create
projects, change visibility, and add or remove members and their roles.

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

When an administrator has connected an LLM, the task form also offers
**✨ Generate** — it drafts a **description** and a **TODO** list for you, which
you can then replace or append.

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

Search is **multi-term**: when you type several words, every word must appear
somewhere in a result for it to match (so extra words narrow the results rather
than widening them).

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
The tabs are **grouped** — top-level groups (**Mail**, **Board**, **Site**) open a
secondary bar of sub-tabs — and which tabs you see depends on your role.

### My Settings

Your personal display preferences live under the **"My settings"** tab:

- **Theme**, **interface language**, **font size** and **time zone**.

These are per-user — they affect only your own view.

### Account

The **Account** tab holds everything tied to your own login:

- **Change password**.
- **Telegram** linking for notifications (see the [Telegram bot](telegram.md)).
- **Two-factor authentication (2FA)** — see below.

#### Two-Factor Authentication (2FA)

2FA adds a one-time code from an authenticator app as a second login step. To
turn it on, open **Account → Enable 2FA**, add the shown secret to your
authenticator app (or scan the `otpauth` link), and enter the current 6-digit
code to confirm. You are then given a set of **one-time recovery codes** — save
them somewhere safe in case you lose the app. From your **next login** onward,
you'll be asked for the current code after your password. You can turn 2FA off
again from the same tab.

### Mail Settings

- **Mail** — rules for sorting incoming mail into folders (see [Mail](#mail)).

### Administrator Settings

Administrators get additional tabs (grouped under **Board** and **Site**):

- **Board → Projects** — create and moderate [projects](#projects) and their
  members.
- **Site** — **"Application name"** (the browser tab title, the name in the header
  and on the login page, and backup file names) and the **default language** for
  new users.
- **Services** — enable or disable services (including the kanban board itself)
  and change their order in the service rail.

For more on managing services see [Ecosystem Services](ecosystem.md).

## Language

Each user picks **their own interface language** under
[**My settings**](#my-settings); it affects only your view.

The **default language for new users** is set the first time the instance is
created: the registration screen has a **flag picker** that is auto-detected from
your browser (falling back to Russian if none of the offered languages match).
An administrator can change this default later in **Settings → Site**.

## Wiki

Confluence-style documentation. The space is selected **in the header** (a
dropdown); the page tree is in the sidebar.

Pages are edited in a **Markdown editor** with live preview and a formatting
toolbar: headings, bold / italic / strikethrough, inline code and code blocks,
lists, quote, link, image insertion, table (size picked with the mouse), and
callouts. There is no separate visual (WYSIWYG) mode.

Each page has a **revision history** — you can view and restore earlier versions.
Pages also support **comments**, **reactions**, reusable **templates** and
embedded **diagrams**. Wiki search goes through the shared
[global search](#global-search).

### Anchor Links

Every heading on a page gets a stable **anchor link**. Hover over a heading and
click the **#** that appears to copy a direct link to that section; opening a URL
like `/wiki/page#section` jumps straight to that heading. Drop a **`[[toc]]`**
token into a page to insert an automatic table of contents built from its
headings.

See [Ecosystem Services](ecosystem.md) for more.

## Chat

Telegram-style messaging: **public channels**, **private groups**, and **direct
messages** (in the DM section every user is available as a contact).

- Messages appear as **bubbles** with **avatars**.
- **Replies** (quoting) that jump to the original message — **double-click a
  message** to reply quickly.
- **Emoji reactions**.
- **Pinned messages** — shown in a bar at the top.
- A message actions menu under the **"⋯"** button (reply, react, pin, edit,
  delete).
- A "**typing…**" indicator and **read receipts** in direct messages.
- **Attachments** (files and images) and **system messages** (chat created,
  joined/left, renamed).
- The chat list is sorted by the most recent message.

### Public Channels vs Private Groups

When you create a chat you choose its type:

- **New channel** — a public channel that all users can find and read.
- **New group** — a private group; you add members with a **member picker**, and
  only members can see it.

### Read-Only Channels

A channel can be made **read-only**, so most members can only read it. Only the
designated **writers**, the channel **creator** and **administrators** can post;
everyone else sees a "read-only" notice instead of the message box.

### Mute and Notification Sound

In the chat header you can **mute** a single channel for **1 hour**, **8 hours**,
or **forever** (and unmute again), and toggle a **global notification sound** for
new messages.

Chat search goes through the shared [global search](#global-search). See
[Ecosystem Services](ecosystem.md) for more.

## Calls

Voice and video calls run directly between participants (WebRTC). Start one from
the **📞 (voice)** or **📹 (video)** buttons in the **chat header** of any
channel, group or direct message.

- The other side sees a **ring** and can **Accept** or **Decline**.
- In-call controls let you **mute the microphone**, toggle the **camera**, and
  **hang up**.
- You can **add any user** to a call already in progress.

> Calls work out of the box on the same network. For reliable calls **across
> different networks** (NAT/firewalls), an administrator needs to configure a
> **TURN server** in the settings.

## Calendar

The view (month / week / day / agenda) and the list of calendars are selected
**in the header**. The calendar supports **recurring events**, **customizable
reminders**, and a read-only overlay of **task and sprint deadlines** from the
board.

**Navigating periods:**

- The month and week views show the **ISO week number** in a column on the left.
- The **mouse wheel** pages through periods by one step matching the current view
  (month / week / day). Agenda view does not page on scroll.
- Clicking the **year** in the header opens a mini calendar: pick a year, then a
  month — a quick jump to any date.

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
