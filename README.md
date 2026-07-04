# Mailfold

[![CI](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml/badge.svg)](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml)

A modern web frontend (admin panel + webmail) for [mailcow-dockerized](https://github.com/mailcow/mailcow-dockerized).

**Website:** [mailfold.site](https://mailfold.site)

**Mailfold** — the name plays on two meanings: a *fold* is both the crease of a letter (mail) and an enclosure for livestock, nodding to mailcow's cattle branding and the "herd" of domains and mailboxes the panel manages.

## Stack

- **Backend:** Go — a thin, typed API layer on top of the mailcow REST API.
- **Frontend:** React (Vite) — an admin panel and a built-in webmail client,
  served by the backend as a single-page app.

All code and documentation are written in English.

## Features

**Admin panel** — run the whole mailcow server from one calm UI: a live
dashboard (container health, storage, mail queue), mailboxes (quotas, a usage
bar, last login, per-mailbox app passwords, Sieve filters, rate limits and
temporary aliases in a slide-over drawer, plus bulk creation from a CSV file),
domains (a detail page per domain
with DKIM key management, rate limits, and a live DNS check that verifies MX,
the mail host's A record, SPF, DKIM and DMARC against the live zone), aliases
(with a mailbox-picker for recipients and a catch-all-for-domain toggle),
the mail queue (flush for redelivery, or discard everything outright),
quarantine (release or delete held messages), the spam policy (per-domain
allow / block lists, plus custom Rspamd rule blocks for finer-grained scoring
and whitelisting), IMAP/POP sync jobs (with a move-instead-of-copy option),
per-service logs (an "All services" merged view and a live-tail toggle),
a fail2ban panel and settings (theme, accent, language). An
"Advanced" section exposes the niche routing/config resources (relay hosts,
transports, TLS policies, forwarding hosts, BCC/recipient maps, templates), and
an "Administration" section manages delegated admins and OAuth2 clients.

**Webmail** — a built-in three-pane webmail client (folders · message list ·
reader) with message previews, star, flag, archive, delete, attachment download
and live new-mail notifications (over SSE). Compose is a rich-text editor
(bold/italic/lists/links, sending HTML), sent mail is saved to Sent, and folders
and labels can be created inline. A user can hold **several mailboxes in one
place** — switch between linked Mailfold accounts, or connect an **external
IMAP mailbox** (Gmail, Yandex, …) that syncs into the inbox — and a Mail /
Calendar toggle opens a full **Month / Week / Day calendar** backed by the
self-hosted CalDAV store, with a sidebar (mini-month, calendar legend and an
upcoming list) and drag-to-reschedule. Recurring events are expanded across the
view, multi-day events span their days, and a detected meeting link surfaces a
Join-call button. Its event editor covers all-day and timed events, guests,
recurrence, reminders and **file attachments** (added or removed on edit, stored
inline in the VEVENT); events are colour-coded by calendar (work / personal /
team / holidays), and clicking one opens a detail card with an Outlook-style
**RSVP** row (Going / Maybe / Can't, persisted as the owner's ATTENDEE
`PARTSTAT`). So end users read, send and schedule without a separate client.

**One unified login** — a single sign-in screen checks the credentials against
*both* the admin login and a mailbox (webmail) login in parallel: whoever you
are, it takes you to the right place, and if you have access to both it simply
asks which one to open. A mailbox-only user gets a clean standalone webmail; an
admin gets the full panel.

**Account security** — the admin can change their own password, edit their
profile, and enroll optional TOTP two-factor auth (with a QR code and one-time
recovery codes) right from Settings; login gains a second-factor step once it's
on. Active sessions are listed by device/IP and can be revoked individually or
all at once. A "forgot password?" link on the sign-in screen emails a reset link
— from a mailbox the admin configures themselves in Settings (see
[`MAILFOLD_ADMIN_ENC_KEY`](.env.example)), not a fixed environment variable.
Optionally, sign-in can also delegate to an OIDC identity provider ("Continue
with SSO"): the provider only ever authenticates a single, explicitly
configured email address into the one Mailfold admin account (see the five
`MAILFOLD_OIDC_*` variables in [`.env.example`](.env.example)) — it is fully
inert until every one of them is set.

**API keys — drive your mailbox from other apps.** Issue durable, individually
revocable API keys so any third-party service can **send and collect mail** for a
mailbox over a simple REST API — without ever handling the mailbox password. It
is far simpler to wire Mailfold into external applications, scripts and
automations this way than to speak raw IMAP/SMTP yourself. See
[API keys](#api-keys-send--collect-mail-from-other-services).

**Groupware** — self-hosted CardDAV and CalDAV (contacts, calendars, tasks), so
clients sync without SOGo.

**Built for internationalisation** — every user-facing string goes through a
translation layer; the UI ships in English today, and adding a language is a
single drop-in locale file.

## Architecture

```
React SPA  ──▶  Mailfold Go backend  ──▶  mailcow API (/api/v1/...)
```

The Go backend authenticates to mailcow with an API key, exposes a clean REST
surface to the frontend, and serves the built SPA.

## Editions (open-core)

Mailfold is open-core. The community edition in this repository is complete and
self-contained; it stores its groupware and API-key data in **SQLite** and needs
no external database. The persistence layer is a small driver registry
([`backend/storage`](backend/storage)), so the same store code runs on more than
one database with no duplication.

The **enterprise edition** adds a **PostgreSQL** backend for larger, multi-node
deployments. It is a separate, private Go module that imports this core and
registers a `postgres` driver — no forked backend, just one extra driver and
entry point. PostgreSQL is strictly enterprise-only: the community binary has no
PostgreSQL dependency and, asked for `MAILFOLD_DB_DRIVER=postgres`, fails fast
with *"database driver \"postgres\" is not available in this build"*.

## Backend

### Configuration

The backend is configured entirely through environment variables (see
[`.env.example`](.env.example)):

| Variable | Default | Description |
| --- | --- | --- |
| `MAILFOLD_ADDR` | `:8080` | Listen address. |
| `MAILFOLD_MAILCOW_URL` | — (required) | Base URL of the mailcow instance. |
| `MAILFOLD_MAILCOW_API_KEY` | — | mailcow API key (`X-API-Key`). |
| `MAILFOLD_MAILCOW_INSECURE_TLS` | `false` | Skip TLS verification (dev only). |
| `MAILFOLD_FRONTEND_DIR` | `./frontend/dist` | Built SPA directory (optional). |

### Run

```bash
cp .env.example .env   # then edit values
make run               # or: cd backend && go run ./cmd/mailfold
```

### API

The backend exposes a broad REST surface over the mailcow API — domains,
mailboxes, aliases, the mail queue, quarantine, spam policy, sync jobs, DKIM,
per-service logs, container/version status, self-hosted CardDAV/CalDAV, the
webmail endpoints, and the machine-to-machine API keys. Rather than duplicate
that list here (and let it drift), the running server documents itself:

### API documentation

The running server documents itself:

- `GET /api/docs` — interactive Swagger UI.
- `GET /api/openapi.yaml` — the raw OpenAPI 3 spec ([backend/internal/api/openapi.yaml](backend/internal/api/openapi.yaml)).

### Docker

A multi-stage [`Dockerfile`](Dockerfile) builds a static binary into a
distroless, non-root image:

```bash
docker build -t mailfold-backend .
docker run --rm -p 8080:8080 \
  -e MAILFOLD_MAILCOW_URL=https://mail.cortexus.ru \
  -e MAILFOLD_MAILCOW_API_KEY=... \
  -e MAILFOLD_ADMIN_PASSWORD=... \
  mailfold-backend
```

### Docker Compose

For local development or simple self-hosting, [`docker-compose.yml`](docker-compose.yml)
runs the backend and connects it to a mailcow instance (by default a mailcow on
the host at `:8443`):

```bash
cp .env.example .env   # set the admin password, mailcow API key, etc.
docker compose up --build
```

## Continuous integration

Every push and pull request runs [`.github/workflows/ci.yml`](.github/workflows/ci.yml):

1. **Build & test** — `go build`, `go vet`, and `go test` with coverage, failing
   the run if total coverage drops below **80%**.
2. **SonarQube quality gate** — analysis is uploaded to SonarQube and the build
   fails unless the project's quality gate is green.

CI requires two repository settings: a `SONAR_TOKEN` secret and a
`SONAR_HOST_URL` variable.

## Frontend

The UI is a Vite + React (JSX) single-page app in [`frontend/`](frontend/):
`main.jsx` → `App.jsx` (auth gate) → the app shell and ~15 pages under
`src/pages/`, talking to the backend through `src/api/client.js`. The design
system is vendored under `src/ds/`, and all user-facing strings go through the
i18n layer in `src/i18n/`. It builds to `frontend/dist/`, which the Go backend
serves (`MAILFOLD_FRONTEND_DIR`). See [`frontend/README.md`](frontend/README.md).

## Target deployment

- Host: `mail.cortexus.ru` (mailcow on UpCloud, `/opt/mailcow-dockerized`).
- Serviced mail domains: `cortexus.ru`, `agentum.digital`, `proteus-vpn.cloud`.

## Groupware (CardDAV / CalDAV)

Mailfold ships self-hosted CardDAV and CalDAV servers so contacts, calendars,
and tasks can be stored and synced without SOGo. They are backed by a local
SQLite database (`MAILFOLD_DB_PATH`) and are disabled when that path is empty.

- CardDAV endpoint: `/dav/carddav/` (discovery at `/.well-known/carddav`).
- CalDAV endpoint: `/dav/caldav/` (discovery at `/.well-known/caldav`),
  supporting events (`VEVENT`) and tasks (`VTODO`).
- Authentication: HTTP Basic with the user's own mailbox credentials (verified
  against IMAP, then cached briefly).

Point a standard client (Thunderbird, iOS/macOS Contacts & Calendar, DAVx⁵) at
the endpoints to sync.

## API keys (send & collect mail from other services)

Mailfold can issue durable, individually-revocable API keys so a third-party
service can **send** and **collect** mail for a single mailbox over a simple REST
API — without ever handling the mailbox password. A key is a thin bearer handle
in front of a mailcow application password scoped to **IMAP + SMTP only**; the
app-password is encrypted at rest (AES-256-GCM) and the plaintext token is shown
exactly once, at creation.

Enable it with `MAILFOLD_APIKEY_ENABLED=true` and a `MAILFOLD_APIKEY_MASTER_KEY`
(≥32 bytes, hex or base64). It reuses `MAILFOLD_DB_PATH` for storage and stays
off until both are set.

- Admin (normal session) mints/lists/revokes keys: `POST/GET /api/apikeys`,
  `DELETE /api/apikeys/{id}`. Revoking a key also deletes its upstream
  app-password.
- The key itself authenticates the machine surface with
  `Authorization: Bearer mf_live_…`:
  - **Send** (`mail:send`): `POST /api/v1/mail/send` — `From` is forced to the
    bound mailbox (no spoofing), with recipient and body-size caps.
  - **Collect** (`mail:read`): `GET /api/v1/mail/folders|messages|message|search|attachment`.
  - **Mutate** (`mail:write`): `POST /api/v1/mail/flag`, `DELETE /api/v1/mail/message`.

Requests are rate-limited per source IP (before authentication) and per key, and
all token verification is constant-time. See `/api/docs` for the full schema.

## License

Mailfold is licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE).

You are free to **use, modify, and share** Mailfold for **any noncommercial
purpose** — personal projects, research, education, nonprofits, and evaluation.
Contributions that keep the project growing are welcome. **Commercial use is not
permitted** under this license.

Commercial licenses — including the right to use Mailfold in a paid product or
service, or to redistribute it commercially — are available separately from the
copyright holder. See [mailfold.site](https://mailfold.site).

Copyright © 2026 Sviatoslav Ivanov (Team26). All rights not expressly granted by
the license are reserved.
