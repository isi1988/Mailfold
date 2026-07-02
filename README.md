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
dashboard (container health, storage, mail queue), mailboxes (create / edit /
delete with quotas and passwords via a slide-over drawer), domains, aliases, the
mail queue, quarantine, the spam policy (per-domain allow / block lists),
IMAP/POP sync jobs, per-service logs and settings (theme, accent, language).

**Webmail** — a built-in three-pane webmail client (folders · message list ·
reader) with compose, reply, star, archive, delete and attachment download, so
end users read and send mail without a separate client.

**One unified login** — a single sign-in screen checks the credentials against
*both* the admin login and a mailbox (webmail) login in parallel: whoever you
are, it takes you to the right place, and if you have access to both it simply
asks which one to open. A mailbox-only user gets a clean standalone webmail; an
admin gets the full panel.

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

### Current API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness probe. |
| `GET` | `/api/domains` | List mail domains. |
| `GET` | `/api/mailboxes` | List mailboxes. |

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

The UI is being built from a dedicated design project and will live in
[`frontend/`](frontend/). See [`frontend/README.md`](frontend/README.md).

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
