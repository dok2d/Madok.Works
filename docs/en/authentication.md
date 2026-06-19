# Authentication & Roles

## Local Authentication

On first launch, the application prompts you to create an administrator. After that, login is done via the login form.

### Sessions

- Cookie-based sessions with a 90-day lifetime
- Session token — a random 64-character string
- HttpOnly cookies for XSS protection
- Automatic cleanup of expired sessions (hourly)

### Password Hashing

Passwords are hashed using PBKDF2-HMAC-SHA256:
- 100,000 iterations
- 32-byte random salt

## Roles

| Role        | Capabilities |
|-------------|-------------|
| **Admin**   | Full access: user management, columns, epics, tags, Telegram settings, export/import |
| **User**    | Create, edit, delete tasks; comments; subscriptions; change own password |
| **Read-only** | View board and tasks; receive notifications |

### User Management (Admin)

Administrators can:

- Create new users with a selected role
- Change an existing user's role
- Reset a user's password
- Delete users

Restrictions: administrators cannot change their own role and cannot delete themselves.

## Two-Factor Authentication (2FA / TOTP)

2FA is opt-in per user and managed under **Settings → Account**.

### Enrollment

1. Open Settings → Account and start 2FA enrollment
2. The server returns an `otpauth://` URL (and the raw secret) — add it to an authenticator app (Google Authenticator, Aegis, FreeOTP, etc.) by scanning the QR code or entering the secret manually
3. Confirm enrollment by entering a current 6-digit code from the app
4. The server issues one-time **recovery codes** — store them safely; each code works once if you lose access to the authenticator

### Login with 2FA

When 2FA is enabled, login becomes a two-step flow:

1. The user submits username and password as usual
2. The login response returns `{ "totp_required": true }` instead of completing the session
3. The client resends the request with the code: `{ "code": "123456" }` (a valid TOTP code or a recovery code)

## CLI password recovery (closed networks)

On an air-gapped/closed network the Telegram password reset is unreachable. This
also matters right after **restoring a backup**, since exports omit password
hashes — every restored user (including admins) starts without a usable password.

Reset (or create) an admin straight from the CLI, against the same database the
app uses:

```bash
# via the kanban.sh wrapper (runs inside the running app container)
./kanban.sh passwd <username> <new-password>

# or the binary directly (no server started); resolves the DB like the app does
kanban -reset-user <username> -reset-password <new-password> -db <same -db> -data-dir <same -data-dir>
```

If the user exists, its password is updated; if not, an **admin** with that name
is created. Passwords must be at least 6 characters. Use this to regain access
after a restore, then sign in and reset other users from the UI.

## Multi-Project Access Model

Access to content is organized around **Projects**. A Project is a container with its own **members** and **roles**:

| Project role | Capabilities within the project |
|--------------|---------------------------------|
| **Admin**    | Manage the project, its members and settings |
| **Editor**   | Create and edit content |
| **Viewer**   | Read-only access |

Each project has a **visibility** setting:

- **Open** — implicitly available to all users; users who are not explicit members get read-only (viewer) access
- **Restricted** — only listed members can access the project

**Global admins** have access to every project regardless of membership or visibility.

This access model gates the project-scoped features: the **kanban board**, **wiki spaces**, and **project calendars**.

## Password Recovery

Available for users with a linked Telegram account:

1. On the login page, click "Forgot password?"
2. Enter your username
3. The bot will send an 8-digit confirmation code
4. Enter the code and set a new password

## LDAP / Active Directory

The application supports LDAP/AD authentication for enterprise environments.

Configuration via API (admin):

```
POST /api/auth/sso/config
```

Configuration parameters:
- LDAP server URL
- Base DN for user search
- Bind DN and password (for search)
- User search filter
- Attribute mapping

With LDAP authentication, users enter their corporate credentials, and the application verifies them via the LDAP server.

## OpenID Connect (OIDC)

SSO support via OIDC providers (Keycloak, Google, Azure AD, etc.).

Configuration via API (admin):

```
POST /api/auth/sso/config
```

Parameters:
- Client ID and Client Secret
- Provider URL (issuer)
- Redirect URI

Once configured, an SSO login button appears on the login page. On first OIDC login, a local account is automatically created.

Endpoints:
- `POST /api/auth/oidc/login` — initiate OIDC authorization
- `POST /api/auth/oidc/callback` — handle provider response
