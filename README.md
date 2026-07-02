# Mailfold

[![CI](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml/badge.svg)](https://github.com/isi1988/Mailfold/actions/workflows/ci.yml)

A modern web frontend (admin panel + webmail) for [mailcow-dockerized](https://github.com/mailcow/mailcow-dockerized).

**Website:** [mailfold.site](https://mailfold.site)

**Mailfold** — the name plays on two meanings: a *fold* is both the crease of a letter (mail) and an enclosure for livestock, nodding to mailcow's cattle branding and the "herd" of domains and mailboxes the panel manages.

## Stack

- **Backend:** Go — a thin, typed API layer on top of the mailcow REST API.
- **Frontend:** React — UI is delivered from a separate design project (WIP).

All code and documentation are written in English.

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
