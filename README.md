# Mailfold

[![CI](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml/badge.svg)](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/isi1988/Mailfold/backend)](https://goreportcard.com/report/github.com/isi1988/Mailfold/backend)
[![Docker Hub](https://img.shields.io/docker/v/sivanov2018/mailfold?sort=semver&label=docker%20hub&logo=docker&logoColor=white)](https://hub.docker.com/r/sivanov2018/mailfold)
[![GitHub release](https://img.shields.io/github/v/release/isi1988/Mailfold?include_prereleases)](https://github.com/isi1988/Mailfold/releases)
[![License: PolyForm Noncommercial 1.0.0](https://img.shields.io/badge/license-PolyForm%20Noncommercial%201.0.0-blue)](LICENSE)

A modern web frontend (admin panel + webmail) for [mailcow-dockerized](https://github.com/mailcow/mailcow-dockerized).

**Website:** [mailfold.site](https://mailfold.site)

**Mailfold** — the name plays on two meanings: a *fold* is both the crease of a letter (mail) and an enclosure for livestock, nodding to mailcow's cattle branding and the "herd" of domains and mailboxes the panel manages.

## Stack

- **Backend:** Go — a thin, typed API layer on top of the mailcow REST API.
- **Frontend:** React (Vite) — an admin panel and a built-in webmail client,
  served by the backend as a single-page app.

All code and documentation are written in English.

## Contents

- [Authentication & access](#authentication--access)
- [Mail](#mail)
- [Calendar & contacts](#calendar--contacts)
- [Admin panel](#admin-panel)
- [API & SDKs](#api--sdks)
- [Internationalization](#internationalization)
- [Architecture](#architecture)
- [Editions (open-core)](#editions-open-core)
- [Backend](#backend)
- [Frontend](#frontend)
- [Continuous integration](#continuous-integration)
- [License](#license)

## Authentication & access

Mailfold recognizes three separate kinds of sign-in, all from the same login
screen: a single set of credentials is checked against every kind in
parallel, whichever matches decides where you land, and if more than one
matches you're simply asked which to open.

- **Admin** — the super-admin account (`MAILFOLD_ADMIN_USER`/`_PASSWORD`)
  gets the full panel.
- **Webmail** — any mailcow mailbox address + its password signs straight
  into a clean, standalone webmail. A user can hold **several mailboxes at
  once** and switch between them.
- **Domain admin** — mailcow's own scoped administrators (limited to a
  subset of domains) get a real login into Mailfold itself, not just
  mailcow's SOGo/admin UI. Setting or changing a domain admin's password
  from the "New domain admin" drawer also registers it for this separate
  sign-in. Once in, a domain admin manages single sign-on for the domain(s)
  mailcow currently reports them scoped to.

On top of password sign-in:

- **Single sign-on (OIDC)** — one or more identity providers can be
  configured from the UI (not a fixed environment variable), each scoped
  either to every domain or to a specific set. A super-admin's shared
  providers are available everywhere; a domain admin can add their own,
  scoped to just the domain(s) they manage. A successful sign-in
  authenticates into the one mailbox whose address matches the provider's
  verified identity — never the admin panel.
- **Device sign-in with an API key** — paste a personal
  [API key](#api--sdks) instead of a password to sign a new device into
  webmail. Useful for a mail client, script, or phone that only holds a key.
  Any active key works regardless of its declared scopes, since the
  app-password behind every key is always fully IMAP+SMTP capable.
- **Two-factor authentication (TOTP)** — the admin and any webmail mailbox
  user can each independently enroll their own optional TOTP 2FA (QR code +
  one-time recovery codes) from their own settings; sign-in gains a
  second-factor step once it's on.
- **Passkeys & security keys (WebAuthn)** — the admin can additionally enroll
  a passkey (Face ID, Touch ID, Windows Hello) or a hardware security key from
  Settings, alongside TOTP rather than replacing it. Once enrolled, sign-in
  offers either factor: enter a TOTP code, or confirm with the passkey — no
  authenticator app needed for the latter. Enrolling requires re-confirming
  the current password first, exactly like TOTP. This needs a database and a
  non-wildcard `MAILFOLD_CORS_ORIGINS` (the browser's WebAuthn API is
  same-origin-only, so a fixed origin is required either way) — no separate
  environment variable to turn it on.
- **Account security & recovery** — the admin can change their password,
  edit their profile, and review/revoke active sessions (by device/IP,
  individually or all at once) from Settings. A "forgot password?" link
  emails a reset link from a mailbox the admin configures themselves (see
  [`MAILFOLD_ADMIN_ENC_KEY`](.env.example)), not a fixed environment
  variable.
- **Audit log** — every admin and domain-admin sign-in (successful or
  failed), sign-out, and mutating action (create/update/delete) is recorded
  with who, when, from which IP, and the outcome — a single place to review
  "who did what" in the admin panel. It's scoped to administration, not
  regular webmail activity (sending or reading mail isn't logged here).
  Available under Administration → Audit log.
- **Security alert emails** — the admin's notify mailbox (the same one used
  for password-reset links) sends a heads-up email on two events: three
  failed admin sign-ins in a row, and a successful admin sign-in from a
  device (IP + browser) not seen before. Both are best-effort — a missing
  notify mailbox just means no email, never a blocked sign-in — and a known
  device is remembered indefinitely so it won't re-alert on every visit.

## Mail

A built-in three-pane webmail client (folders · message list · reader):

- Message previews, star, flag, archive, delete, attachment download, and
  live new-mail notifications over SSE.
- **Compose** is a rich-text editor (bold/italic/lists/links, sending HTML);
  sent mail is saved to Sent, and folders and labels can be created inline.
- **Multi-account** — hold several linked Mailfold mailboxes in one place
  and switch between them, or connect an **external IMAP mailbox** (Gmail,
  Yandex, …) that syncs into the same inbox.
- **Rules** — simple "if sender/recipient/subject contains X, move to
  folder Y" automation, backed by mailcow's Sieve filters, configured from
  webmail settings.
- **Signatures** — a personal signature added automatically to new
  messages, replies, and forwards.
- **Collapsed quote history** — a long back-and-forth thread doesn't pile
  up as an ever-growing wall of quotes: the reader collapses everything past
  the 3 most recent quoted messages behind a "show N earlier" toggle, and
  reply/forward generate a proper quoted header instead of a bare `>`
  prefix.
- Opening webmail from inside the admin panel hides the admin sidebar for a
  distraction-free reading view, with a button back to the panel; an admin
  opening a mailbox they hadn't opened before is asked whether to keep it
  linked to their account or just view it once.
- **Installable PWA + push notifications** — webmail has a manifest and
  service worker, so it can be installed as an app from the browser. From
  Settings > Notifications a mailbox can turn on Web Push per device: a
  background poller checks for new mail every minute and delivers a system
  notification even with no Mailfold tab open anywhere, using a VAPID key
  pair Mailfold generates and stores itself. Requires `MAILFOLD_DB_PATH` and
  `MAILFOLD_ADMIN_ENC_KEY` (the subscription's IMAP password is kept
  encrypted so the poller can check mail on its own); see
  `MAILFOLD_VAPID_CONTACT_EMAIL` in [`.env.example`](.env.example).

## Calendar & contacts

Mailfold ships self-hosted CardDAV and CalDAV servers so contacts,
calendars, and tasks can be stored and synced without SOGo, backed by a
local SQLite database (`MAILFOLD_DB_PATH`).

- CardDAV endpoint: `/dav/carddav/` (discovery at `/.well-known/carddav`).
- CalDAV endpoint: `/dav/caldav/` (discovery at `/.well-known/caldav`),
  supporting events (`VEVENT`) and tasks (`VTODO`).
- Authentication: HTTP Basic with the user's own mailbox credentials
  (verified against IMAP, then cached briefly).

Point a standard client (Thunderbird, iOS/macOS Contacts & Calendar, DAVx⁵)
at the endpoints to sync, or use the **built-in calendar** in webmail: a
Mail / Calendar toggle opens a full Month / Week / Day view with a sidebar
(mini-month, calendar legend, upcoming list) and drag-to-reschedule.
Recurring events expand across the view, multi-day events span their days,
and a detected meeting link surfaces a Join-call button. The event editor
covers all-day and timed events, guests, recurrence, reminders and file
attachments; events are colour-coded by calendar (work / personal / team /
holidays), and clicking one opens a detail card with an Outlook-style
**RSVP** row (Going / Maybe / Can't).

## Admin panel

Run the whole mailcow server from one calm UI:

- **Dashboard** — container health, storage, mail queue at a glance.
- **Mailboxes** — quotas, a usage bar, last login, per-mailbox app
  passwords, Sieve filters, rate limits and temporary aliases in a
  slide-over drawer, plus bulk creation from a CSV file.
- **Domains** — a detail page per domain with DKIM key management, rate
  limits, and a live DNS check that verifies MX, the mail host's A record,
  SPF, DKIM and DMARC against the live zone.
- **Aliases** — a mailbox-picker for recipients and a catch-all-for-domain
  toggle.
- **Mail queue** — flush for redelivery, or discard everything outright; the
  same ready/queued/stuck counts the table shows are also drawn as pens of
  cow-envelopes at the top of the page, playing on Mailfold's own
  "Cow-Managed" branding.
- **Quarantine** — release or delete held messages.
- **Spam policy** — per-domain allow/block lists, plus custom Rspamd rule
  blocks for finer-grained scoring and whitelisting.
- **Sync jobs** — IMAP/POP import, with a move-instead-of-copy option.
- **Logs** — per-service, with an "All services" merged view and a
  live-tail toggle.
- **Fail2ban** and **Settings** (theme, accent, language).
- **Advanced** — the niche routing/config resources: relay hosts,
  transports, TLS policies, forwarding hosts, BCC/recipient maps, and
  domain/mailbox templates.
- **Administration** — delegated admins, domain admins, SSO providers, and
  OAuth2 clients.

## API & SDKs

**API keys — drive your mailbox from other apps.** Issue durable,
individually-revocable API keys so any third-party service can **send and
collect mail** for a mailbox over a simple REST API — without ever handling
the mailbox password. A key is a thin bearer handle in front of a mailcow
application password scoped to **IMAP + SMTP only**; the app-password is
encrypted at rest (AES-256-GCM) and the plaintext token is shown exactly
once, at creation.

Enable it with `MAILFOLD_APIKEY_ENABLED=true` and a
`MAILFOLD_APIKEY_MASTER_KEY` (≥32 bytes, hex or base64). It reuses
`MAILFOLD_DB_PATH` for storage and stays off until both are set.

- Admin (normal session) mints/lists/revokes keys: `POST/GET /api/apikeys`,
  `DELETE /api/apikeys/{id}`. Revoking a key also deletes its upstream
  app-password.
- The key itself authenticates the machine surface with
  `Authorization: Bearer mf_live_…`:
  - **Send** (`mail:send`): `POST /api/v1/mail/send` — `From` is forced to
    the bound mailbox (no spoofing), with recipient and body-size caps.
  - **Collect** (`mail:read`): `GET /api/v1/mail/folders|messages|message|search|attachment`.
  - **Mutate** (`mail:write`): `POST /api/v1/mail/flag`, `DELETE /api/v1/mail/message`.
  - The same key can also [sign into webmail directly](#authentication--access)
    on a new device, in place of the mailbox password.

Requests are rate-limited per source IP (before authentication) and per
key (defaults: 120 requests/minute per key, 50 recipients per send, 1MB
combined text+HTML body — all tunable via `MAILFOLD_APIKEY_RATE_MAX`,
`MAILFOLD_APIKEY_RATE_WINDOW`, `MAILFOLD_APIKEY_MAX_RECIPIENTS`), and all
token verification is constant-time.

### Why an API instead of talking SMTP/IMAP directly?

You *can* always drop down to raw SMTP and IMAP — Mailfold doesn't hide or
disable them. The API exists because, for the integrations most people
actually build (a service that sends notifications and occasionally reads
a reply), it removes work SMTP/IMAP push onto every caller:

- **One credential, one endpoint, both directions.** SMTP alone can only
  send. Reading needs a second protocol, a second library, and a second
  credential — the API key does both over the same HTTPS endpoint.
- **Never touches the real mailbox password.** A key fronts a scoped
  mailcow app-password. Leak one key and you revoke *that one key* —
  nothing else breaks, and the mailbox password never changes.
- **Scoped and individually revocable per integration.** Mint a
  `mail:send`-only key for a contact-form mailer, a `mail:read`-only key
  for a ticket importer, and so on — instead of every integration sharing
  one all-or-nothing mailbox password.
- **Guardrails you'd otherwise build yourself.** Recipient caps, a
  body-size cap, per-IP and per-key rate limiting, and CRLF/header-injection
  rejection on every field are enforced server-side, not left as an
  exercise for each client.
- **Plain HTTPS gets through firewalls that block mail ports.** SMTP
  (587/465) and IMAP (993) are routinely blocked by corporate egress rules,
  CI runners, and some cloud providers — including, at one point, the very
  server Mailfold's own reference deployment runs on. Port 443 almost
  always works.
- **No protocol/session state to manage.** No STARTTLS handshake, no IMAP
  `SELECT`/`IDLE` state machine, no connection pooling — every call is one
  stateless, safely-retryable HTTP request.
- **Structured JSON, not raw MIME.** Messages, folders, and attachments
  come back already decoded into typed fields, instead of something your
  code has to parse out of the wire format yourself.
- **Official, maintained SDKs in three languages** — see below — instead of
  hunting for (or hand-rolling) a decent SMTP+IMAP library pair per
  language.

### What an API key can and can't do

The key-authenticated surface is a thin, safe layer over the operations
most integrations need — **send and collect**, not a full IMAP/SMTP
replacement. Concretely, with an API key you can:

- Send a message (plain text and/or HTML) and list, search, read, flag, or
  permanently delete messages in any folder of the one mailbox it's bound to.

You **cannot**, with an API key alone:

- **Attach files when sending** — `POST /api/v1/mail/send` takes a subject
  and text/HTML body only, no attachments. (Downloading an attachment from
  a *received* message is supported.)
- **Move a message between folders, or create a new folder** — those are
  webmail-session-only operations.
- **Read or write calendars/contacts** — CardDAV/CalDAV authenticate with
  the mailbox's real IMAP password over HTTP Basic, not an API key.
- **Get pushed new-mail notifications in real time** — the SSE stream
  needs a webmail session token; with an API key, poll `messages`/`search`
  instead.
- **Touch any mailbox other than the one the key is bound to**, or perform
  admin operations (creating mailboxes, configuring domains) — those need
  the separate admin session, a different authentication tier entirely.
- **Read or change the mailbox's signature/rules** — those are also
  webmail-session-only settings.

If an integration outgrows the API-key surface, it doesn't need a second
credential: the same key can be
[exchanged once for a full webmail session](#authentication--access) via
device sign-in, which unlocks everything above.

### Official client SDKs

Official, minimal client libraries wrap this API for you — mint a key in
the admin panel and start sending/collecting mail in a few lines. All three
are published and installable right now:

| Language | Repository | Package |
| --- | --- | --- |
| Python | [isi1988/mailfold-python](https://github.com/isi1988/mailfold-python) | `pip install mailfold-client` — [PyPI](https://pypi.org/project/mailfold-client/) (zero third-party dependencies) |
| Go | [isi1988/mailfold-go](https://github.com/isi1988/mailfold-go) | `go get github.com/isi1988/mailfold-go` — [pkg.go.dev](https://pkg.go.dev/github.com/isi1988/mailfold-go) (zero third-party dependencies) |
| Rust | [isi1988/mailfold-rust](https://github.com/isi1988/mailfold-rust) | `cargo add mailfold` — [crates.io](https://crates.io/crates/mailfold) |

Each repository has its own README with a full quickstart. For any other
language, the REST surface is small enough to call directly — see below.

### API documentation

The running server documents itself:

- `GET /api/docs` — interactive Swagger UI.
- `GET /api/openapi.yaml` — the raw OpenAPI 3 spec
  ([backend/internal/api/openapi.yaml](backend/internal/api/openapi.yaml)).

## Internationalization

Every user-facing string goes through a translation layer; the UI ships in
English today, and adding a language is a single drop-in locale file.

Domains in a non-Latin script (e.g. родоскоп.рф) are stored by mailcow in
punycode (`xn--d1amkbbgbl.xn--p1ai`); Mailfold decodes them back to
readable Unicode everywhere they're shown, and normalizes either form to
punycode before IMAP/SMTP authentication and in every outgoing recipient
address, so a mailbox works the same — logging in, sending, and receiving —
whether you type its address in Cyrillic or in punycode.

## Architecture

```
React SPA  ──▶  Mailfold Go backend  ──▶  mailcow API (/api/v1/...)
```

The Go backend authenticates to mailcow with an API key, exposes a clean
REST surface to the frontend, and serves the built SPA.

## Scaling: sessions survive a restart and a load balancer

Every bearer-token session Mailfold issues — the admin's, a webmail
mailbox's, and a domain admin's, plus the pending token in between the
password and TOTP/passkey steps of a login — is backed by the same database
already used for DAV, API keys, and the audit log
([`backend/internal/sessionstore`](backend/internal/sessionstore)). No new
environment variable is needed: it activates automatically whenever
`MAILFOLD_DB_PATH` is set, exactly like every other optional store in this
codebase. Without a database configured, every session manager falls back to
its original in-process map — correct for a single instance, but unable to
recognize a token minted by another one.

This is what makes running more than one Mailfold instance behind a load
balancer *without sticky sessions* possible: a token minted by instance A is
validated by instance B by reading the same row, not by asking A. Only its
hash is ever stored (never the raw token, matching password-reset and
API-key tokens elsewhere in this codebase), and a webmail session's mailbox
password — needed on every IMAP/SMTP call — is additionally AES-256-GCM
encrypted at rest, using the same `MAILFOLD_ADMIN_ENC_KEY` that already
protects TOTP secrets and the notify-sender password; without that key,
webmail sessions specifically stay in-memory even with a database present,
rather than ever persisting a password unencrypted.

Two smaller pieces of transient state deliberately stay per-process rather
than move to the database, since losing one mid-flight just costs a retry
rather than an established session: login rate limiters, and the few-minute
WebAuthn/passkey and SSO ceremony challenges. A load balancer without sticky
sessions can occasionally land a passkey or SSO callback on a different
instance than the one that started it — the fix is simply to try again — but
can never land on an instance that doesn't recognize an already-signed-in
user.

## Editions (open-core)

Mailfold is open-core. The community edition in this repository is
complete and self-contained; it stores its groupware and API-key data in
**SQLite** and needs no external database. The persistence layer is a small
driver registry ([`backend/storage`](backend/storage)), so the same store
code runs on more than one database with no duplication.

The **enterprise edition** adds a **PostgreSQL** backend for larger,
multi-node deployments. It is a separate, private Go module that imports
this core and registers a `postgres` driver — no forked backend, just one
extra driver and entry point. PostgreSQL is strictly enterprise-only: the
community binary has no PostgreSQL dependency and, asked for
`MAILFOLD_DB_DRIVER=postgres`, fails fast with *"database driver
\"postgres\" is not available in this build"*.

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

### Docker

A prebuilt image is published to Docker Hub:

```bash
docker run --rm -p 8080:8080 \
  -e MAILFOLD_MAILCOW_URL=https://mail.example.com \
  -e MAILFOLD_MAILCOW_API_KEY=... \
  -e MAILFOLD_ADMIN_PASSWORD=... \
  sivanov2018/mailfold:0.1.0
```

Or build it yourself — a multi-stage [`Dockerfile`](Dockerfile) builds a
static binary into a distroless, non-root image:

```bash
docker build -t mailfold-backend .
docker run --rm -p 8080:8080 \
  -e MAILFOLD_MAILCOW_URL=https://mail.example.com \
  -e MAILFOLD_MAILCOW_API_KEY=... \
  -e MAILFOLD_ADMIN_PASSWORD=... \
  mailfold-backend
```

### Docker Compose

For local development or simple self-hosting,
[`docker-compose.yml`](docker-compose.yml) runs the backend and connects it
to a mailcow instance (by default a mailcow on the host at `:8443`):

```bash
cp .env.example .env   # set the admin password, mailcow API key, etc.
docker compose up --build
```

## Frontend

The UI is a Vite + React (JSX) single-page app in [`frontend/`](frontend/):
`main.jsx` → `App.jsx` (auth gate) → the app shell and pages under
`src/pages/`, talking to the backend through `src/api/client.js`. The design
system is vendored under `src/ds/`, and all user-facing strings go through
the i18n layer in `src/i18n/`. It builds to `frontend/dist/`, which the Go
backend serves (`MAILFOLD_FRONTEND_DIR`). See
[`frontend/README.md`](frontend/README.md).

## Continuous integration

Every push and pull request runs
[`.github/workflows/ci.yml`](.github/workflows/ci.yml):

1. **Build & test** — `go build`, `go vet`, and `go test` with coverage,
   failing the run if total coverage drops below **80%**.
2. **SonarQube quality gate** — analysis is uploaded to SonarQube and the
   build fails unless the project's quality gate is green.

CI requires two repository settings: a `SONAR_TOKEN` secret and a
`SONAR_HOST_URL` variable.

## License

Mailfold is licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE).

You are free to **use, modify, and share** Mailfold for **any noncommercial
purpose** — personal projects, research, education, nonprofits, and
evaluation. Contributions that keep the project growing are welcome.
**Commercial use is not permitted** under this license.

Commercial licenses — including the right to use Mailfold in a paid product
or service, or to redistribute it commercially — are available separately
from the copyright holder. See [mailfold.site](https://mailfold.site).

The official [Python](https://github.com/isi1988/mailfold-python),
[Go](https://github.com/isi1988/mailfold-go), and
[Rust](https://github.com/isi1988/mailfold-rust) client SDKs are separate,
MIT-licensed projects — they contain no Mailfold server code, only a thin
API wrapper, so they're licensed for the widest possible reuse.

Copyright © 2026 Sviatoslav Ivanov (Team26). All rights not expressly
granted by the license are reserved.
